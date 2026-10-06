package populous2

import "fmt"

type NativeGameplayScreenExportCall struct {
	Operation, Name string
	Handle          uint32
	Data            []byte
	Mode            uint32
	Frame           *NativeFrameRegisterContext
}
type NativeGameplayScreenExportResult struct {
	Complete bool
	Value    int32
}
type NativeGameplayScreenExportCallbacks struct {
	NativeGameplayEditorInputCallbacks
	IO func(NativeGameplayScreenExportCall, *uint32) (NativeGameplayScreenExportResult, error)
}
type NativeGameplayScreenExportState struct {
	Started, Finished          bool
	PC                         int
	Registers, SavedD, IOSaved [8]uint32
	A, SavedA, IOSavedA        [7]NativeRequesterAddress
	Handle                     uint32
	IOActive                   bool
	IOPhase                    uint32
	Call                       NativeGameplayScreenExportCall
	failed                     error
}

// Advance executes1A55A/101B4 exactly: paletteheader, actual200×4 planarrows,
// nativeDOScounts/errors and the once-only filename-byte increment. HostIO
// is an explicit operation, never a hidden disk write or success placeholder.
func (s *NativeGameplayScreenExportState) Advance(cb NativeGameplayScreenExportCallbacks) (out NativeGameplayEditorInputStep, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) || !winMemoryValid(cb.RAM) || cb.IO == nil {
		return out, fmt.Errorf("native screenexport backing/IOmissing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x1a55a
		s.Registers = cb.Frame.D
		s.SavedD = cb.Frame.D
		s.SavedA = s.A
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	c := cb.Frame
	ca := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	io := func(operation string) (bool, error) {
		if !s.IOActive {
			s.IOActive = true
			s.IOSaved = c.D
			s.IOSavedA = s.A
			s.Call = NativeGameplayScreenExportCall{Operation: operation, Handle: s.Handle, Frame: c}
			switch operation {
			case "open":
				for at := s.A[1].Address; ; at++ {
					v, e := cb.RAM.Read8(int(at))
					if e != nil {
						return false, e
					}
					if v == 0 {
						break
					}
					s.Call.Name += string(v)
				}
				s.Call.Mode = 1006
				c.D[1], c.D[2] = s.A[1].Address, 1006
				library, e := cb.Memory.Read32(0x14c)
				if e != nil {
					return false, e
				}
				s.A[6] = NativeRequesterAddress{Address: library, Absolute: true}
			case "write":
				address, count := cb.CodeBase+0x1027c, 104
				if s.PC == 0x1022e {
					address, count = s.A[0].Address, 40
				}
				s.Call.Data = make([]byte, count)
				for i := range s.Call.Data {
					v, e := cb.RAM.Read8(int(address) + i)
					if e != nil {
						return false, e
					}
					s.Call.Data[i] = v
				}
				c.D[1], c.D[2], c.D[3] = s.Handle, address, uint32(count)
			case "close":
				c.D[1] = s.Handle
			}
		}
		if s.Call.Operation != operation {
			return false, fmt.Errorf("native screenexport IOchangedinwait")
		}
		library, e := cb.Memory.Read32(0x14c)
		if e != nil {
			return false, e
		}
		s.A[6] = NativeRequesterAddress{Address: library, Absolute: true}
		s.Call.Frame = c
		result, e := cb.IO(s.Call, &s.IOPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			return false, nil
		}
		copy(c.D[1:], s.IOSaved[1:])
		s.A = s.IOSavedA
		c.D[0] = uint32(result.Value)
		s.IOActive = false
		s.IOPhase = 0
		return true, nil
	}
	for transitions := 0; transitions < 1000; transitions++ {
		switch s.PC {
		case 0x1a55a:
			target, e := cb.Memory.Read32(0x1a)
			if e != nil {
				return out, e
			}
			s.A[0] = NativeRequesterAddress{Address: target, Chip: true}
			s.A[1], s.A[2] = ca(0x1a584), ca(0x33844)
			s.PC = 0x101b4
		case 0x101b4:
			done, e := io("open")
			if e != nil {
				return out, e
			}
			if !done {
				return out, nil
			}
			s.Handle = c.D[0]
			c.D[7] = c.D[0]
			s.A[3] = ca(0x102ac)
			s.A[1] = s.A[3]
			c.D[6] = 15
			for i := 0; i < 16; i++ {
				v, e := cb.RAM.Read16(int(s.A[2].Address))
				if e != nil {
					return out, e
				}
				s.A[2].Address += 2
				c.Word(0, v)
				c.Word(1, uint16(c.D[0])&0xf0)
				c.Word(0, uint16(c.D[0])^uint16(c.D[1]))
				v = uint16(c.D[0])
				c.Word(0, v>>4|v<<12)
				for _, value := range []byte{uint8(c.D[0]), uint8(c.D[1]), uint8(uint16(c.D[0]) >> 8)} {
					if e = cb.RAM.Write8(int(s.A[3].Address), value); e != nil {
						return out, e
					}
					s.A[3].Address++
				}
				c.Word(0, uint16(c.D[0])>>8)
				c.Word(6, uint16(c.D[6])-1)
			}
			s.PC = 0x10202
		case 0x10202:
			done, e := io("write")
			if e != nil {
				return out, e
			}
			if !done {
				return out, nil
			}
			c.D[6] = 199
			c.D[5] = 3
			s.PC = 0x1022e
		case 0x1022e:
			done, e := io("write")
			if e != nil {
				return out, e
			}
			if !done {
				return out, nil
			}
			s.A[0].Address += 8000
			c.Word(5, uint16(c.D[5])-1)
			if uint16(c.D[5]) != 0xffff {
				continue
			}
			s.A[0].Address = uint32(int64(s.A[0].Address) + int64(-31960))
			c.Word(6, uint16(c.D[6])-1)
			if uint16(c.D[6]) != 0xffff {
				c.D[5] = 3
				continue
			}
			s.PC = 0x10260
		case 0x10260:
			done, e := io("close")
			if e != nil {
				return out, e
			}
			if !done {
				return out, nil
			}
			v, e := cb.Code.Read8(0x1a587)
			if e != nil {
				return out, e
			}
			if e = cb.Code.Write8(0x1a587, v+1); e != nil {
				return out, e
			}
			c.D = s.SavedD
			s.A = s.SavedA
			s.Finished = true
			s.PC = 0x1a582
			out.Complete = true
			return out, nil
		default:
			return out, fmt.Errorf("native screenexport PC%x unsupported", s.PC)
		}
	}
	return out, nil
}
