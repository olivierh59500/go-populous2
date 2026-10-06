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
