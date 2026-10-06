package populous2

import (
	"fmt"
	"os"
)

// NativeGameplayScreenExportFilesystem is an explicit source file port.
// Resolve selects the actual configured outputpath for the native filename;
// no defaultdirectory or diskmutation exists without that caller operation.
type NativeGameplayScreenExportFilesystem struct {
	Resolve      func(nativeName string) (string, error)
	AllowReplace bool
	handles      map[uint32]*os.File
	next         uint32
	LastError    error
}

func (f *NativeGameplayScreenExportFilesystem) IO(call NativeGameplayScreenExportCall, _ *uint32) (NativeGameplayScreenExportResult, error) {
	out := NativeGameplayScreenExportResult{Complete: true}
	if f == nil || f.Resolve == nil {
		return out, fmt.Errorf("native screenexport configuredpathmissing")
	}
	if f.handles == nil {
		f.handles = map[uint32]*os.File{}
		f.next = 1
	}
	f.LastError = nil
	switch call.Operation {
	case "open":
		if call.Mode != 1006 {
			return out, fmt.Errorf("native screenexport openmode%d unsupported", call.Mode)
		}
		path, e := f.Resolve(call.Name)
		if e != nil {
			f.LastError = e
			return out, nil
		}
		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		if f.AllowReplace {
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}
		file, e := os.OpenFile(path, flags, 0600)
		if e != nil {
			f.LastError = e
			return out, nil
		}
		if f.next == 0 || f.next > 0x7fffffff {
			_ = file.Close()
			return out, fmt.Errorf("native screenexport handle range exhausted")
		}
		handle := f.next
		f.next++
		f.handles[handle] = file
		out.Value = int32(handle)
	case "write":
		file, ok := f.handles[call.Handle]
		if !ok {
			f.LastError = os.ErrInvalid
			out.Value = -1
			return out, nil
		}
		n, e := file.Write(call.Data)
		if e != nil {
			f.LastError = e
			if n == 0 {
				out.Value = -1
				return out, nil
			}
		}
		out.Value = int32(n)
	case "close":
		file, ok := f.handles[call.Handle]
		if !ok {
			f.LastError = os.ErrInvalid
			return out, nil
		}
		delete(f.handles, call.Handle)
		if e := file.Close(); e != nil {
			f.LastError = e
			return out, nil
		}
		out.Value = -1
	default:
		return out, fmt.Errorf("native screenexport operation%s unsupported", call.Operation)
	}
	return out, nil
}
func (f *NativeGameplayScreenExportFilesystem) Close() error {
	if f == nil {
		return nil
	}
	var first error
	for handle, file := range f.handles {
		if e := file.Close(); e != nil && first == nil {
			first = e
		}
		delete(f.handles, handle)
	}
	return first
}
