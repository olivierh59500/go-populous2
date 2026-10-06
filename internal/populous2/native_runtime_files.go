package populous2

import "fmt"

type NativeRuntimeFilesCallbacks struct {
	NativeFileFrameCallbacks
	Child func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

// NativeRuntimeFilesState composes $19936/$19afc/$19c1c against borrowed
// runtime BSS. Source $3f92 owns selected-pointer rebasing, not this DOS child.
// Partial native reads remain visible; caches refresh only after real return.
type NativeRuntimeFilesState struct {
	DOS            *NativeDOSFrameState
	Routine        int
	OuterA         [7]NativeRequesterAddress
	Overwrite      *NativeRuntimeFileOverwriteState
	OverwriteA0    NativeRequesterAddress
	Resource       *NativeResourceHostFrameState
	RefreshPending bool
}

func (s *NativeRuntimeFilesState) AdvanceChild(h *NativeRuntimeHost, store *NativeRuntimeFileStore, call NativeStartupResetFrameCall, phase *uint32, supplied NativeRuntimeFilesCallbacks) (NativeCommandFrameResult, error) {
	if s == nil || h == nil || h.Memory == nil || h.Session == nil || store == nil || store.DOS == nil || call.Frame == nil || call.A == nil || phase == nil || call.Frame.AddressBase != h.Memory.BSSBase {
		return NativeCommandFrameResult{}, fmt.Errorf("native runtime file context/store missing")
	}
	if *phase == 0 {
		s.Routine = call.Routine
		s.OuterA = *call.A
		s.DOS = &NativeDOSFrameState{}
		*phase = 1
	} else if s.DOS == nil || s.Routine != call.Routine {
		return NativeCommandFrameResult{}, fmt.Errorf("native runtime file routine changed during wait")
	}
	base := supplied.NativeFileFrameCallbacks
	base.Frame = call.Frame
	base.Code = h.Memory.Code
	base.Memory = h.Memory.BSS
	base.CodeBase = h.Memory.CodeBase
	base.Presentation = h.Session.Presentation
	base.Bitmap = h.Bitmap
	base.ReadAbsolute = func(at uint32) (uint8, error) { return h.Memory.RAM.Read8(int(at)) }
	cb := NativeDOSFrameCallbacks{Code: base.Code, Memory: base.Memory, CodeBase: base.CodeBase, Frame: call.Frame, Port: store.Port()}
	cb.Call = func(child NativeFileFrameCall, inner *uint32) (NativeCommandFrameResult, error) {
		switch child.Routine {
		case 0x341e:
			if s.Overwrite == nil {
				s.OverwriteA0 = s.DOS.A[0]
				s.Overwrite = &NativeRuntimeFileOverwriteState{}
			}
			step, e := s.Overwrite.Advance(base, &s.DOS.A)
			if step.Complete {
				s.DOS.A[0] = s.OverwriteA0 // $19b7c restores the caller's path.
				s.Overwrite = nil
			}
			return NativeCommandFrameResult{Complete: step.Complete, Zero: call.Frame.D[0] == 0}, e
		case 0x1a32a:
			if s.Resource == nil {
				s.Resource = &NativeResourceHostFrameState{Landscape: true}
			}
			resource, e := h.ResourceCallbacks(call.Frame, NativeErrorFrameCallbacks{NativeFileFrameCallbacks: base, RAM: h.Memory.RAM})
			if e != nil {
				return NativeCommandFrameResult{}, e
			}
			resource.ResourceCallerA = s.DOS.A
			step, e := s.Resource.Advance(&h.ResourceRules, resource)
			if step.Complete {
				s.Resource = nil
			}
			return NativeCommandFrameResult{Complete: step.Complete}, e
		default:
			if supplied.Child == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native runtime file child $%x missing", child.Routine)
			}
			return supplied.Child(NativeStartupResetFrameCall{Routine: child.Routine, Frame: call.Frame, A: &s.DOS.A}, inner)
		}
	}
	step, e := s.DOS.Advance(call.Routine, *call.A, cb)
	*call.A = s.DOS.A
	if e != nil {
		return NativeCommandFrameResult{}, e
	}
	if call.Routine == 0x19c1c && int32(s.DOS.Transferred) > 0 && s.DOS.Transferred <= NativeGAMSize {
		s.RefreshPending = true
	}
	if step.Complete {
		if call.Routine != 0x19936 {
			// $19c16/$19cca restore the outer D1-D7/A0-A6 MOVEM.
			*call.A = s.OuterA
		}
		s.DOS = nil
	}
	return NativeCommandFrameResult{Complete: step.Complete}, nil
}

// RefreshLoadedViews is a post-return cache operation, not another load.
// Invoke after source $3f92 has restored its two absolute selected pointers
// and the surrounding native frame has released borrowed raw ownership.
func (s *NativeRuntimeFilesState) RefreshLoadedViews(h *NativeRuntimeHost) error {
	if s == nil || h == nil || h.Memory == nil || h.World == nil || h.Session == nil {
		return fmt.Errorf("native runtime load refresh context missing")
	}
	if !s.RefreshPending {
		return nil
	}
	if s.DOS != nil || h.World.nativeCallDepth != 0 || h.Session.world != nil || h.Session.Phase != NativeFrameSessionIdle {
		return fmt.Errorf("native loaded views require completed idle ownership")
	}
	if e := refreshNativeRuntimeLoadedViews(h); e != nil {
		return e
	}
	s.RefreshPending = false
	return nil
}
