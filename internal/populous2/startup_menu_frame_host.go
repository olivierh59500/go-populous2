package populous2

import "fmt"

// NativeStartupMenuHostFrameState binds the actual palette child and retains
// its seventeen VBlank phases. Help/file operations retain their supplied
// full-register host continuation; missing bodies are explicit errors.
type NativeStartupMenuHostFrameState struct {
	Menu    NativeStartupMenuFrameState
	Palette *NativeFramePaletteState
}

func (s *NativeStartupMenuHostFrameState) Advance(cb NativeCampaignFrameCallbacks) (NativeCampaignFrameStep, error) {
	if s == nil {
		return NativeCampaignFrameStep{}, fmt.Errorf("native startup menu host state missing")
	}
	external := cb.Child
	cb.Child = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x102e4 {
			if external == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native startup menu child%x has no actual host body", call.Routine)
			}
			return external(call, phase)
		}
		if s.Palette == nil {
			bank := func(address NativeRequesterAddress) (NativeFramePaletteBank, error) {
				value := NativeFramePaletteBank{Address: address.Address}
				if address.Address < cb.CodeBase || address.Address&1 != 0 {
					return value, fmt.Errorf("native startup palette outside aligned CODE")
				}
				for i := range value.Words {
					v, err := cb.Code.Read16(int(address.Address-cb.CodeBase) + i*2)
					if err != nil {
						return value, err
					}
					value.Words[i] = v
				}
				return value, nil
			}
			source, err := bank(call.A[2])
			if err != nil {
				return NativeCommandFrameResult{}, err
			}
			target, err := bank(call.A[3])
			if err != nil {
				return NativeCommandFrameResult{}, err
			}
			s.Palette = NewNativeFramePaletteState(source, target, cb.CodeBase)
		}
		done, err := s.Palette.Advance(cb.Presentation, call.Frame, cb.Memory)
		offset := uint32(0x34)
		if done {
			offset = 0x74
			s.Palette = nil
		}
		// Actual102E4's two palette-writing address cursors. D0 is restored
		// by its own MOVEM; the remaining source register outputs survive.
		call.A[0] = NativeRequesterAddress{Address: cb.Presentation.ChipBase + offset, Chip: true}
		call.A[1] = NativeRequesterAddress{Address: cb.Presentation.ChipBase + offset + 0x200, Chip: true}
		return NativeCommandFrameResult{Complete: done}, err
	}
	return s.Menu.Advance(cb)
}
