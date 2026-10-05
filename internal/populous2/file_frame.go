package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFileFrameRules struct{ Keys NativeInputRules }

func DecodeNativeFileFrameRules(exe *amiga.Executable) (NativeFileFrameRules, error) {
	var r NativeFileFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x34868 {
		return r, fmt.Errorf("native file frame resources missing")
	}
	var e error
	r.Keys, e = DecodeNativeInputRules(exe)
	return r, e
}

// NativeFileFrameCall is an actual source child boundary. The callback owns
// its operation and register returns; completion and CCR.Z remain separate.
type NativeFileFrameCall struct {
	Routine   int
	Arguments uint8 // Source-assigned address registers supplied to this child.
	A         [7]NativeRequesterAddress
	Frame     *NativeFrameRegisterContext
}

func fileFrameArguments(routine int) uint8 {
	switch routine {
	case 0x19936:
		return 1<<0 | 1<<1 | 1<<4
	case 0x102e4:
		return 1<<2 | 1<<3
	case 0x19afc, 0x19c1c, 0xd8cc:
		return 1 << 0
	}
	return 0
}

type NativeFileFrameCallbacks struct {
	Code, Memory FollowerCleanupMemory
	CodeBase     uint32
	Frame        *NativeFrameRegisterContext
	Presentation *NativeFramePresentationState
	Bitmap       func(uint32) ([]byte, error)
	Sound        func(uint16, *NativeFrameRegisterContext) error
	// ReadAbsolute supplies actual physical RAM for the source's null row
	// pointer. An absent pointer is not replaced with an empty typed string.
	ReadAbsolute func(uint32) (uint8, error)
	Call         func(NativeFileFrameCall, *uint32) (NativeCommandFrameResult, error)
}

type NativeFileFrameStep struct {
	Complete, Idle, Waiting bool
	PC                      int
	Calls                   []int
}

// NativeFileFrameState is the original $3f92 file browser. CODE strings,
// list pointers and requester scratch are shared backing, not typed copies.
// A return to the idle polling edge yields to the host without completing
// the native child. A real text modal yields only at its unsatisfied $786.
type NativeFileFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildPhase        uint32
	ChildRoutine      int
	ChildA            [7]NativeRequesterAddress
	Modal             NativeFileTextModal
	modalReturn       int
	dialogPhase       uint8
	dialogFlag        uint16
	dialogSaved       [8]uint32
	failed            error
}

