package populous2

import "fmt"

func (s *NativeRuntimeTransport) controlCallbacks(frame *NativeFrameRegisterContext) (NativeTransportFrameCallbacks, error) {
	if s == nil || s.Host == nil || s.Conn == nil || frame == nil || frame.AddressBase != s.Host.Memory.BSSBase {
		return NativeTransportFrameCallbacks{}, fmt.Errorf("native runtime transport frame/base missing")
	}
	cb := s.Callbacks
	cb.Frame = frame
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
