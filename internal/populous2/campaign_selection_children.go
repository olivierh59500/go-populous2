package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeCampaignSelectionChildrenRules struct {
	Render    NativeRenderFrameRules
	Panel     NativeProfilePanelFrameRules
	Audio     NativeAudioControlFrameRules
	Resources NativeResourceFrameRules
}

func DecodeNativeCampaignSelectionChildrenRules(exe *amiga.Executable) (NativeCampaignSelectionChildrenRules, error) {
	var r NativeCampaignSelectionChildrenRules
	var e error
	r.Render, e = DecodeNativeRenderFrameRules(exe)
	if e == nil {
		r.Panel, e = DecodeNativeProfilePanelFrameRules(exe)
	}
	if e == nil {
		r.Audio, e = DecodeNativeAudioControlFrameRules(exe)
	}
	if e == nil {
		r.Resources, e = DecodeNativeResourceFrameRules(exe)
	}
	return r, e
}

type NativeCampaignSelectionChildrenCallbacks struct {
	NativeCampaignHelpFrameCallbacks
	ResourceIO func(NativeResourceFrameIOCall, *uint32) (NativeResourceFrameIOResult, error)
}

// NativeCampaignSelectionChildren retains one genuine child operation while
// $3cba is suspended. Every operation uses the same CODE/RAM/input/display
// owner; a child return never substitutes typed campaign or UI defaults.
type NativeCampaignSelectionChildren struct {
	Routine         int
	Opponent        NativeCampaignOpponentFrameState
	Help            NativeCampaignHelpFrameState
	Error           NativeErrorFrameState
	Palette         *NativeFramePaletteState
	Resource        NativeResourceHostFrameState
	ResourceActive  bool
	ResourceRoutine int
	ResourceA       [7]NativeRequesterAddress
}

func (s *NativeCampaignSelectionChildren) Call(r *NativeCampaignSelectionChildrenRules, cb NativeCampaignSelectionChildrenCallbacks, call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
	if s == nil || r == nil || phase == nil || call.A == nil || call.Frame == nil || call.Frame != cb.Frame {
		return NativeCommandFrameResult{}, fmt.Errorf("native campaign child shared frame/address missing")
	}
	if *phase == 0 {
		s.Routine = call.Routine
		*phase = 1
		switch call.Routine {
		case 0xaf82:
			s.Opponent = NativeCampaignOpponentFrameState{A: *call.A}
		case 0x517a:
			s.Help = NativeCampaignHelpFrameState{A: *call.A}
		case 0x33b2:
			s.Error = NativeErrorFrameState{Routine: 0x33b2, A: *call.A}
		}
	} else if s.Routine != call.Routine {
		return NativeCommandFrameResult{}, fmt.Errorf("native campaign child routine changed during wait")
	}
	backing := cb.NativeCampaignFrameCallbacks
	backing.Child = func(inner NativeStartupResetFrameCall, childPhase *uint32) (NativeCommandFrameResult, error) {
		if inner.Routine != 0x19cd0 && inner.Routine != 0x1a32a {
			return NativeCommandFrameResult{}, fmt.Errorf("native campaign nestedchild%x unsupported", inner.Routine)
		}
		if cb.ResourceIO == nil {
			return NativeCommandFrameResult{}, fmt.Errorf("native campaign actual resourceIO missing")
		}
		if !s.ResourceActive {
			s.ResourceActive = true
			s.ResourceRoutine = inner.Routine
			s.Resource = NativeResourceHostFrameState{Landscape: inner.Routine == 0x1a32a}
			s.ResourceA = *inner.A
		}
		if s.ResourceRoutine != inner.Routine {
			return NativeCommandFrameResult{}, fmt.Errorf("native campaign resource changed during wait")
		}
		result, e := s.Resource.Advance(&r.Resources, NativeResourceHostFrameCallbacks{NativeErrorFrameCallbacks: NativeErrorFrameCallbacks{NativeFileFrameCallbacks: cb.NativeFileFrameCallbacks, RAM: cb.RAM}, IO: cb.ResourceIO, ResourceCallerA: s.ResourceA})
		if e != nil {
			return NativeCommandFrameResult{}, e
		}
		if result.Complete {
			s.ResourceActive = false
			*inner.A = s.ResourceA
		}
		return NativeCommandFrameResult{Complete: result.Complete}, nil
	}
	var done bool
	var e error
	switch call.Routine {
	case 0x111ae:
		plan, err := r.Panel.SwitchProfile(cb.Memory, cb.Frame)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		call.A[0] = NativeRequesterAddress{Address: plan.A0}
		call.A[1] = NativeRequesterAddress{Address: plan.A1}
		done = true
	case 0xaf82:
		step, err := s.Opponent.Advance(&r.Render, backing)
		done, e = step.Complete, err
		*call.A = s.Opponent.A
	case 0x517a:
		help := cb.NativeCampaignHelpFrameCallbacks
		help.NativeCampaignFrameCallbacks = backing
		step, err := s.Help.Advance(&r.Render, &r.Audio, help)
		done, e = step.Complete, err
		*call.A = s.Help.A
	case 0x33b2:
		step, err := s.Error.Advance(NativeErrorFrameCallbacks{NativeFileFrameCallbacks: cb.NativeFileFrameCallbacks, RAM: cb.RAM})
		done, e = step.Complete, err
		*call.A = s.Error.A
	case 0x102e4:
		if s.Palette == nil {
			bank := func(address uint32) (NativeFramePaletteBank, error) {
				b := NativeFramePaletteBank{Address: address}
				for i := range b.Words {
					v, e := cb.RAM.Read16(int(address) + i*2)
					if e != nil {
						return b, e
					}
					b.Words[i] = v
				}
				return b, nil
			}
			source, err := bank(call.A[2].Address)
			if err != nil {
				return NativeCommandFrameResult{}, err
			}
			target, err := bank(call.A[3].Address)
			if err != nil {
				return NativeCommandFrameResult{}, err
			}
			s.Palette = NewNativeFramePaletteState(source, target, cb.CodeBase)
		}
		done, e = s.Palette.Advance(cb.Presentation, cb.Frame, cb.Memory)
		at := uint32(0x34)
		if done {
			at = 0x74
			s.Palette = nil
		}
		call.A[0] = NativeRequesterAddress{Address: cb.Presentation.ChipBase + at, Chip: true}
		call.A[1] = NativeRequesterAddress{Address: cb.Presentation.ChipBase + at + 0x200, Chip: true}
	case 0x19cd0, 0x1a32a:
		return backing.Child(call, phase)
	default:
		return NativeCommandFrameResult{}, fmt.Errorf("native campaign child%x unsupported", call.Routine)
	}
	if e != nil {
		return NativeCommandFrameResult{}, e
	}
	return NativeCommandFrameResult{Complete: done}, nil
}
