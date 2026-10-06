package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeStartupCampaignHostRules struct {
	Startup   NativeStartupHostFrameRules
	Selection NativeCampaignSelectionFrameRules
	Children  NativeCampaignSelectionChildrenRules
	Panel     NativeStartupPanelFrameRules
}

func DecodeNativeStartupCampaignHostRules(exe *amiga.Executable) (NativeStartupCampaignHostRules, error) {
	var r NativeStartupCampaignHostRules
	var e error
	r.Startup, e = DecodeNativeStartupHostFrameRules(exe)
	if e == nil {
		r.Selection, e = DecodeNativeCampaignSelectionFrameRules(exe)
	}
	if e == nil {
		r.Children, e = DecodeNativeCampaignSelectionChildrenRules(exe)
	}
	if e == nil {
		r.Panel, e = DecodeNativeStartupPanelFrameRules(exe)
	}
	return r, e
}

type NativeStartupCampaignHostCallbacks struct {
	NativeStartupHostFrameCallbacks
	Campaign  NativeCampaignSelectionChildrenCallbacks
	Ownership func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error
	Errors    NativeErrorFrameCallbacks
}

// NativeStartupCampaignHostState composes the real $10ad8 campaign entry.
// The caller must already own original startup resources/ICON preparation;
// this entry does not replace $10a10 or acknowledge its pending main menu.
type NativeStartupCampaignHostState struct {
	Startup   NativeStartupHostFrameState
	Selection *NativeCampaignSelectionFrameState
	Children  NativeCampaignSelectionChildren
	Blitter   NativeCampaignBlitterState
	Started   bool
	failed    error
}

func (s *NativeStartupCampaignHostState) Advance(h *NativeRuntimeHost, r *NativeStartupCampaignHostRules, frame *NativeFrameRegisterContext, supplied NativeStartupCampaignHostCallbacks) (out NativeStartupResetFrameStep, failure error) {
	if s == nil || h == nil || r == nil || frame == nil {
		return out, fmt.Errorf("native campaign startup host/context missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.Startup.Startup.Entry = 0x10ad8
	}
	defer func() {
		if failure != nil {
			s.failed = failure
		}
	}()
	supplied.Campaign.Blitter = &s.Blitter
	campaign, e := h.CampaignCallbacks(frame, &r.Children, &s.Children, supplied.Campaign)
	if e != nil {
		return out, e
	}
	outer := supplied.NativeStartupHostFrameCallbacks
	external := outer.Call
	outer.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		switch call.Routine {
		case 0x3cba:
			if s.Selection == nil {
				s.Selection = &NativeCampaignSelectionFrameState{A: *call.A}
			}
			cb := campaign
			cb.Frame = call.Frame
			step, e := s.Selection.Advance(&r.Selection, cb)
			*call.A = s.Selection.A
			if step.Complete {
				s.Selection = nil
			}
			return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, e
		case 0x1da0:
			if supplied.Ownership == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native campaign panel ownership operation missing")
			}
			e := h.RestoreStartupPanel(&r.Panel, call.Frame, call.A, supplied.Ownership)
			return NativeCommandFrameResult{Complete: e == nil}, e
		default:
			if external == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native campaign startup child%x has no actual operation", call.Routine)
			}
			return external(call, phase)
		}
	}
	cb, e := h.StartupCallbacks(frame, outer, supplied.Errors)
	if e != nil {
		return out, e
	}
	return s.Startup.Advance(&r.Startup, cb)
}
