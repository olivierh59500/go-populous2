package populous2

import "fmt"

// CampaignCallbacks binds the actual chooser and its retained children to
// one physical runtime. Blitter/audio operations remain explicit caller-owned
// state, so preview waits cannot reset inherited chip or device state.
func (h *NativeRuntimeHost) CampaignCallbacks(frame *NativeFrameRegisterContext, rules *NativeCampaignSelectionChildrenRules, state *NativeCampaignSelectionChildren, supplied NativeCampaignSelectionChildrenCallbacks) (NativeCampaignFrameCallbacks, error) {
	if h == nil || h.Memory == nil || h.Session == nil || h.Files == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase || rules == nil || state == nil {
		return NativeCampaignFrameCallbacks{}, fmt.Errorf("native runtime campaign context/rules missing")
	}
	supplied.Frame, supplied.Memory, supplied.Code = frame, h.Memory.BSS, h.Memory.Code
	supplied.CodeBase, supplied.RAM = h.Memory.CodeBase, h.Memory.RAM
	supplied.Presentation, supplied.Bitmap = h.Session.Presentation, h.Bitmap
	supplied.ReadAbsolute = func(address uint32) (uint8, error) { return h.Memory.RAM.Read8(int(address)) }
	supplied.ResourceIO = h.Files.IO
	base := supplied.NativeCampaignFrameCallbacks
	base.Child = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		current := supplied
		current.Frame = call.Frame
		return state.Call(rules, current, call, phase)
	}
	return base, nil
}
