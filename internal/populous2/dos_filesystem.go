package populous2

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type nativeDOSHostHandle struct {
	file      *os.File
	directory bool
	key       uint32
}
type nativeDOSHostResult struct {
	d0    uint32
	data  []byte
	cause error
}
type nativeDOSHostRequest struct {
	vector       int
	path         string
	mode, handle uint32
	data         []byte
}

// NativeDOSFilesystem supplies real portable operations to the native DOS
// leaf controllers. Async uses a retained operation future: polling neither
// repeats an Open/Write nor fabricates a successful library result. Read
// payloads and FIB bytes reach native backing only on the owner goroutine.
type NativeDOSFilesystem struct {
	Root                      string
	ResolvePath               func([]byte) (string, error)
	Async                     bool
	mu                        sync.Mutex
	nextHandle, nextOperation uint32
	handles                   map[uint32]*nativeDOSHostHandle
	pending                   map[uint32]chan nativeDOSHostResult
	lastError                 error
	closed                    bool
}

func NewNativeDOSFilesystem(root string) *NativeDOSFilesystem {
	return &NativeDOSFilesystem{Root: root, nextHandle: 1, nextOperation: 1, handles: map[uint32]*nativeDOSHostHandle{}, pending: map[uint32]chan nativeDOSHostResult{}}
}

func (f *NativeDOSFilesystem) LastError() error { f.mu.Lock(); defer f.mu.Unlock(); return f.lastError }

// Close releases any handles retained by an interrupted host session. It is
// separate from the original per-operation Close/UnLock library vectors.
func (f *NativeDOSFilesystem) Close() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	var first error
	for id, h := range f.handles {
		if e := h.file.Close(); e != nil && first == nil {
			first = e
		}
		delete(f.handles, id)
	}
	return first
}

func (f *NativeDOSFilesystem) resolve(name []byte) (string, error) {
	if f.ResolvePath != nil {
		return f.ResolvePath(append([]byte(nil), name...))
	}
	value := string(name)
	if strings.Contains(value, ":") {
		return "", fmt.Errorf("native DOS volume needs an explicit host resolver")
	}
	if value == "" {
		return f.Root, nil
	}
	path := value
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.Root, path)
	}
	path, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	// Native DOS lookup ignores ASCII case. Resolve identity without
	// sorting the separate open directory stream used by ExNext.
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(clean, current), string(filepath.Separator)) {
		candidate := filepath.Join(current, component)
		if _, e := os.Lstat(candidate); e == nil {
			current = candidate
			continue
		}
		directory, e := os.Open(current)
		if e != nil {
			current = candidate
			continue
		}
		entries, _ := directory.Readdirnames(-1)
		_ = directory.Close()
		found := component
		for _, actual := range entries {
			if asciiDOSNameEqual(actual, component) {
				found = actual
				break
			}
		}
		current = filepath.Join(current, found)
	}
	return current, nil
}

func asciiDOSNameEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x >= 'a' && x <= 'z' {
			x -= 32
		}
		if y >= 'a' && y <= 'z' {
			y -= 32
		}
		if x != y {
			return false
		}
	}
	return true
}

// Port uses the documented DOSFALSE/DOSTRUE (0/-1), positive handles and
// actual transfer counts. The caller's raw D0 high word remains significant
// after a successful Close; the native controller applies MOVE.W/CLR.W.
func (f *NativeDOSFilesystem) Port() NativeDOSPort {
	return NativeDOSPort{Call: func(call NativeDOSLibraryCall, phase *uint32) (NativeDOSLibraryResult, error) {
		if f == nil || phase == nil || call.Frame == nil {
			return NativeDOSLibraryResult{}, fmt.Errorf("native DOS filesystem state missing")
		}
		apply := func(result nativeDOSHostResult) (NativeDOSLibraryResult, error) {
			f.mu.Lock()
			f.lastError = result.cause
			f.mu.Unlock()
			if len(result.data) != 0 {
				memory, at := call.Memory, int(int64(call.Buffer.Address)-int64(call.Frame.AddressBase))
				if call.Buffer.Code {
					memory = call.Code
					at = int(int64(call.Buffer.Address) - int64(call.CodeBase))
				}
				for i, v := range result.data {
					if e := memory.Write8(at+i, v); e != nil {
						return NativeDOSLibraryResult{}, e
					}
				}
			}
			return NativeDOSLibraryResult{Complete: true, D0: result.d0}, nil
		}
		if *phase != 0 {
			f.mu.Lock()
			done, ok := f.pending[*phase]
			f.mu.Unlock()
			if !ok {
				return NativeDOSLibraryResult{}, fmt.Errorf("native DOS pending operation missing")
			}
			select {
			case result := <-done:
				f.mu.Lock()
				delete(f.pending, *phase)
				f.mu.Unlock()
				*phase = 0
				return apply(result)
			default:
				return NativeDOSLibraryResult{}, nil
			}
		}
		request := nativeDOSHostRequest{vector: call.Vector, handle: call.Handle}
		switch call.Vector {
		case -84, -30:
			path, e := f.resolve(call.Path)
			if e != nil {
				f.mu.Lock()
				f.lastError = e
				f.mu.Unlock()
				return NativeDOSLibraryResult{Complete: true}, nil
			}
			request.path = path
			request.mode = call.Frame.D[2]
		case -42, -48:
			if call.Count != NativeGAMSize || call.Buffer.Address != call.Frame.AddressBase+NativeGAMStart || call.Buffer.Code {
				return NativeDOSLibraryResult{}, fmt.Errorf("native DOS filesystem transfer is outside original GAM span")
			}
			if call.Vector == -48 {
				request.data = make([]byte, NativeGAMSize)
				for i := range request.data {
					v, e := call.Memory.Read8(NativeGAMStart + i)
					if e != nil {
						return NativeDOSLibraryResult{}, e
					}
					request.data[i] = v
				}
			}
		case -102, -108, -90, -36:
		default:
			return NativeDOSLibraryResult{}, fmt.Errorf("native DOS filesystem vector%d unsupported", call.Vector)
		}
		if !f.Async {
			return apply(f.run(request))
		}
		f.mu.Lock()
		id := f.nextOperation
		f.nextOperation++
		done := make(chan nativeDOSHostResult, 1)
		f.pending[id] = done
		f.mu.Unlock()
		*phase = id
		go func() { done <- f.run(request) }()
		return NativeDOSLibraryResult{}, nil
	}}
}

