package populous2

import "fmt"

// NativeRuntimeDirector runs actual10A10 startup through the real initial
// menu and world chooser. It retains one set of child state across input,
// resource and palette waits. It does not acknowledge unsupplied deity/file
// or transport bodies.
type NativeRuntimeDirector struct {
	Startup   NativeStartupHostFrameState
	Menu      *NativeStartupMenuHostFrameState
	Selection *NativeCampaignSelectionFrameState
	Children  NativeCampaignSelectionChildren
	Blitter   NativeCampaignBlitterState
	started   bool
	failed    error
	ready     bool
}

type NativeRuntimeDirectorCallbacks struct {
	NativeStartupCampaignHostCallbacks
	MenuChild func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

func (s *NativeRuntimeDirector) Advance(h *NativeRuntimeHost, r *NativeStartupCampaignHostRules, frame *NativeFrameRegisterContext, supplied NativeRuntimeDirectorCallbacks) (out NativeStartupResetFrameStep, failure error) {
	if s == nil || h == nil || r == nil || frame == nil {
		return out, fmt.Errorf("native runtime director context missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.started {
		s.started = true
		s.Startup.Startup.Entry = 0x10a10
	}
	defer func() {
		if failure != nil {
			s.failed = failure
		}
	}()
	supplied.Campaign.Blitter = &s.Blitter
	campaign, err := h.CampaignCallbacks(frame, &r.Children, &s.Children, supplied.Campaign)
	if err != nil {
		return out, err
	}
	outer := supplied.NativeStartupHostFrameCallbacks
	external := outer.Call
	outer.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		switch call.Routine {
		case 0x3b64:
			if s.Menu == nil {
				s.Menu = &NativeStartupMenuHostFrameState{Menu: NativeStartupMenuFrameState{A: *call.A}}
			}
			cb := campaign
			cb.Frame = call.Frame
			cb.Child = supplied.MenuChild
			step, err := s.Menu.Advance(cb)
			*call.A = s.Menu.Menu.A
			if step.Complete {
				s.Menu = nil
			}
			return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, err
		case 0x3cba:
			if s.Selection == nil {
				s.Selection = &NativeCampaignSelectionFrameState{A: *call.A}
			}
			cb := campaign
			cb.Frame = call.Frame
			step, err := s.Selection.Advance(&r.Selection, cb)
			*call.A = s.Selection.A
			if step.Complete {
				s.Selection = nil
			}
			return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, err
		case 0x1da0:
			if supplied.Ownership == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native director panel ownership missing")
			}
			err := h.RestoreStartupPanel(&r.Panel, call.Frame, call.A, supplied.Ownership)
			return NativeCommandFrameResult{Complete: err == nil}, err
		default:
			if external == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native director child%x missing", call.Routine)
			}
			return external(call, phase)
		}
	}
	cb, err := h.StartupCallbacks(frame, outer, supplied.Errors)
	if err != nil {
		return out, err
	}
	out, failure = s.Startup.Advance(&r.Startup, cb)
	if failure == nil && out.Complete && !s.ready {
		if err := h.RefreshWorldCaches(); err != nil {
			return out, err
		}
		s.ready = true
	}
	return out, failure
}
