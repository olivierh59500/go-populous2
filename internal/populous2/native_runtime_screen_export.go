package populous2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NativeRuntimeScreenExport binds1A55A to an explicitly chosen directory.
// Each invocation retains the native filename, palette, planar rows and IO
// continuation; existing files are preserved unless replacement is enabled.
type NativeRuntimeScreenExport struct {
	Files NativeGameplayScreenExportFilesystem
	State *NativeGameplayScreenExportState
}

func NewNativeRuntimeScreenExport(root string, allowReplace bool) (*NativeRuntimeScreenExport, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("native screen export requires an existing directory")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &NativeRuntimeScreenExport{Files: NativeGameplayScreenExportFilesystem{AllowReplace: allowReplace, Resolve: func(name string) (string, error) {
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:") || filepath.Base(name) != name {
			return "", fmt.Errorf("native screen export filename outside configured directory")
		}
		return filepath.Join(root, name), nil
	}}}, nil
}

func (s *NativeRuntimeScreenExport) AdvanceChild(h *NativeRuntimeHost, call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
	if s == nil || h == nil || h.Memory == nil || call.Routine != 0x1a55a || call.Frame == nil || call.A == nil || phase == nil || call.Frame.AddressBase != h.Memory.BSSBase {
		return NativeCommandFrameResult{}, fmt.Errorf("native screen export runtime/context missing")
	}
	if *phase == 0 {
		s.State = &NativeGameplayScreenExportState{A: *call.A}
		*phase = 1
	}
	if s.State == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native screen export state missing during wait")
	}
	step, err := s.State.Advance(NativeGameplayScreenExportCallbacks{
		NativeGameplayEditorInputCallbacks: NativeGameplayEditorInputCallbacks{
			NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: call.Frame, Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase}, Bitmap: h.Bitmap,
		}, IO: s.Files.IO,
	})
	*call.A = s.State.A
	if step.Complete {
		s.State = nil
	}
	return NativeCommandFrameResult{Complete: step.Complete}, err
}

func (s *NativeRuntimeScreenExport) Close() error {
	if s == nil {
		return nil
	}
	return s.Files.Close()
}
