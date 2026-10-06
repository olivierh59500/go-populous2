package populous2

import "fmt"

// SerialChild supplies actual byte/ring/configuration operations to4984.
// A0 resolves real physical bytes; temporary CR words use the explicitly
// provided stack payload and source D0 count, not an extra modem terminator.
func (s *NativeRuntimeTransport) SerialChild(call NativeSerialFrameCall, phase *uint32) (NativeSerialFrameChildResult, error) {
	if phase == nil {
		return NativeSerialFrameChildResult{}, fmt.Errorf("native runtime serial child phase missing")
	}
	cb, err := s.controlCallbacks(call.Frame)
	if err != nil {
		return NativeSerialFrameChildResult{}, err
	}
	h := s.Host
	switch call.Routine {
	case 0xab4:
		baud, err := h.Memory.BSS.Read16(0x15a)
		if err != nil {
			return NativeSerialFrameChildResult{}, err
		}
		if err := s.Conn.Port().Configure(baud); err != nil {
			return NativeSerialFrameChildResult{}, err
		}
		return NativeSerialFrameChildResult{Complete: true}, nil
	case 0xae2:
		result, err := transportFrameAvailable(cb)
		return result.NativeSerialFrameChildResult, err
	case 0x1826e:
		err := DisconnectNativeSerial(NativeSerialCallbacks{Memory: h.Memory.BSS, Port: s.Conn.Port()})
		if err == nil {
			err = transportFrameSyncReceive(cb)
		}
		return NativeSerialFrameChildResult{Complete: err == nil}, err
	case 0x17eec:
		if *phase == 0 {
			s.Handshake = NativeTransportHandshakeState{}
			*phase = 1
		}
		step, err := s.AdvanceHandshake(call.Frame)
		return step.NativeSerialFrameChildResult, err
	case 0xbbe, 0xc2a:
		if *phase == 0 {
			count := int(uint16(call.Frame.D[0]))
			if count == 0 && call.Routine == 0xc2a {
				count = 65536
			}
			if count == 0 {
				return NativeSerialFrameChildResult{}, fmt.Errorf("native serial NUL receive requires its unbounded source entry")
			}
			data := make([]byte, count)
			if len(call.StackBytes) > 0 {
				if len(call.StackBytes) < count {
					return NativeSerialFrameChildResult{}, fmt.Errorf("native serial temporary word payload too short")
				}
				copy(data, call.StackBytes[:count])
			} else if call.Routine == 0xc2a {
				for i := range data {
					value, err := h.Memory.RAM.Read8(int(call.A[0].Address) + i)
					if err != nil {
						return NativeSerialFrameChildResult{}, err
					}
					data[i] = value
				}
			}
			s.Transfer = NativeTransportIOState{Data: data}
			*phase = 1
		}
		if s.Transfer.Started && s.Transfer.Routine != call.Routine {
			return NativeSerialFrameChildResult{}, fmt.Errorf("native serial IO changed while pending")
		}
		previous := s.Transfer.Position
		step, err := s.Transfer.Advance(call.Routine, s.Transfer.Data, cb)
		if call.Routine == 0xbbe {
			for i := previous; i < s.Transfer.Position; i++ {
				if e := h.Memory.RAM.Write8(int(call.A[0].Address)+i, s.Transfer.Data[i]); err == nil {
					err = e
				}
			}
		}
		return step, err
	default:
		return NativeSerialFrameChildResult{}, fmt.Errorf("native runtime serial operation%x unavailable", call.Routine)
	}
}
