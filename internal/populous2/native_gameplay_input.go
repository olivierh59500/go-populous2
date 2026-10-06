package populous2

import "fmt"

// NativeGameplayInputState retains the original $110e..$1bd6 continuation.
// It runs after deferred commands, so newly produced commands execute on the
// following frame. Modal children retain the incoming data/address registers.
type NativeGameplayInputState struct {
	Started, Finished bool
	PC                int
	D                 [8]uint32
	A                 [7]NativeRequesterAddress
	ChildPhase        uint32
	ChildRoutine      int
	ChildZero         bool
	failed            error
}

type NativeGameplayInputCallbacks struct {
	NativeStartupResetFrameCallbacks
	Input *NativeInputState
	Keys  NativeInputRules
}

type NativeGameplayInputStep struct {
	Complete, Waiting, ExitRequested bool
	PC                               int
}

func (s *NativeGameplayInputState) Advance(cb NativeGameplayInputCallbacks) (out NativeGameplayInputStep, failure error) {
	if s == nil || cb.Frame == nil || cb.Input == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native gameplay input backing/context missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x110e
		s.D = cb.Frame.D
	}
	cb.Frame.D = s.D
	defer func() {
		s.D = cb.Frame.D
		out.PC = s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		v, e := cb.Memory.Read16(0x3aa)
		out.ExitRequested = v != 0
		return out, e
	}
	c, m := cb.Frame, cb.Memory
	child := func(routine, next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native gameplay input child%x missing", routine)
		}
		if s.ChildRoutine == 0 {
			s.ChildRoutine = routine
		} else if s.ChildRoutine != routine {
			return false, fmt.Errorf("native gameplay input child changed during wait")
		}
		result, err := cb.Call(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
		if err != nil {
			return false, err
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.ChildRoutine, s.ChildPhase, s.PC = 0, 0, next
		s.ChildZero = result.Zero
		return true, nil
	}
	key := func(offset int) (bool, error) { v, e := cb.RAM.Read8(int(s.A[0].Address) + offset); return v != 0, e }
	clearKey := func(offset int) error { return cb.RAM.Write8(int(s.A[0].Address)+offset, 0) }
	clearLast := func() error { return m.Write8(0x2a, 0) }
	command := func(value uint8) error {
		p, e := m.Read32(0xeb6a)
		if e != nil {
			return e
		}
		s.A[0] = NativeRequesterAddress{Address: p}
		return cb.RAM.Write8(int(p)+1, value)
	}
	word := func(at int) (uint16, error) { return m.Read16(at) }
	modify := func(at int, delta int16) error {
		v, e := word(at)
		if e != nil {
			return e
		}
		return m.Write16(at, v+uint16(delta))
	}
	for transitions := 0; transitions < 256; transitions++ {
		switch s.PC {
		case 0x110e:
			v, e := word(0x132)
			if e != nil {
				return out, e
			}
			if v == 0 {
				s.PC = 0x1560
			} else {
				s.A[0] = NativeRequesterAddress{Address: c.AddressBase + 0x32}
				s.PC = 0x111e
			}
		case 0x111e, 0x1140:
			offset, next, routine := 0x53, 0x1140, 0xd97e
			if s.PC == 0x1140 {
				offset, next, routine = 0x51, 0x116e, 0xd962
			}
			pressed, e := key(offset)
			if e != nil {
				return out, e
			}
			if !pressed {
				s.PC = next
				continue
			}
			if e = clearKey(offset); e != nil {
				return out, e
			}
			edit, e := word(0xf0e)
			if e != nil {
				return out, e
			}
			if edit == 0 {
				if e = clearLast(); e != nil {
					return out, e
				}
				s.PC = 0x1560
			} else if routine == 0xd97e {
				s.PC = 0x1130
			} else {
				s.PC = 0x1152
			}
		case 0x1130:
			done, e := child(0xd97e, 0x1136)
			if e != nil || !done {
				return out, e
			}
		case 0x1152:
			done, e := child(0xd962, 0x1158)
			if e != nil || !done {
				return out, e
			}
		case 0x1158:
			p, e := m.Read32(0x22)
			if e != nil {
				return out, e
			}
			s.A[0] = NativeRequesterAddress{Address: p}
			s.PC = 0x115e
		case 0x115e:
			done, e := child(0xd8cc, 0x1136)
			if e != nil || !done {
				return out, e
			}
		case 0x1136:
			if e := clearLast(); e != nil {
				return out, e
			}
			s.PC = 0x1560
		case 0x116e:
			pressed, e := key(0x53)
			if e != nil {
				return out, e
			}
			if !pressed {
				s.PC = 0x1188
				continue
			}
			if e = clearKey(0x53); e != nil {
				return out, e
			}
			s.PC = 0x1178
		case 0x1178:
			done, e := child(0x1a55a, 0x1136)
			if e != nil || !done {
				return out, e
			}
		case 0x1188:
			pressed, e := key(0x4d)
			if e != nil {
				return out, e
			}
			if !pressed {
				s.PC = 0x11d2
				continue
			}
			x, e := word(0x138)
			if e != nil {
				return out, e
			}
			y, e := word(0x13a)
			if e != nil {
				return out, e
			}
			if x == 0 && y == 0 {
				if e = m.Write16(0x3aa, 1); e != nil {
					return out, e
				}
				s.PC = 0x1560
				continue
			}
			edit, e := word(0xf0e)
			if e != nil {
				return out, e
			}
			if edit != 0 {
				v, e := word(0xf3e)
				if e != nil {
					return out, e
				}
				if e = m.Write16(0xf3e, ^v); e != nil {
					return out, e
				}
			} else {
				if e = command(104); e != nil {
					return out, e
				}
				if e = clearLast(); e != nil {
					return out, e
				}
			}
			s.PC = 0x1560
		case 0x11d2, 0x11fa, 0x1222, 0x124a, 0x1272, 0x129a:
			index := (s.PC - 0x11d2) / 0x28
			offset := 0x5f - index*2
			pressed, e := key(offset)
			if e != nil {
				return out, e
			}
			if !pressed {
				if index == 5 {
					s.PC = 0x12c2
				} else {
					s.PC += 0x28
				}
				continue
			}
			c.Word(0, 0x1cc)
			s.PC = []int{0x11dc, 0x1204, 0x122c, 0x1254, 0x127c, 0x12a4}[index]
		case 0x11dc, 0x1204, 0x122c, 0x1254, 0x127c, 0x12a4:
			index := (s.PC - 0x11dc) / 0x28
			done, e := child(0x184f6, []int{0x11e2, 0x120a, 0x1232, 0x125a, 0x1282, 0x12aa}[index])
			if e != nil || !done {
				return out, e
			}
		case 0x11e2, 0x120a, 0x1232, 0x125a, 0x1282, 0x12aa:
			index := (s.PC - 0x11e2) / 0x28
			if e := m.Write16(0xf3a, uint16(index*2)); e != nil {
				return out, e
			}
			s.PC = 0x11ea
		case 0x11ea:
			done, e := child(0x1da0, 0x1136)
			if e != nil || !done {
				return out, e
			}
		case 0x12c2, 0x12dc, 0x12f6, 0x1310, 0x132a:
			index := (s.PC - 0x12c2) / 0x1a
			pressed, e := key(0xfd - index*2)
			if e != nil {
				return out, e
			}
			if !pressed {
				if index == 4 {
					s.PC = 0x1344
				} else {
					s.PC += 0x1a
				}
				continue
			}
			c.Word(2, uint16(index))
			s.PC = 0x12cc
		case 0x12cc:
			done, e := child(0x2472, 0x1136)
			if e != nil || !done {
				return out, e
			}
		case 0x1344, 0x136a, 0x1390, 0x13b6, 0x13dc:
			index := (s.PC - 0x1344) / 0x26
			pressed, e := key([]int{0x4b, 0x49, 0x47, 0x45, 0x8b}[index])
			if e != nil {
				return out, e
			}
			if !pressed {
				if index == 4 {
					s.PC = 0x13fe
				} else {
					s.PC += 0x26
				}
				continue
			}
			c.Word(0, 0x1cc)
			s.PC = 0x134e + index*0x26
		case 0x134e, 0x1374, 0x139a, 0x13c0, 0x13e6:
			index := (s.PC - 0x134e) / 0x26
			done, e := child(0x184f6, 0x1354+index*0x26)
			if e != nil || !done {
				return out, e
			}
		case 0x1354, 0x137a, 0x13a0, 0x13c6, 0x13ec:
			index := (s.PC - 0x1354) / 0x26
			if index == 4 {
				if e := m.Write16(0xeb18, 12); e != nil {
					return out, e
				}
			} else if e := command([]uint8{14, 18, 20, 16}[index]); e != nil {
				return out, e
			}
			if e := clearLast(); e != nil {
				return out, e
			}
			s.PC = 0x1560
		case 0x13fe, 0x1428:
			increase := s.PC == 0x13fe
			offset, next := 0x43, 0x1428
			if !increase {
				offset, next = 0x6b, 0x144a
			}
			pressed, e := key(offset)
			if e != nil {
				return out, e
			}
			if !pressed {
				s.PC = next
				continue
			}
			v, e := word(0xf0c)
			if e != nil {
				return out, e
			}
			if increase {
				v++
				if int16(v) > 23 {
					v = 23
					if e = clearLast(); e != nil {
						return out, e
					}
				}
			} else {
				v--
				if int16(v) <= 0 {
					v = 1
					if e = clearLast(); e != nil {
						return out, e
					}
				}
			}
			if e = m.Write16(0xf0c, v); e != nil {
				return out, e
			}
			s.PC = 0x1560
		case 0x144a:
			pressed, e := key(0x79)
			if e != nil {
				return out, e
			}
			if !pressed {
				s.PC = 0x1482
				continue
			}
			if e = clearKey(0x79); e != nil {
				return out, e
			}
			v, e := word(0xf0c)
			if e != nil {
				return out, e
			}
			if v == 8 {
				v = 23
			} else {
				v = 8
			}
			if e = m.Write16(0xf0c, v); e != nil {
				return out, e
			}
			if e = clearLast(); e != nil {
				return out, e
			}
			s.PC = 0x1560
		case 0x1482:
			groups := []struct {
				offsets []int
				dx, dy  int16
			}{{[]int{0x83, 0x67}, 0, -1}, {[]int{0xc3, 0x65}, 0, 1}, {[]int{0xa5, 0x61}, -1, 0}, {[]int{0xa1, 0x63}, 1, 0}, {[]int{0x85}, -1, -1}, {[]int{0x81}, 1, -1}, {[]int{0xc5}, -1, 1}, {[]int{0xc1}, 1, 1}}
			for _, g := range groups {
				pressed := false
				for _, offset := range g.offsets {
					p, e := key(offset)
					if e != nil {
						return out, e
					}
					pressed = pressed || p
				}
				if pressed {
					if g.dx != 0 {
						if e := modify(0x5f44, g.dx); e != nil {
							return out, e
						}
					}
					if g.dy != 0 {
						if e := modify(0x5f46, g.dy); e != nil {
							return out, e
						}
					}
					if e := clearLast(); e != nil {
						return out, e
					}
				}
			}
			s.PC = 0x1548
		case 0x1548:
			value, e := cb.Keys.Character(cb.Input, &c.D)
			if e != nil {
				return out, e
			}
			if value != 0 {
				if e = command(114); e != nil {
					return out, e
				}
				if e = cb.RAM.Write8(int(s.A[0].Address)+2, value); e != nil {
					return out, e
				}
			}
			s.PC = 0x1560
		case 0x1bd0:
			v, e := word(0x3aa)
			if e != nil {
				return out, e
			}
			s.Finished = true
			out.Complete = true
			out.ExitRequested = v != 0
			s.PC = 0
			return out, nil
		default:
			return s.advanceMouse(cb, out, child)
		}
	}
	return out, fmt.Errorf("native gameplay input transitions exceeded")
}
