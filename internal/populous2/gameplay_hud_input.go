package populous2

import "fmt"

// NativeGameplayHUDInputState is full$23ee/$2472. Raw power-byte toggles,
// costs, modifier admission and delayed command records remain source bytes;
// genuine panel/help waits keep the complete caller D/A continuation.
type NativeGameplayHUDInputState struct {
	Entry, PC         int
	Started, Finished bool
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildRoutine      int
	ChildPhase        uint32
	SavedWord         uint16
	ReturnValue       uint32
	failed            error
}

func (s *NativeGameplayHUDInputState) Advance(r *NativeGameplayHUDInputRules, cb NativeGameplayHUDInputCallbacks) (out NativeGameplayHUDInputStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native HUD input backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		if s.Entry == 0 {
			s.Entry = 0x23ee
		}
		if s.Entry != 0x23ee && s.Entry != 0x2472 {
			return out, fmt.Errorf("native HUD entry%x unsupported", s.Entry)
		}
		s.Started = true
		s.PC = s.Entry
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
		out.Complete, out.FlagsKnown = true, true
		out.Zero = s.ReturnValue == 0
		return out, nil
	}
	c, m := cb.Frame, cb.Memory
	child := func(routine, next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native HUD actual child%x missing", routine)
		}
		if s.ChildRoutine == 0 {
			s.ChildRoutine = routine
		} else if s.ChildRoutine != routine {
			return false, fmt.Errorf("native HUD child changed duringwait")
		}
		result, e := cb.Call(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.ChildRoutine, s.ChildPhase, s.PC = 0, 0, next
		return true, nil
	}
	command := func(value byte) error {
		pointer, e := m.Read32(0xeb6a)
		if e != nil {
			return e
		}
		s.A[0] = NativeRequesterAddress{Address: pointer}
		return cb.RAM.Write8(int(pointer)+1, value)
	}
	for transitions := 0; transitions < 64; transitions++ {
		switch s.PC {
		case 0x23ee, 0x2448:
			x, y := uint16(59), uint16(137)
			if s.PC == 0x2448 {
				x, y = 42, 151
			}
			c.Word(0, uint16(c.D[6]))
			c.Word(1, uint16(c.D[7]))
			c.Word(0, uint16(c.D[0])-x)
			c.Word(1, uint16(c.D[1])-y)
			c.Word(0, uint16(int16(c.D[0])>>1))
			c.Word(3, uint16(c.D[1])-uint16(c.D[0]))
			c.Word(2, uint16(c.D[0])+uint16(c.D[1]))
			c.Word(3, uint16(int16(c.D[3])>>4))
			valid := uint16(c.D[3]) == 0
			if valid {
				c.Word(2, uint16(int16(c.D[2])>>4))
				valid = int16(c.D[2]) >= 0 && int16(c.D[2]) <= 5
			}
			if !valid {
				if s.PC == 0x23ee {
					s.PC = 0x2448
				} else {
					s.PC = 0x2850
				}
				continue
			}
			if s.PC == 0x23ee {
				c.Word(2, uint16(c.D[2])*2)
				branch, e := cb.Code.Read16(0x2422 + int(int16(c.D[2])))
				if e != nil {
					return out, e
				}
				c.Word(0, branch)
				s.PC = 0x2422 + int(int16(branch))
			} else {
				s.PC = 0x2472
			}
		case 0x242e:
			c.Word(0, 0x1cc)
			s.PC = 0x2432
		case 0x2432:
			done, e := child(0x184f6, 0x2438)
			if e != nil || !done {
				return out, e
			}
		case 0x2438:
			if e := m.Write16(0xf3a, uint16(c.D[2])); e != nil {
				return out, e
			}
			s.PC = 0x243e
		case 0x243e:
			done, e := child(0x1da0, 0x284c)
			if e != nil || !done {
				return out, e
			}
		case 0x2472:
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(3, profile)
			c.D[3] = uint32(uint16(c.D[3])) * 314
			s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe76a) + int64(int16(c.D[3])))}
			group, e := m.Read16(0xf3a)
			if e != nil {
				return out, e
			}
			c.Word(0, group)
			c.D[0] = uint32(uint16(c.D[0])) * 3
			c.Word(4, uint16(c.D[0])+uint16(c.D[2]))
			modifier, e := m.Read8(0xad)
			if e != nil {
				return out, e
			}
			left, e := m.Read16(0x140)
			if e != nil {
				return out, e
			}
			if modifier != 0 && left != 0 {
				at := int(s.A[1].Address) + 0x70 + int(int16(c.D[4]))
				value, e := cb.RAM.Read8(at)
				if e != nil {
					return out, e
				}
				if e = cb.RAM.Write8(at, -value); e != nil {
					return out, e
				}
				s.PC = 0x24a6
			} else {
				s.PC = 0x24b6
			}
		case 0x24a6:
			done, e := child(0x1da0, 0x24ac)
			if e != nil || !done {
				return out, e
			}
		case 0x24ac:
			if e := CompileNativeStartupChoices(cb.NativeStartupResetFrameCallbacks, &s.A); e != nil {
				return out, e
			}
			s.PC = 0x284c
		case 0x24b6:
			flag, e := cb.RAM.Read8(int(s.A[1].Address) + 0x70 + int(int16(c.D[4])))
			if e != nil {
				return out, e
			}
			if int8(flag) <= 0 {
				s.PC = 0x284c
				continue
			}
			c.Word(0, uint16(c.D[0])*2+uint16(c.D[2])*2)
			help, e := m.Read8(0x73)
			if e != nil {
				return out, e
			}
			if help != 0 {
				c.Word(1, uint16(c.D[0]))
				s.PC = 0x24ce
			} else {
				s.PC = 0x24d8
			}
		case 0x24ce:
			done, e := child(0x517a, 0x284c)
			if e != nil || !done {
				return out, e
			}
		case 0x24d8:
			right, e := m.Read16(0x142)
			if e != nil {
				return out, e
			}
			if right != 0 {
				s.PC = 0x2512
				continue
			}
			free, e := m.Read16(0xf0e)
			if e != nil {
				return out, e
			}
			if free != 0 {
				if e = m.Write16(0xf10, 0); e != nil {
					return out, e
				}
				s.PC = 0x2512
				continue
			}
			s.SavedWord = uint16(c.D[0])
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(1, profile)
			c.Word(0, uint16(c.D[0])>>1)
			if e = NativeGameplayHUDCost(r, cb, &s.A); e != nil {
				return out, e
			}
			c.D[3] = c.D[0] * 4
			c.Word(0, s.SavedWord)
			c.Word(2, uint16(c.D[0]))
			mana, e := cb.RAM.Read32(int(s.A[1].Address))
			if e != nil {
				return out, e
			}
			if c.D[3] > mana {
				s.PC = 0x284c
			} else {
				s.PC = 0x2512
			}
		case 0x2512:
			s.SavedWord = uint16(c.D[0])
			c.Word(0, 0x1cc)
			s.PC = 0x2518
		case 0x2518:
			done, e := child(0x184f6, 0x251e)
			if e != nil || !done {
				return out, e
			}
		case 0x251e:
			c.Word(0, s.SavedWord)
			branch, e := cb.Code.Read16(0x2528 + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			c.Word(3, branch)
			s.PC = 0x2528 + int(int16(c.D[3]))
		case 0x2570:
			if e := m.Write16(0xeb18, 2); e != nil {
				return out, e
			}
			s.PC = 0x284c
		case 0x257c:
			right, e := m.Read16(0x142)
			if e != nil {
				return out, e
			}
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			if right != 0 {
				c.Word(1, profile)
				c.D[1] = uint32(uint16(c.D[1])) * 314
				s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe76a) + int64(int16(c.D[1])))}
				leader, e := cb.RAM.Read16(int(s.A[1].Address) + 10)
				if e != nil {
					return out, e
				}
				c.Word(1, leader)
				s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0x76c0) + int64(int16(c.D[1])))}
				c.D[0] = 0
				x, e := cb.RAM.Read8(int(s.A[1].Address) + 6)
				if e != nil {
					return out, e
				}
				c.Byte(0, x)
				c.Word(0, uint16(c.D[0])-4)
				if e = m.Write16(0x5f44, uint16(c.D[0])); e != nil {
					return out, e
				}
				y, e := cb.RAM.Read8(int(s.A[1].Address) + 8)
				if e != nil {
					return out, e
				}
				c.Byte(0, y)
				c.Word(0, uint16(c.D[0])-4)
				if e = m.Write16(0x5f46, uint16(c.D[0])); e != nil {
					return out, e
				}
				if e = ClampNativeFrameCamera(m, c); e != nil {
					return out, e
				}
			} else {
				c.Word(2, profile)
				c.D[2] = uint32(uint16(c.D[2])) * 314
				s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe76a) + int64(int16(c.D[2])))}
				population, e := cb.RAM.Read16(int(s.A[1].Address) + 8)
				if e != nil {
					return out, e
				}
				if population != 0 {
					if e = m.Write16(0xeb18, 8); e != nil {
						return out, e
					}
				}
			}
			s.PC = 0x284c
		case 0x25ec, 0x2694, 0x26f2, 0x274e, 0x27b0, 0x281e:
			step, e := RunNativeGameplayHUDScan(0x2940, cb, &s.A)
			if e != nil {
				return out, e
			}
			if !step.Zero {
				s.PC = 0x284c
				continue
			}
			value := map[int]byte{0x25ec: 0x24, 0x2694: 0x3a, 0x26f2: 0x3c, 0x274e: 0x42, 0x27b0: 0x44, 0x281e: 0x46}[s.PC]
			if e = command(value); e != nil {
				return out, e
			}
			s.PC = 0x284c
		case 0x267e, 0x26c6, 0x270c, 0x2722, 0x2738, 0x2784, 0x279a, 0x27ca, 0x27e0, 0x27f6:
			step, e := RunNativeGameplayHUDScan(0x2854, cb, &s.A)
			if e != nil {
				return out, e
			}
			if !step.Zero {
				s.PC = 0x284c
				continue
			}
			value := map[int]uint16{0x267e: 0x1a, 0x26c6: 0x28, 0x270c: 0x1c, 0x2722: 0x16, 0x2738: 0x40, 0x2784: 6, 0x279a: 0x26, 0x27ca: 0x3e, 0x27e0: 0x4a, 0x27f6: 0x18}[s.PC]
			if e = m.Write16(0xeb18, value); e != nil {
				return out, e
			}
			s.PC = 0x284c
		case 0x2606, 0x263c, 0x2652, 0x2668, 0x26dc, 0x280a, 0x2838:
			right, e := m.Read16(0x142)
			if e != nil {
				return out, e
			}
			if right == 0 {
				value := map[int]uint16{0x2606: 0x4e, 0x263c: 0x2e, 0x2652: 0x50, 0x2668: 0x36, 0x26dc: 0x30, 0x280a: 0x34, 0x2838: 0x38}[s.PC]
				if s.PC == 0x2606 {
					pointer, e := m.Read32(0xeb6a)
					if e != nil {
						return out, e
					}
					s.A[0] = NativeRequesterAddress{Address: pointer}
				}
				if e = m.Write16(0xeb18, value); e != nil {
					return out, e
				}
			}
			s.PC = 0x284c
		case 0x2622:
			right, e := m.Read16(0x142)
			if e != nil {
				return out, e
			}
			if right == 0 {
				if e = command(0x48); e != nil {
					return out, e
				}
			}
			s.PC = 0x284c
		case 0x2768:
			right, e := m.Read16(0x142)
			if e != nil {
				return out, e
			}
			if right == 0 {
				pointer, e := m.Read32(0xeb6a)
				if e != nil {
					return out, e
				}
				s.A[0] = NativeRequesterAddress{Address: pointer}
				if e = m.Write16(0xeb18, 0x4c); e != nil {
					return out, e
				}
			}
			s.PC = 0x284c
		case 0x26ae:
			if e := m.Write16(0xeb18, 0x2a); e != nil {
				return out, e
			}
			s.PC = 0x284c
		case 0x26ba:
			if e := m.Write16(0xeb18, 0x22); e != nil {
				return out, e
			}
			s.PC = 0x284c
		case 0x284c, 0x2850:
			s.ReturnValue = 1
			if s.PC == 0x2850 {
				s.ReturnValue = 0
			}
			c.D[0] = s.ReturnValue
			s.Finished = true
			out.Complete, out.FlagsKnown = true, true
			out.Zero = s.ReturnValue == 0
			return out, nil
		default:
			return out, fmt.Errorf("native HUD sourcePC%x unsupported", s.PC)
		}
	}
	out.Waiting = true
	return out, nil
}
