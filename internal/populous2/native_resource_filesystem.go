package populous2

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// NativeResourceFilesystem exposes original encoded file bytes to19CD0.
// Bundle.Raw is decoded and must not be used as an encoded read payload.
// Calls are serialized by the retained loader's host update goroutine.
type NativeResourceFilesystem struct {
	Files     fs.FS
	Paths     map[string]string
	handles   map[uint32]fs.File
	next      uint32
	LastError error
}

func NewNativeResourceFilesystem(files fs.FS) (*NativeResourceFilesystem, error) {
	paths, err := resourcePaths(files)
	if err != nil {
		return nil, err
	}
	return &NativeResourceFilesystem{Files: files, Paths: paths, handles: make(map[uint32]fs.File), next: 1}, nil
}

// IO returns actual host transfer counts. Native I/O failures return0/-1
// and stay available in LastError so the genuine failure requester can run.
func (f *NativeResourceFilesystem) IO(call NativeResourceFrameIOCall, _ *uint32) (NativeResourceFrameIOResult, error) {
	out := NativeResourceFrameIOResult{Complete: true}
	if f == nil || f.Files == nil || f.handles == nil {
		return out, fmt.Errorf("native encoded resource filesystem missing")
	}
	f.LastError = nil
	switch call.Operation {
	case "open":
		path, exists := f.Paths[strings.ToUpper(call.Name)]
		if !exists {
			f.LastError = fs.ErrNotExist
			return out, nil
		}
		file, err := f.Files.Open(path)
		if err != nil {
			f.LastError = err
			return out, nil
		}
		if f.next == 0 || f.next > 0x7fffffff {
			_ = file.Close()
			return out, fmt.Errorf("native resource handle range exhausted")
		}
		handle := f.next
		f.next++
		f.handles[handle] = file
		out.Value = int32(handle)
	case "read":
		file, exists := f.handles[call.Handle]
		if !exists {
			f.LastError = fs.ErrInvalid
			out.Value = -1
			return out, nil
		}
		if call.Limit > MaxDecodedBytes {
			return out, fmt.Errorf("native resource read limit%d unsupported", call.Limit)
		}
		data := make([]byte, int(call.Limit))
		n, err := file.Read(data)
		if n < 0 || n > len(data) {
			return out, fmt.Errorf("native host returned invalid resource count")
		}
		if err != nil && err != io.EOF {
			f.LastError = err
			if n == 0 {
				out.Value = -1
				return out, nil
			}
		}
		out.Data, out.Value = data[:n], int32(n)
	case "close":
		file, exists := f.handles[call.Handle]
		if !exists {
			f.LastError = fs.ErrInvalid
			return out, nil
		}
		delete(f.handles, call.Handle)
		if err := file.Close(); err != nil {
			f.LastError = err
			return out, nil
		}
		out.Value = -1 // AmigaDOS DOSTRUE.
	default:
		return out, fmt.Errorf("native resource operation%q unsupported", call.Operation)
	}
	return out, nil
}

func (f *NativeResourceFilesystem) Close() error {
	if f == nil {
		return nil
	}
	var first error
	for handle, file := range f.handles {
		if err := file.Close(); err != nil && first == nil {
			first = err
		}
		delete(f.handles, handle)
	}
	return first
}
