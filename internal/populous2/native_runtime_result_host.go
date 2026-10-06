package populous2

import "fmt"

// NativeRuntimeResultHost connects381E to actual reset/menu/chooser bodies.
// Progression supplies the genuine B244 award/ending child. The parent frame
// retains its completed follower recount while this result UI waits.
type NativeRuntimeResultHost struct {
	Result         *NativeRuntimeResultState
	Reset          *NativeRuntimeDirector
	Rules          NativeStartupCampaignHostRules
	Callbacks      NativeRuntimeResultCallbacks
	Startup        NativeRuntimeDirectorCallbacks
	Progression    func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
	RefreshPending bool
}

func (s *NativeRuntimeResultHost) Advance(h *NativeRuntimeHost, identity uint16, frame *NativeFrameRegisterContext) (bool, error) {
	if s == nil || h == nil || frame == nil {
		return false, fmt.Errorf("native result host/context missing")
	}
	if s.Result == nil {
		s.Result = &NativeRuntimeResultState{}
		if err := s.Result.Begin(identity, frame); err != nil {
			return false, err
		}
	}
	cb := s.Callbacks
	cb.Child = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		switch call.Routine {
		case 0xb244:
			if s.Progression == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native result award/ending child missing")
			}
			return s.Progression(call, phase)
		case 0x10a8c:
			if s.Reset == nil {
				var err error
				s.Reset, err = NewNativeRuntimeResetDirector(0x10a8c)
				if err != nil {
					return NativeCommandFrameResult{}, err
				}
				s.Reset.Startup.Startup.A = *call.A
			}
			step, err := s.Reset.Advance(h, &s.Rules, call.Frame, s.Startup)
			*call.A = s.Reset.Startup.Startup.A
			if step.Complete {
				s.Reset = nil
				s.RefreshPending = true
			}
			return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, err
		default:
			return NativeCommandFrameResult{}, fmt.Errorf("native result child%x unavailable", call.Routine)
		}
	}
	step, err := s.Result.Advance(h, frame, cb)
	if step.Complete {
		s.Result = nil
	}
	return step.Complete, err
}
