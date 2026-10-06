package populous2

import "fmt"

// GameplayInput binds the complete source suffix to the same runtime BSS,
// physical CODE and keyboard owner. Call supplies actual full-context child
// operations; the suffix cannot complete through missing modal bodies.
func (h *NativeRuntimeHost) GameplayInput(state *NativeGameplayInputState, supplied NativeStartupResetFrameCallbacks) (func(FollowerCleanupMemory, *NativeFrameRegisterContext, *uint32) (bool, error), error) {
	if h == nil || h.Session == nil || state == nil {
		return nil, fmt.Errorf("native runtime gameplay input owners missing")
	}
	keys, err := DecodeNativeInputRules(h.Bundle.Executable)
	if err != nil {
		return nil, err
	}
	return func(_ FollowerCleanupMemory, frame *NativeFrameRegisterContext, phase *uint32) (bool, error) {
		if frame == nil || phase == nil || frame.AddressBase != h.Memory.BSSBase {
			return false, fmt.Errorf("native runtime gameplay input frame/base missing")
		}
		if *phase == 0 {
			*state = NativeGameplayInputState{}
			*phase = 1
		} else if *phase != 1 {
			return false, fmt.Errorf("native runtime input phase unavailable")
		}
		base := supplied
		base.Frame, base.Memory, base.Code = frame, h.Memory.BSS, h.Memory.Code
		base.RAM, base.CodeBase = h.Memory.RAM, h.Memory.CodeBase
		step, err := state.Advance(NativeGameplayInputCallbacks{NativeStartupResetFrameCallbacks: base, Input: &h.Session.Presentation.Input, Keys: keys})
		if step.Complete {
			*phase = 2
		}
		return step.Complete, err
	}, nil
}
