package populous2

import "fmt"

// RestoreStartupPanel executes the source panel against real physical sprite
// banks and the shared image/audio state. Ownership is a host operation with
// the source's full-register save/restore contract.
func (h *NativeRuntimeHost) RestoreStartupPanel(rules *NativeStartupPanelFrameRules, frame *NativeFrameRegisterContext, a *[7]NativeRequesterAddress, ownership func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error) error {
	if h == nil || h.Memory == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase || a == nil {
		return fmt.Errorf("native runtime panel context/base missing")
	}
	logical, err := h.LogicalCode()
	if err != nil {
		return err
	}
	_, err = rules.RestorePanel(NativeStartupPanelFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: frame, Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase}, Logical: logical, Image: &h.Session.Image, Bitmap: h.Bitmap, Ownership: ownership}, a)
	return err
}
