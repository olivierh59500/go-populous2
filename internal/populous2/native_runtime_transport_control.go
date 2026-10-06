package populous2

import "fmt"

func (s *NativeRuntimeTransport) controlCallbacks(frame *NativeFrameRegisterContext) (NativeTransportFrameCallbacks, error) {
	if s == nil || s.Host == nil || s.Conn == nil || frame == nil || frame.AddressBase != s.Host.Memory.BSSBase {
		return NativeTransportFrameCallbacks{}, fmt.Errorf("native runtime transport frame/base missing")
	}
	cb := s.Callbacks
	cb.Frame = frame
	external := cb.CallTransport
	cb.CallTransport = func(call NativeFileFrameCall, phase *uint32) (NativeSerialFrameChildResult, error) {
		if call.Routine == 0x111ae {
			rules, err := DecodeNativeProfilePanelFrameRules(s.Host.Bundle.Executable)
			if err != nil {
				return NativeSerialFrameChildResult{}, err
			}
			_, err = rules.SwitchProfile(s.Host.Memory.BSS, call.Frame)
			return NativeSerialFrameChildResult{Complete: err == nil}, err
		}
		if external == nil {
			return NativeSerialFrameChildResult{}, fmt.Errorf("native transport child%x missing", call.Routine)
		}
		return external(call, phase)
	}
	return cb, nil
}

// AdvanceHandshake executes the actual17EEC retained body. WaitCPU and
// CallTransport supply measured/configured host waits and genuine children;
// a missing world reset never becomes an established game.
func (s *NativeRuntimeTransport) AdvanceHandshake(frame *NativeFrameRegisterContext) (NativeTransportFrameStep, error) {
	cb, err := s.controlCallbacks(frame)
	if err != nil {
		return NativeTransportFrameStep{}, err
	}
	return s.Handshake.Advance(cb)
}

func (s *NativeRuntimeTransport) AdvanceResume(frame *NativeFrameRegisterContext) (NativeTransportFrameStep, error) {
	cb, err := s.controlCallbacks(frame)
	if err != nil {
		return NativeTransportFrameStep{}, err
	}
	return s.Resume.Advance(cb)
}

// ResumeMenuChild connects the original menu's181C0 call to one retained
// resume invocation. A later menu visit starts another invocation, while a
// pending transfer keeps its packet and registers. The source corrupt RTS
// path cannot be represented as a successful Go child return.
func (s *NativeRuntimeTransport) ResumeMenuChild(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
	if s == nil || call.Routine != 0x181c0 || call.Frame == nil || phase == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native menu resume context missing")
	}
	if *phase == 0 {
		s.Resume = NativeTransportResumeState{}
		*phase = 1
	}
	step, err := s.AdvanceResume(call.Frame)
	if err != nil {
		return NativeCommandFrameResult{}, err
	}
	if step.CorruptReturn {
		return NativeCommandFrameResult{}, fmt.Errorf("native menu resume corrupt RTS to%08x: %v", step.NativeStackReturn, step.Failure)
	}
	if step.Complete && !step.FlagsKnown {
		return NativeCommandFrameResult{}, fmt.Errorf("native menu resume omitted source condition flags")
	}
	return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, nil
}
