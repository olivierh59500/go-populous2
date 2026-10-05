package populous2

import "fmt"

type NativeResourceHostFrameCallbacks struct {
	NativeErrorFrameCallbacks
	IO              func(NativeResourceFrameIOCall, *uint32) (NativeResourceFrameIOResult, error)
	ResourceCallerA [7]NativeRequesterAddress // Actual incoming loader address ABI.
}

// NativeResourceHostFrameState composes the actual loader/error requester,
// retaining both while IO, palette waits and acknowledgment are outstanding.
// It owns no allocations: every physical span is supplied coherently by host.
type NativeResourceHostFrameState struct {
	Landscape    bool
	Resource     NativeResourceFrameState
	Graphics     NativeLandscapeResourceFrameState
	Error        NativeErrorFrameState
	ErrorActive  bool
	errorRoutine int
	failed       error
}

func (s *NativeResourceHostFrameState) Advance(r *NativeResourceFrameRules, cb NativeResourceHostFrameCallbacks) (out NativeResourceFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || !winMemoryValid(cb.RAM) || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || cb.IO == nil {
		return out, fmt.Errorf("native resource host aliases/IO missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	defer func() {
		if failure != nil {
			s.failed = failure
		}
	}()
	loader := NativeResourceFrameCallbacks{RAM: cb.RAM, CodeBase: cb.CodeBase, Frame: cb.Frame, Input: &cb.Presentation.Input, IO: cb.IO}
	loader.Call = func(call NativeFileFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x339e && call.Routine != 0x33b2 {
			return NativeCommandFrameResult{}, fmt.Errorf("native resource error child %#x unsupported", call.Routine)
		}
		if !s.ErrorActive {
			s.ErrorActive, s.errorRoutine = true, call.Routine
			s.Error = NativeErrorFrameState{Routine: call.Routine, A: cb.ResourceCallerA}
			resource := &s.Resource
			if s.Landscape {
				resource = &s.Graphics.Load
			}
			s.Error.A[0] = NativeRequesterAddress{Address: resource.Descriptor, Code: true}
			s.Error.A[1] = NativeRequesterAddress{Address: resource.Descriptor + 18, Code: true}
			for index := range call.A {
				if call.Arguments&(1<<uint(index)) != 0 {
					s.Error.A[index] = call.A[index]
				}
			}
		}
		if s.errorRoutine != call.Routine {
			return NativeCommandFrameResult{}, fmt.Errorf("native resource error requester changed while waiting")
		}
		step, err := s.Error.Advance(cb.NativeErrorFrameCallbacks)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		if !step.Complete {
			return NativeCommandFrameResult{}, nil
		}
		s.ErrorActive = false
		return NativeCommandFrameResult{Complete: true}, nil
	}
	if s.Landscape {
		return s.Graphics.Advance(r, loader)
	}
	return s.Resource.Advance(r, loader)
}