func (f *NativeDOSFilesystem) run(request nativeDOSHostRequest) nativeDOSHostResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	failure := func(value uint32, e error) nativeDOSHostResult { return nativeDOSHostResult{d0: value, cause: e} }
	if f.closed {
		return failure(0, os.ErrClosed)
	}
	handle := f.handles[request.handle]
	switch request.vector {
	case -84, -30:
		var file *os.File
		var e error
		if request.vector == -84 || request.mode == 1005 {
			file, e = os.Open(request.path)
		} else if request.mode == 1006 {
			file, e = os.OpenFile(request.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		} else {
			return failure(0, fmt.Errorf("native DOS open mode%d unsupported", request.mode))
		}
		if e != nil {
			return failure(0, e)
		}
		info, e := file.Stat()
		if e != nil {
			_ = file.Close()
			return failure(0, e)
		}
		id := f.nextHandle
		f.nextHandle++
		f.handles[id] = &nativeDOSHostHandle{file: file, directory: info.IsDir()}
		return nativeDOSHostResult{d0: id}
	case -102:
		if handle == nil {
			return failure(0, os.ErrInvalid)
		}
		info, e := handle.file.Stat()
		if e != nil {
			return failure(0, e)
		}
		return nativeDOSHostResult{d0: 0xffffffff, data: nativeDOSFileInfo(info, handle.key)}
	case -108:
		if handle == nil || !handle.directory {
			return failure(0, os.ErrInvalid)
		}
		// Readdir on the open stream preserves the actual returned host order.
		// os.ReadDir would sort names and is deliberately not used here.
		entries, e := handle.file.Readdir(1)
		if len(entries) == 0 {
			return failure(0, e)
		}
		handle.key++
		return nativeDOSHostResult{d0: 0xffffffff, data: nativeDOSFileInfo(entries[0], handle.key)}
	case -90, -36:
		if handle == nil {
			return failure(0, os.ErrInvalid)
		}
		e := handle.file.Close()
		delete(f.handles, request.handle)
		if e != nil {
			return failure(0, e)
		}
		return nativeDOSHostResult{d0: 0xffffffff}
	case -42:
		if handle == nil {
			return failure(0xffffffff, os.ErrInvalid)
		}
		data := make([]byte, NativeGAMSize)
		n, e := handle.file.Read(data)
		if n == 0 && e != nil && e != io.EOF {
			return failure(0xffffffff, e)
		}
		return nativeDOSHostResult{d0: uint32(n), data: data[:n], cause: e}
	case -48:
		if handle == nil {
			return failure(0xffffffff, os.ErrInvalid)
		}
		n, e := handle.file.Write(request.data)
		if n == 0 && e != nil {
			return failure(0xffffffff, e)
		}
		return nativeDOSHostResult{d0: uint32(n), cause: e}
	}
	return failure(0, fmt.Errorf("native DOS operation unsupported"))
}

func nativeDOSFileInfo(info os.FileInfo, key uint32) []byte {
	b := make([]byte, 260)
	binary.BigEndian.PutUint32(b, key)
	typ := uint32(0xfffffffd)
	if info.IsDir() {
		typ = 2
	} else if !info.Mode().IsRegular() {
		typ = 3
	}
	binary.BigEndian.PutUint32(b[4:], typ)
	name := []byte(info.Name())
	copy(b[8:115], name)
	binary.BigEndian.PutUint32(b[120:], typ)
	binary.BigEndian.PutUint32(b[124:], uint32(info.Size()))
	binary.BigEndian.PutUint32(b[128:], uint32((info.Size()+511)/512))
	return b
}
