package populous2

import "fmt"

// NativeRuntimeFileBrowserFrameCallbacks returns actual mutable child A
// registers. Complete and source condition flags remain separate.
type NativeRuntimeFileBrowserFrameCallbacks struct {
	NativeFileFrameCallbacks
	Child func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

// NativeRuntimeFileBrowserFrameState adds the address-register continuation
// to the proven $3f92 data controller. Its helper calls execute once in source
// order; modal, palette and DOS waits retain every live pointer.
type NativeRuntimeFileBrowserFrameState struct {
	NativeFileFrameState
	Modal          NativeRuntimeFileBrowserModal
	dialogA        [7]NativeRequesterAddress
	paletteSavedA0 NativeRequesterAddress
}

func (s *NativeRuntimeFileBrowserFrameState) Advance(r *NativeFileFrameRules, cb NativeRuntimeFileBrowserFrameCallbacks) (out NativeFileFrameStep, failure error) {
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
	b := nativeFileBrowserBacking(cb.NativeFileFrameCallbacks)
	c, code, m := cb.Frame, cb.Code, cb.Memory
	caddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	baddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
	}
	child := func(routine int, next int) (bool, error) {
		if cb.Child == nil {
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
		result, e := cb.Child(NativeStartupResetFrameCall{Routine: routine, A: &s.ChildA, Frame: c}, &s.ChildPhase)
		s.A = s.ChildA
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
		if !s.ChildActive && s.PC == 0x4288 {
			s.paletteSavedA0 = s.A[0]
		}
		s.A[2], s.A[3] = caddr(first), caddr(second)
		preserve := s.PC == 0x4288
		done, e := child(0x102e4, next)
		if done && preserve {
			s.A[0] = s.paletteSavedA0
		}
		return done, e
	}
	copyScreen := func() error { return campaignRequesterCopy(b, &s.A) }
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
		if e := campaignRequesterText(b, &s.A, target, 0xab4e+int(int16(start))); e != nil {
			return e
		}
		return fileFrameSwap(b, cb.Presentation)
	}
	startDialog := func() {
		s.A[1], s.A[2] = caddr(0x91d0), caddr(0xa93c)
		s.dialogSaved, s.dialogA = c.D, s.A
		s.dialogPhase = 0
		s.PC = 0x33b2
	}
	for transitions := 0; transitions < 128; transitions++ {
		if s.Modal.Active {
			done, e := s.Modal.advance(b, cb.Presentation, r.Keys, &s.A)
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
			s.A[1], s.A[2] = caddr(0x7af2), caddr(0x4380)
			if e := nativeFileBrowserCompile(b, &s.A, s.A[1].Address, s.A[2].Address); e != nil {
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
			s.A[1], s.A[2] = list, caddr(0x4384)
			c.Word(0, 11)
			count, e := code.Read16(0x443e)
			if e != nil {
				return out, e
			}
			c.Word(1, count)
			for slot := 1; ; slot++ {
				s.A[2] = caddr(0x4384 + (slot-1)*4)
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
				s.A[1] = list
				s.A[2].Address += 4
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
			s.A[1], s.A[2] = caddr(0x7af2), caddr(0x4380)
			if e := nativeFileBrowserCompile(b, &s.A, s.A[1].Address, s.A[2].Address); e != nil {
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
			end, e := campaignRequesterClick(b, &s.A)
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
				if e := s.Modal.begin(b, field, int(int64(end)-int64(cb.CodeBase)), &s.A); e != nil {
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
			s.A[1], s.A[2] = from, caddr(to)
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
				s.A[1], s.A[2] = from, caddr(to)
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
			s.A[1], s.A[3] = caddr(writeAt), caddr(compare)
			suffix := 0xab49
			s.A[6] = caddr(suffix)
			for {
				v, e := code.Read8(suffix)
				if e != nil {
					return out, e
				}
				suffix++
				s.A[6] = caddr(suffix)
				c.Byte(0, v)
				if v == 0 {
					break
				}
				compare--
				s.A[3] = caddr(compare)
				got, e := code.Read8(compare)
				if e != nil {
					return out, e
				}
				if got != v {
					c.D[0] = 3
					s.A[6] = caddr(0xab4d)
					for source := 0xab4c; source >= 0xab49; source-- {
						v, e := code.Read8(source)
						if e != nil {
							return out, e
						}
						if e := code.Write8(writeAt, v); e != nil {
							return out, e
						}
						writeAt++
						s.A[6], s.A[1] = caddr(source), caddr(writeAt)
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
			s.A[0], s.A[1] = caddr(0x43c0), caddr(to)
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
				_ = message
				if e := nativeFileBrowserCompile(b, &s.A, s.A[1].Address, s.A[2].Address); e != nil {
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
				_, e := campaignRequesterClick(b, &s.A)
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
				c.D, s.A = s.dialogSaved, s.dialogA
				s.PC = 0x4010
			}
		default:
			return out, fmt.Errorf("native file frame PC%x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native file frame exceeded source transition budget")
}
