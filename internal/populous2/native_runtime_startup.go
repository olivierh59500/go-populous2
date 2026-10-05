package populous2

import "fmt"

// StartupCallbacks binds physical resource/presentation owners without
// acknowledging unsupplied source children. The caller retains the startup
// state and implements actual menu, transport, panel and audio operations.
func (h *NativeRuntimeHost) StartupCallbacks(frame *NativeFrameRegisterContext, supplied NativeStartupHostFrameCallbacks, errors NativeErrorFrameCallbacks) (NativeStartupHostFrameCallbacks, error) {
	if h == nil || h.Memory == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return NativeStartupHostFrameCallbacks{}, fmt.Errorf("native runtime startup context/base missing")
	}
	supplied.Frame, supplied.Code, supplied.Memory = frame, h.Memory.Code, h.Memory.BSS
	supplied.CodeBase, supplied.RAM = h.Memory.CodeBase, h.Memory.RAM
	supplied.Bitmap = h.Bitmap
	resource, err := h.ResourceCallbacks(frame, errors)
	if err != nil {
		return NativeStartupHostFrameCallbacks{}, err
	}
	supplied.Resource = &resource
	return supplied, nil
}
