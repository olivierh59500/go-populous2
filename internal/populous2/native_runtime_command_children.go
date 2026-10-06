package populous2

import "fmt"

// NativeRuntimeCommandChildren retains actual reset/resource children of17500.
// File/deity/transport operations remain supplied real ports. Parent command
// rules own their saved D/palette/target arguments and resume phases.
type NativeRuntimeCommandChildren struct {
	Host           *NativeRuntimeHost
	Rules          NativeStartupCampaignHostRules
	Supplied       NativeRuntimeDirectorCallbacks
	Startup        *NativeRuntimeDirector
	Resource       *NativeResourceHostFrameState
	Routine        int
	Audio          NativeAudioControlFrameCallbacks
	Other          func(NativeCommandFrameCall, *uint32) (NativeCommandFrameResult, error)
	RefreshPending bool
}

func (s *NativeRuntimeCommandChildren) Call(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
	if s == nil || s.Host == nil || call.Context == nil || phase == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native runtime command child context missing")
	}
	h := s.Host
	if call.Routine == 0x102e4 || call.Routine == 0xd8cc {
		return h.Session.AdvanceBuiltInCommandChild(call, phase)
	}
	frame := NativeFrameRegisterContext{D: call.Context.D, AddressBase: h.Memory.BSSBase}
	defer func() { call.Context.D = frame.D }()
	switch call.Routine {
	case 0x1842e:
		cb := s.Audio
		cb.Frame, cb.Memory, cb.CodeBase = &frame, h.Memory.BSS, h.Memory.CodeBase
		_, err := s.Rules.Startup.Audio.Run(0x1842e, cb)
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0x10ad8, 0x10a8c:
		if s.Startup == nil {
			s.Startup = &NativeRuntimeDirector{started: true, deferCaches: true}
			s.Startup.Startup.Startup.Entry = call.Routine
			s.Routine = call.Routine
		} else if s.Routine != call.Routine {
			return NativeCommandFrameResult{}, fmt.Errorf("native command reset changed duringwait")
		}
		step, err := s.Startup.Advance(h, &s.Rules, &frame, s.Supplied)
		if step.Complete {
			s.RefreshPending = true
			s.Startup = nil
			s.Routine = 0
		}
		return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, err
	case 0x1a32a:
		if s.Resource == nil {
			s.Resource = &NativeResourceHostFrameState{Landscape: true}
		}
		cb, err := h.ResourceCallbacks(&frame, s.Supplied.Errors)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		step, err := s.Resource.Advance(&h.ResourceRules, cb)
		if step.Complete {
			s.Resource = nil
		}
		return NativeCommandFrameResult{Complete: step.Complete}, err
	default:
		if s.Other == nil {
			return NativeCommandFrameResult{}, fmt.Errorf("native runtime command body%x missing", call.Routine)
		}
		return s.Other(call, phase)
	}
}