func (s *NativeFileFrameState) Advance(r *NativeFileFrameRules, cb NativeFileFrameCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return out, fmt.Errorf("native file frame state/backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x3f92
		s.Registers = cb.Frame.D
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		out.PC = s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	b := nativeRequesterFrameBacking{Code: cb.Code, Memory: cb.Memory, CodeBase: cb.CodeBase, Frame: cb.Frame, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	c, code, m := cb.Frame, cb.Code, cb.Memory
	caddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	baddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
	}
	child := func(routine int, next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native file frame child%x callback missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
			s.ChildA = s.A
			out.Calls = append(out.Calls, routine)
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native file frame child changed across suspension")
		}
		result, e := cb.Call(NativeFileFrameCall{Routine: routine, Arguments: fileFrameArguments(routine), A: s.ChildA, Frame: c}, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.ChildActive = false
		s.ChildPhase = 0
		s.PC = next
		return true, nil
	}
	palette := func(first, second, next int) (bool, error) {
		s.A[2], s.A[3] = caddr(first), caddr(second)
		return child(0x102e4, next)
	}
	copyScreen := func() error {
		if cb.Bitmap == nil {
			return fmt.Errorf("native file frame screen resolver missing")
		}
		front, e := m.Read32(0x1a)
		if e != nil {
			return e
		}
		back, e := m.Read32(0x1e)
		if e != nil {
			return e
		}
		source, e := cb.Bitmap(front)
		if e != nil {
			return e
		}
		target, e := cb.Bitmap(back)
		if e != nil {
			return e
		}
		if len(source) < 32000 || len(target) < 32000 {
			return fmt.Errorf("native file frame copy outside screen RAM")
		}
		// Each source MOVEM reads 32 bytes before writing them; the next
		// block observes any real overlap with that preceding write.
		for at := 0; at < 32000; at += 32 {
			var block [32]byte
			copy(block[:], source[at:at+32])
			copy(target[at:at+32], block[:])
		}
		return nil
	}
	draw := func() error {
		target, e := m.Read32(0x1e)
		if e != nil {
			return e
		}
		start, e := code.Read16(0xab4e)
		if e != nil {
			return e
		}
		column, e := code.Read16(0xab50)
		if e != nil {
			return e
		}
		row, e := code.Read16(0xab52)
		if e != nil {
			return e
		}
		c.Word(0, column)
		c.Word(1, row)
		if e := b.text(target, 0xab4e+int(int16(start))); e != nil {
			return e
		}
		return fileFrameSwap(b, cb.Presentation)
	}
	parameters := func() ([]NativeRequesterAddress, error) {
		p := make([]NativeRequesterAddress, 16)
		for i := range p {
			value, e := code.Read32(0x4380 + i*4)
			if e != nil {
				return nil, e
			}
			p[i] = NativeRequesterAddress{Address: value, Code: i == 0 || i >= 13}
		}
		return p, nil
	}
	startDialog := func() { s.dialogSaved = c.D; s.dialogPhase = 0; s.PC = 0x33b2 }
	for transitions := 0; transitions < 128; transitions++ {
		if s.Modal.Active {
			done, e := s.Modal.advance(b, cb.Presentation, r.Keys)
			if e != nil {
				return out, e
			}
			if !done {
				out.Waiting = true
				return out, nil
			}
			s.PC = s.modalReturn
		}
		switch s.PC {
		case 0x3f92:
			label := caddr(0xa972)
			verb, e := code.Read16(0x4468)
			if e != nil {
				return out, e
			}
			if verb != 0 {
				label = caddr(0xa977)
			}
			if e := code.Write32(0x4380, label.Address); e != nil {
				return out, e
			}
			if e := code.Write32(0x43bc, label.Address); e != nil {
				return out, e
			}
			c.D[1] = 11
			for at := 0x4384; at < 0x43b4; at += 4 {
				if e := code.Write32(at, 0); e != nil {
					return out, e
				}
				c.Word(1, uint16(c.D[1])-1)
			}
			c.D[3] = 1
			p, e := parameters()
			if e != nil {
				return out, e
			}
			if e := b.compile(caddr(0x7af2).Address, p); e != nil {
				return out, e
			}
			if e := draw(); e != nil {
				return out, e
			}
			if e := m.Write8(0x3be, 0); e != nil {
				return out, e
			}
			s.A[0], s.A[1], s.A[4] = caddr(0x4440), baddr(0x3be), baddr(0xd9e)
			s.PC = 0x3ffe
		case 0x3ffe:
			done, e := child(0x19936, 0x4004)
			if e != nil || !done {
				return out, e
			}
		case 0x4004:
			if e := code.Write16(0x4412, 0); e != nil {
				return out, e
			}
			if e := code.Write16(0x4414, 0); e != nil {
				return out, e
			}
			s.PC = 0x4010
		case 0x4010:
			list := baddr(0x3be)
			offset, e := code.Read16(0x4412)
			if e != nil {
				return out, e
			}
			c.Word(0, offset)
			if offset != 0 {
				c.Word(0, uint16(c.D[0])-1)
				for {
					for {
						v, e := b.read(list)
						if e != nil {
							return out, e
						}
						list.Address++
						if v == 0 {
							break
						}
					}
					c.Word(0, uint16(c.D[0])-1)
					if uint16(c.D[0]) == 0xffff {
						break
					}
				}
			}
			c.Word(0, 11)
			count, e := code.Read16(0x443e)
			if e != nil {
				return out, e
			}
			c.Word(1, count)
			for slot := 1; ; slot++ {
				if e := code.Write32(0x4380+slot*4, list.Address); e != nil {
					return out, e
				}
				for {
					v, e := b.read(list)
					if e != nil {
						return out, e
					}
					list.Address++
					if v == 0 {
						break
					}
				}
				previous := int16(c.D[1])
				c.Word(1, uint16(c.D[1])-1)
				if int32(previous)-1 <= 0 {
					break
				}
				c.Word(0, uint16(c.D[0])-1)
				if uint16(c.D[0]) == 0xffff {
					break
				}
			}
			c.D[3] = 1
			p, e := parameters()
			if e != nil {
				return out, e
			}
			if e := b.compile(caddr(0x7af2).Address, p); e != nil {
				return out, e
			}
			if e := copyScreen(); e != nil {
				return out, e
			}
			if e := draw(); e != nil {
				return out, e
			}
			if e := draw(); e != nil {
				return out, e
			}
			s.PC = 0x4088
		case 0x4082:
			if e := draw(); e != nil {
				return out, e
			}
			s.PC = 0x4088
		case 0x4088:
			end, e := b.click()
			if e != nil {
				return out, e
			}
			action := uint16(c.D[0])
			c.Word(1, action)
			offset, e := code.Read16(0x40b2 + int(int16(action)))
			if e != nil {
				return out, e
			}
			c.Word(0, offset)
			s.PC = 0x40b2 + int(int16(offset))
			if s.PC == 0x4082 {
				out.Idle = true
				return out, nil
			}
			if action == 30 || action == 32 {
				field := 0x4416
				s.modalReturn = 0x4010
				if action == 30 {
					field = 0x4440
					s.modalReturn = 0x3f92
				}
				if e := s.Modal.begin(b, field, int(int64(end)-int64(cb.CodeBase))); e != nil {
					return out, e
				}
			}
		case 0x40d8:
			c.D[0] = 0
			s.Finished = true
			out.Complete = true
			return out, nil
		case 0x40ae:
			c.D[0] = 1
			s.Finished = true
			out.Complete = true
			return out, nil
		case 0x40dc:
			value, e := code.Read16(0x4412)
			if e != nil {
				return out, e
			}
			c.Word(0, value)
			if value != 0 {
				c.Word(0, value-1)
				if e := code.Write16(0x4412, uint16(c.D[0])); e != nil {
					return out, e
				}
			}
			s.PC = 0x4010
		case 0x40f4, 0x4106, 0x4118, 0x412a, 0x413a, 0x414a, 0x415a, 0x416a, 0x417a, 0x418a, 0x419a, 0x41aa:
			slot := 0
			for i, at := range []int{0x40f4, 0x4106, 0x4118, 0x412a, 0x413a, 0x414a, 0x415a, 0x416a, 0x417a, 0x418a, 0x419a, 0x41aa} {
				if s.PC == at {
					slot = i
					break
				}
			}
			pointer, e := code.Read32(0x4384 + slot*4)
			if e != nil {
				return out, e
			}
			if e := code.Write16(0x4414, uint16(slot)); e != nil {
				return out, e
			}
			from, to := NativeRequesterAddress{Address: pointer, Absolute: pointer == 0}, 0x4416
			for {
				v, e := b.read(from)
				if e != nil {
					return out, e
				}
				from.Address++
				if e := code.Write8(to, v); e != nil {
					return out, e
				}
				to++
				if v == 0 {
					break
				}
			}
			s.PC = 0x4010
		case 0x41c6:
			value, e := code.Read16(0x4412)
			if e != nil {
				return out, e
			}
			c.Word(0, value)
			c.Word(0, uint16(c.D[0])+1)
			count, e := code.Read16(0x443e)
			if e != nil {
				return out, e
			}
			c.Word(1, count)
			c.Word(1, uint16(c.D[1])-12)
			if int32(int16(count))-12 > 0 {
				if int16(c.D[0]) >= int16(c.D[1]) {
					c.Word(0, uint16(c.D[1]))
				}
				if e := code.Write16(0x4412, uint16(c.D[0])); e != nil {
					return out, e
				}
			}
			s.PC = 0x4010
		case 0x41ee, 0x41fe:
			// The text modal was entered from the genuine click's A0.
			if !s.Modal.Active {
				return out, fmt.Errorf("native file edit reached without actual click origin")
			}
		case 0x420e:
			end := 0x4416
			for {
				v, e := code.Read8(end)
				if e != nil {
					return out, e
				}
				end++
				if v == 0 {
					break
				}
			}
			writeAt, compare := end-1, end-1
			suffix := 0xab49
			for {
				v, e := code.Read8(suffix)
				if e != nil {
					return out, e
				}
				suffix++
				c.Byte(0, v)
				if v == 0 {
					break
				}
				compare--
				got, e := code.Read8(compare)
				if e != nil {
					return out, e
				}
				if got != v {
					c.D[0] = 3
					for source := 0xab4c; source >= 0xab49; source-- {
						v, e := code.Read8(source)
						if e != nil {
							return out, e
						}
						if e := code.Write8(writeAt, v); e != nil {
							return out, e
						}
						writeAt++
						c.Word(0, uint16(c.D[0])-1)
					}
					if e := code.Write8(writeAt, 0); e != nil {
						return out, e
					}
					break
				}
			}
			to, from := 0x43c0, 0x4440
			first, e := code.Read8(from)
			if e != nil {
				return out, e
			}
			if first != 0 {
				for {
					v, e := code.Read8(from)
					if e != nil {
						return out, e
					}
					from++
					if e := code.Write8(to, v); e != nil {
						return out, e
					}
					to++
					if v == 0 {
						break
					}
				}
				to--
				last, e := code.Read8(to - 1)
				if e != nil {
					return out, e
				}
				if last != ':' && last != '/' {
					if e := code.Write8(to, '/'); e != nil {
						return out, e
					}
					to++
					if e := code.Write8(to, 0); e != nil {
						return out, e
					}
				}
			}
			for from = 0x4416; ; {
				v, e := code.Read8(from)
				if e != nil {
					return out, e
				}
				from++
				if e := code.Write8(to, v); e != nil {
					return out, e
				}
				to++
				if v == 0 {
					break
				}
			}
			s.A[0] = caddr(0x43c0)
			verb, e := code.Read16(0x4468)
			if e != nil {
				return out, e
			}
			if verb != 0 {
				s.PC = 0x4324
			} else {
				s.PC = 0x4288
			}
		case 0x4288:
			paletteAt := 0x33844
			alternate, e := code.Read16(0x3f90)
			if e != nil {
				return out, e
			}
			if alternate != 0 {
				paletteAt = 0x3c528
			}
			done, e := palette(paletteAt, 0x3361a, 0x42b0)
			if e != nil || !done {
				return out, e
			}
		case 0x42b0:
			done, e := child(0x19c1c, 0x42b6)
			if e != nil || !done {
				return out, e
			}
		case 0x42b6:
			paletteAt := 0x33844
			alternate, e := code.Read16(0x3f90)
			if e != nil {
				return out, e
			}
			if alternate != 0 {
				paletteAt = 0x3c528
			}
			done, e := palette(0x3361a, paletteAt, 0x42d6)
			if e != nil || !done {
				return out, e
			}
		case 0x42d6:
			if uint16(c.D[0]) == 0 {
				startDialog()
				break
			}
			for _, at := range []int{0xf32, 0xf36} {
				value, e := m.Read32(at)
				if e != nil {
					return out, e
				}
				if value != 0 {
					if e := m.Write32(at, value+c.AddressBase+0x76c0); e != nil {
						return out, e
					}
				}
			}
			target, e := m.Read32(0x22)
			if e != nil {
				return out, e
			}
			s.A[0] = NativeRequesterAddress{Address: target, Chip: true}
			s.PC = 0x4306
		case 0x4306:
			done, e := child(0xd8cc, 0x430c)
			if e != nil || !done {
				return out, e
			}
		case 0x430c:
			done, e := child(0x1da0, 0x4312)
			if e != nil || !done {
				return out, e
			}
		case 0x4312:
			c.D[6], c.D[7] = 0, 0x00400040
			s.PC = 0x431a
		case 0x431a:
			done, e := child(0xd838, 0x40ae)
			if e != nil || !done {
				return out, e
			}
		case 0x4324:
			for _, at := range []int{0xf32, 0xf36} {
				value, e := m.Read32(at)
				if e != nil {
					return out, e
				}
				if value != 0 {
					if e := m.Write32(at, value-c.AddressBase-0x76c0); e != nil {
						return out, e
					}
				}
			}
			s.PC = 0x4348
		case 0x4348:
			done, e := child(0x19afc, 0x434e)
			if e != nil || !done {
				return out, e
			}
		case 0x434e:
			for _, at := range []int{0xf32, 0xf36} {
				value, e := m.Read32(at)
				if e != nil {
					return out, e
				}
				if value != 0 {
					if e := m.Write32(at, value+c.AddressBase+0x76c0); e != nil {
						return out, e
					}
				}
			}
			if uint16(c.D[0]) == 0 {
				startDialog()
			} else if int16(c.D[0]) < 0 {
				s.PC = 0x4010
			} else {
				s.PC = 0x40ae
			}
		case 0x33b2:
			switch s.dialogPhase {
			case 0:
				c.D[3] = 1
				message, e := code.Read32(0xa93c)
				if e != nil {
					return out, e
				}
				if e := b.compile(caddr(0x91d0).Address, []NativeRequesterAddress{{Address: message, Code: true}}); e != nil {
					return out, e
				}
				if e := copyScreen(); e != nil {
					return out, e
				}
				flag, e := m.Read16(0x3b0)
				if e != nil {
					return out, e
				}
				s.dialogFlag = flag
				s.dialogPhase = 1
			case 1:
				if s.dialogFlag == 0 {
					done, e := palette(0x3361a, 0x33844, 0x33b2)
					if e != nil || !done {
						return out, e
					}
				}
				s.dialogPhase = 2
			case 2:
				if e := draw(); e != nil {
					return out, e
				}
				_, e := b.click()
				if e != nil {
					return out, e
				}
				action := uint16(c.D[0])
				offset, e := code.Read16(0x341a + int(int16(action)))
				if e != nil {
					return out, e
				}
				c.Word(0, offset)
				target := 0x341a + int(int16(offset))
				if target == 0x33ea {
					out.Idle = true
					return out, nil
				}
				if target != 0x33fe {
					return out, fmt.Errorf("native file error dialog dispatch%x unavailable", target)
				}
				s.dialogPhase = 3
			case 3:
				if s.dialogFlag == 0 {
					done, e := palette(0x33844, 0x3361a, 0x33b2)
					if e != nil || !done {
						return out, e
					}
				}
				c.D = s.dialogSaved
				s.PC = 0x4010
			}
		default:
			return out, fmt.Errorf("native file frame PC%x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native file frame exceeded source transition budget")
}
