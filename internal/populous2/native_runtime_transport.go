package populous2

import "fmt"

// NativeRuntimeTransport binds source serial controllers to the shared
// physical runtime and one real ordered connection. Host delays and genuine
// startup/menu children remain explicit, not fabricated successful returns.
type NativeRuntimeTransport struct {
	Host      *NativeRuntimeHost
	Conn      *NativeSerialConn
	Handshake NativeTransportHandshakeState
	Resume    NativeTransportResumeState
	Transfer  NativeTransportIOState
	packets   [2]*NativeSerialPacket
	mismatch  *NativeErrorFrameState
	palette   *NativeFramePaletteState
	Callbacks NativeTransportFrameCallbacks
	Ownership func(bool, *NativeFrameRegisterContext) error
}

func (h *NativeRuntimeHost) NewTransport(conn *NativeSerialConn, cb NativeTransportFrameCallbacks, ownership func(bool, *NativeFrameRegisterContext) error) (*NativeRuntimeTransport, error) {
	if h == nil || h.Memory == nil || conn == nil || ownership == nil {
		return nil, fmt.Errorf("native runtime transport owners missing")
	}
	cb.Memory, cb.Code, cb.CodeBase = h.Memory.BSS, h.Memory.Code, h.Memory.CodeBase
	cb.Presentation, cb.Bitmap = h.Session.Presentation, h.Bitmap
	cb.Port, cb.ReceiveImage = conn.Port(), conn.NativeTransportReceiveImage
	cb.ReadAbsolute = func(address uint32) (uint8, error) { return h.Memory.RAM.Read8(int(address)) }
	return &NativeRuntimeTransport{Host: h, Conn: conn, Callbacks: cb, Ownership: ownership}, nil
}

// PacketCallback belongs to the source1744C transport boundary. It retains a
// partial sender/receiver packet and the genuine33B2 mismatch dialog. Commands
// are executed by the parent only after this callback returns complete.
func (s *NativeRuntimeTransport) PacketCallback(caller int, mode uint8, c *NativeCommandRegisterContext, phase *uint32) (bool, error) {
	if s == nil || s.Host == nil || s.Conn == nil || c == nil || phase == nil {
		return false, fmt.Errorf("native runtime packet context missing")
	}
	index := (caller - 0xeb56) / 10
	if index < 0 || index >= 2 || caller != 0xeb56+index*10 {
		return false, fmt.Errorf("native runtime packet caller outside source records")
	}
	h := s.Host
	if *phase == 0 {
		packet, err := NewNativeSerialPacket(caller, mode)
		if err != nil {
			return false, err
		}
		s.packets[index] = packet
		*phase = 1
	}
	packet := s.packets[index]
	if packet == nil || packet.Caller != caller || packet.Mode != mode {
		return false, fmt.Errorf("native runtime packet changed while pending")
	}
	frame := NativeFrameRegisterContext{D: c.D, AddressBase: h.Memory.BSSBase}
	defer func() { c.D = frame.D }()
	ownedD := frame.D
	if err := s.Ownership(false, &frame); err != nil {
		return false, err
	}
	frame.D = ownedD
	acquired := false
	step, err := packet.Advance(c, NativeSerialCallbacks{Memory: h.Memory.BSS, Port: s.Conn.Port(), Message: func(kind NativeSerialMessage) (bool, error) {
		if kind != NativeSerialLandscapeMismatch {
			return false, fmt.Errorf("native packet requester kind unavailable")
		}
		if !acquired {
			frame.D = c.D
			saved := frame.D
			if err := s.Ownership(true, &frame); err != nil {
				return false, err
			}
			frame.D = saved
			acquired = true
		}
		if s.mismatch == nil {
			s.mismatch = &NativeErrorFrameState{Routine: 0x33b2}
			s.mismatch.A[1] = NativeRequesterAddress{Address: h.Memory.CodeBase + 0x86f0, Code: true}
			s.mismatch.A[2] = NativeRequesterAddress{Address: h.Memory.CodeBase + 0xa9ea, Code: true}
		}
		frame.D = c.D
		cb := s.Callbacks.NativeFileFrameCallbacks
		cb.Frame = &frame
		result, err := s.mismatch.Advance(NativeErrorFrameCallbacks{NativeFileFrameCallbacks: cb, RAM: h.Memory.RAM})
		c.D = frame.D
		if result.Complete {
			s.mismatch = nil
		}
		return result.Complete, err
	}})
	frame.D = c.D
	ownedD = frame.D
	if !acquired {
		if e := s.Ownership(true, &frame); err == nil {
			err = e
		}
	}
	frame.D = ownedD
	if err != nil {
		return false, err
	}
	if step.Complete {
		if e := transportFrameSyncReceive(NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Memory: h.Memory.BSS}, ReceiveImage: s.Conn.NativeTransportReceiveImage}); e != nil {
			return false, e
		}
		s.packets[index] = nil
		*phase = 2
	}
	return step.Complete, nil
}
