package populous2

import (
	"encoding/binary"
	"fmt"
)

type NativeTransportFrameStep struct {
	NativeSerialFrameChildResult
	Waiting           bool
	FlagsKnown        bool
	PC                int
	Failure           error
	NativeStackReturn uint32
	CorruptReturn     bool
}

// NativeTransportResumeState follows $181c0 with its actual ten-byte stack
// packet. The original negative-availability branch leaves it unpopped; that
// physical RTS outcome is reported separately from a normal completed call.
type NativeTransportResumeState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	Packet            [10]byte
	PacketBase        uint32
	Transfer          NativeTransportIOState
	ChildActive       bool
	ChildPhase        uint32
	Failure           error
	failed            error
}

func (s *NativeTransportResumeState) Advance(cb NativeTransportFrameCallbacks) (out NativeTransportFrameStep, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return out, fmt.Errorf("native resume backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x181c0
		s.Registers = cb.Frame.D
		if cb.CallerStackPointer != 0 {
			s.PacketBase = cb.CallerStackPointer - 10
		}
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		out.PC = s.PC
		out.Failure = s.Failure
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		out.Zero = true
		out.FlagsKnown = true
		return out, nil
	}
	c, m := cb.Frame, cb.Memory
	b := nativeRequesterFrameBacking{Code: cb.Code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound}
	for {
		switch s.PC {
		case 0x181c0:
			pointer, e := m.Read32(0xeb6a)
			if e != nil {
				return out, e
			}
			mode, e := m.Read8(int(int64(pointer)-int64(c.AddressBase)) + 8)
			if e != nil {
				return out, e
			}
			if mode != 6 && mode != 8 {
				s.PC = 0x18262
				continue
			}
			if !validNativeSerialPort(cb.Port) {
				return out, fmt.Errorf("native resume real transport missing")
			}
			for i, at := range []int{0xeb42, 0xe90c, 0xea46, 0xeb2c, 0xeb2e} {
				value, e := m.Read16(at)
				if e != nil {
					return out, e
				}
				binary.BigEndian.PutUint16(s.Packet[i*2:], value)
			}
			c.D[0] = 10
			s.Transfer = NativeTransportIOState{}
			s.PC = 0x181fa
		case 0x181fa:
			result, e := s.Transfer.Advance(0xc2a, s.Packet[:], cb)
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			s.Failure = s.Transfer.Failure
			s.PC = 0x18200
		case 0x18200:
			message, e := cb.Code.Read32(0x1826a)
			if e != nil {
				return out, e
			}
			c.Word(3, 1)
			if e := b.compile(cb.CodeBase+0x91d0, []NativeRequesterAddress{{Address: message, Code: true}}); e != nil {
				return out, e
			}
			target, e := m.Read32(0x1e)
			if e != nil {
				return out, e
			}
			start, e := cb.Code.Read16(0xab4e)
			if e != nil {
				return out, e
			}
			col, e := cb.Code.Read16(0xab50)
			if e != nil {
				return out, e
			}
			row, e := cb.Code.Read16(0xab52)
			if e != nil {
				return out, e
			}
			c.Word(0, col)
			c.Word(1, row)
			if e := b.text(target, 0xab4e+int(int16(start))); e != nil {
				return out, e
			}
			if e := fileFrameSwap(b, cb.Presentation); e != nil {
				return out, e
			}
			s.PC = 0x1821c
		case 0x1821c:
			result, e := transportFrameAvailable(cb)
			if result.Failure != nil {
				s.Failure = result.Failure
			}
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			if result.Negative {
				s.PC = 0x18266
				continue
			}
			if int16(c.D[0]) < 10 {
				out.Waiting = true
				return out, nil
			}
			c.D[0] = 10
			s.Transfer = NativeTransportIOState{}
			s.PC = 0x18230
		case 0x18230:
			result, e := s.Transfer.Advance(0xbbe, s.Packet[:], cb)
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			s.Failure = s.Transfer.Failure
			c.Word(0, binary.BigEndian.Uint16(s.Packet[:]))
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			if uint16(c.D[0]) == profile {
				s.PC = 0x18240
			} else {
				s.PC = 0x18246
			}
		case 0x18240:
			if cb.Call == nil && cb.CallTransport == nil {
				return out, fmt.Errorf("native resume profile child111ae missing")
			}
			s.ChildActive = true
			var result NativeCommandFrameResult
			var e error
			call := NativeFileFrameCall{Routine: 0x111ae, Frame: c}
			if cb.CallTransport != nil {
				child, err := cb.CallTransport(call, &s.ChildPhase)
				result.Complete, e = child.Complete, err
			} else {
				result, e = cb.Call(call, &s.ChildPhase)
			}
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			s.ChildActive = false
			s.ChildPhase = 0
			s.PC = 0x18246
		case 0x18246:
			for i, at := range []int{0xe90c, 0xea46} {
				if e := m.Write16(at, binary.BigEndian.Uint16(s.Packet[2+i*2:])); e != nil {
					return out, e
				}
			}
			for i, at := range []int{0xeb2c, 0xeb2e} {
				value := binary.BigEndian.Uint16(s.Packet[6+i*2:])
				c.Word(0, value)
				old, e := m.Read16(at)
				if e != nil {
					return out, e
				}
				if e := m.Write16(at, old|value); e != nil {
					return out, e
				}
			}
			s.PC = 0x18262
		case 0x18262:
			c.D[0] = 0
			s.Finished = true
			out.Complete = true
			out.Zero = true
			out.FlagsKnown = true
			return out, nil
		case 0x18266:
			c.D[0] = 1
			out.Negative = false
			out.Zero = false
			out.FlagsKnown = true
			out.CorruptReturn = true
			out.NativeStackReturn = binary.BigEndian.Uint32(s.Packet[:])
			return out, nil
		default:
			return out, fmt.Errorf("native resume branch unsupported")
		}
	}
}
