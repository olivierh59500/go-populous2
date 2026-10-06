package populous2

import "fmt"

// NativeRuntimeDeity retains the actual B740 editor and its physical
// profile/resource/palette children. Name/password modal input is owned by
// the source controller; no typed Deity fields replace its raw bytes.
type NativeRuntimeDeity struct {
	State         *NativeStartupDeityFrameState
	Render        NativeRenderFrameRules
	ChildrenRules NativeCampaignSelectionChildrenRules
	Children      NativeCampaignSelectionChildren
	decoded       bool
}

func (s *NativeRuntimeDeity) AdvanceChild(h *NativeRuntimeHost, call NativeStartupResetFrameCall, phase *uint32, supplied NativeCampaignSelectionChildrenCallbacks) (NativeCommandFrameResult, error) {
	if s == nil || h == nil || call.Frame == nil || call.A == nil || phase == nil || call.Routine != 0xb740 {
		return NativeCommandFrameResult{}, fmt.Errorf("native runtime deity context missing")
	}
	if !s.decoded {
		var err error
		s.Render, err = DecodeNativeRenderFrameRules(h.Bundle.Executable)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		s.ChildrenRules, err = DecodeNativeCampaignSelectionChildrenRules(h.Bundle.Executable)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		logical, err := h.LogicalCode()
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		if err := s.Render.BindCode(h.Memory.Code, logical.Read32); err != nil {
			return NativeCommandFrameResult{}, err
		}
		s.decoded = true
	}
	if *phase == 0 {
		s.State = &NativeStartupDeityFrameState{A: *call.A}
		*phase = 1
	} else if s.State == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native deity state missing duringwait")
	}
	cb, err := h.CampaignCallbacks(call.Frame, &s.ChildrenRules, &s.Children, supplied)
	if err != nil {
		return NativeCommandFrameResult{}, err
	}
	step, err := s.State.Advance(&s.Render, cb)
	*call.A = s.State.A
	if step.Complete {
		s.State = nil
	}
	return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, err
}
