package populous2

import "fmt"

type NativeGameplayHUDHostCallbacks struct {
	Campaign  NativeCampaignSelectionChildrenCallbacks
	Panel     NativeStartupPanelFrameRules
	Ownership func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error
	Audio     NativeRuntimeAudioOperations
}

// NativeGameplayHUDHost retains the actual HUD child while the outer source
// input pass waits. Call inside the caller's shared NativeRuntimeAccess scope;
// audio callbacks must borrow that same ownership rather than relock it.
type NativeGameplayHUDHost struct {
	Input    *NativeGameplayHUDInputState
	Children NativeCampaignSelectionChildren
	Blitter  NativeCampaignBlitterState
}

func (s *NativeGameplayHUDHost) Call(h *NativeRuntimeHost, r *NativeGameplayHUDInputRules, children *NativeCampaignSelectionChildrenRules, supplied NativeGameplayHUDHostCallbacks, call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
	if s == nil || h == nil || h.Memory == nil || r == nil || children == nil || call.Frame == nil || call.A == nil || phase == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native HUD host child/context missing")
	}
	if call.Routine == 0x147e0 {
		cb := NativeGameplayHUDInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase, Frame: call.Frame}}
		_, e := NativeGameplayHUDAdmission(r, cb, call.A)
		return NativeCommandFrameResult{Complete: e == nil, Zero: uint16(call.Frame.D[4]) == 0}, e
	}
	if call.Routine != 0x23ee && call.Routine != 0x2472 {
		return NativeCommandFrameResult{}, fmt.Errorf("native HUD host routine%x unsupported", call.Routine)
	}
	if *phase == 0 {
		s.Input = &NativeGameplayHUDInputState{Entry: call.Routine, A: *call.A}
		*phase = 1
	} else if s.Input == nil || s.Input.Entry != call.Routine {
		return NativeCommandFrameResult{}, fmt.Errorf("native HUD host entry changed duringwait")
	}
	supplied.Campaign.Blitter = &s.Blitter
	campaign, e := h.CampaignCallbacks(call.Frame, children, &s.Children, supplied.Campaign)
	if e != nil {
		return NativeCommandFrameResult{}, e
	}
	cb := NativeGameplayHUDInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase, Frame: call.Frame}}
	cb.Call = func(inner NativeStartupResetFrameCall, childPhase *uint32) (NativeCommandFrameResult, error) {
		switch inner.Routine {
		case 0x184f6:
			if supplied.Audio.DirectCue == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native HUD actualdirectaudio missing")
			}
			e := supplied.Audio.DirectCue(uint16(inner.Frame.D[0]), inner.Frame)
			return NativeCommandFrameResult{Complete: e == nil}, e
		case 0x1da0:
			if supplied.Ownership == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native HUD actualpanelownership missing")
			}
			e := h.RestoreStartupPanel(&supplied.Panel, inner.Frame, inner.A, supplied.Ownership)
			return NativeCommandFrameResult{Complete: e == nil}, e
		case 0x517a:
			return campaign.Child(inner, childPhase)
		default:
			return NativeCommandFrameResult{}, fmt.Errorf("native HUD nestedchild%x unsupported", inner.Routine)
		}
	}
	step, e := s.Input.Advance(r, cb)
	*call.A = s.Input.A
	if step.Complete {
		s.Input = nil
	}
	return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, e
}
