package populous2

import "fmt"

// NativeInGameFrameState is the actual $446a requester controller. Its
// outer MOVEM restores all caller data registers only after the real resume
// gate, panel and audio children finish. Idle polling is not completion.
type NativeInGameFrameState struct {
	Started, Finished             bool
	PC                            int
	Registers, Saved, DialogSaved [8]uint32
	ChildActive                   bool
	ChildRoutine                  int
	ChildPhase                    uint32
	LastZero                      bool
	DialogPhase                   uint8
	DialogFlag                    uint16
	failed                        error
}

func (s *NativeInGameFrameState) Advance(cb NativeFileFrameCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || cb.Frame == nil || cb.Presentation == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || cb.Bitmap == nil {
		return out, fmt.Errorf("native in-game frame backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x446a
		s.Registers = cb.Frame.D
		s.Saved = cb.Frame.D
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
	c, code, m := cb.Frame, cb.Code, cb.Memory
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	caddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	child := func(routine, next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native in-game child%x missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
			out.Calls = append(out.Calls, routine)
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native in-game child changed across suspension")
		}
		call := NativeFileFrameCall{Routine: routine, Frame: c}
		if routine == 0x102e4 {
			first, second := 0x3361a, 0x33844
			if s.DialogPhase == 3 {
				first, second = second, first
			}
			call.Arguments = 1<<2 | 1<<3
			call.A[2], call.A[3] = caddr(first), caddr(second)
		}
		result, e := cb.Call(call, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.LastZero = result.Zero
		s.ChildActive = false
		s.ChildPhase = 0
		s.PC = next
		return true, nil
	}
	copyScreen := func() error {
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
			return fmt.Errorf("native in-game screen RAM missing")
		}
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
	queue := func(command uint8) error {
		pointer, e := m.Read32(0xeb6a)
		if e != nil {
			return e
		}
		return m.Write8(int(int64(pointer)-int64(c.AddressBase))+1, command)
	}
	for transitions := 0; transitions < 128; transitions++ {
		switch s.PC {
		case 0x446a:
			done, e := child(0x1842e, 0x4474)
			if e != nil || !done {
				return out, e
			}
		case 0x4474:
			cursor, e := code.Read16(0xa2a)
			if e != nil {
				return out, e
			}
			if e := m.Write16(0xddc, cursor); e != nil {
				return out, e
			}
			if e := code.Write16(0xa2a, 0); e != nil {
				return out, e
			}
			s.PC = 0x4486
		case 0x4486:
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(0, profile)
			first := 0x9670
			if uint16(c.D[0]) != 1 {
				first = 0x9675
			}
			if e := code.Write32(0x4710, caddr(first).Address); e != nil {
				return out, e
			}
			c.D[0] = uint32(uint16(c.D[0])) * 314
			god := 0xe76a + int(int16(c.D[0]))
			mode, e := m.Read16(god + 0x1a)
			if e != nil {
				return out, e
			}
			second, third := 0x967e, 0x9682
			if mode == 18 {
				second = 0x967a
			} else if mode != 2 {
				third = 0x96aa
				if mode == 4 {
					third = 0x9696
				}
			}
			if e := code.Write32(0x4714, caddr(second).Address); e != nil {
				return out, e
			}
			if e := code.Write32(0x4718, caddr(third).Address); e != nil {
				return out, e
			}
			c.D[3] = 1
			if e := b.compile(caddr(0x88e4).Address, []NativeRequesterAddress{caddr(first), caddr(second), caddr(third)}); e != nil {
				return out, e
			}
			if e := copyScreen(); e != nil {
				return out, e
			}
			start, e := code.Read16(0xab4e)
			if e != nil {
				return out, e
			}
			at := 0xab4e + int(int16(start))
			c.D[1], c.D[2] = 0, 0
			for {
				v, e := code.Read8(at)
				if e != nil {
					return out, e
				}
				at++
				c.Byte(0, v)
				if v == 0 {
					break
				}
				switch v {
				case 'y':
					game, e := m.Read16(0xeb44)
					if e != nil {
						return out, e
					}
					expected := uint16(4)
					if uint16(c.D[2]) == 0 {
						c.Word(2, 1)
						expected = 2
					}
					if game != expected {
						if e := code.Write8(at-1, 'z'); e != nil {
							return out, e
						}
					}
				case 'c':
					if uint16(c.D[1]) == 0 {
						c.D[1] = 1
						game, e := m.Read16(0xeb44)
						if e != nil {
							return out, e
						}
						if game == 6 {
							if e := code.Write8(at-1, 'd'); e != nil {
								return out, e
							}
						}
					} else if uint16(c.D[1]) == 1 {
						c.D[1] = 2
						paint, e := m.Read16(0xf0e)
						if e != nil {
							return out, e
						}
						if paint == 0 {
							if e := code.Write8(at-1, 'd'); e != nil {
								return out, e
							}
						}
					}
				}
			}
			s.PC = 0x45a4
		case 0x45a4:
			if e := draw(); e != nil {
				return out, e
			}
			s.PC = 0x45aa
		case 0x45aa:
			if _, e := b.click(); e != nil {
				return out, e
			}
			action := uint16(c.D[0])
			offset, e := code.Read16(0x45e4 + int(int16(action)))
			if e != nil {
				return out, e
			}
			c.Word(0, offset)
			s.PC = 0x45e4 + int(int16(offset))
			if s.PC == 0x45a4 {
				out.Idle = true
				return out, nil
			}
		case 0x45b8:
			done, e := child(0x181c0, 0x45be)
			if e != nil || !done {
				return out, e
			}
		case 0x45be:
			if !s.LastZero {
				s.PC = 0x4486
			} else {
				s.PC = 0x45c2
			}
		case 0x45c2:
			done, e := child(0x1da0, 0x45c8)
			if e != nil || !done {
				return out, e
			}
		case 0x45c8:
			cursor, e := m.Read16(0xddc)
			if e != nil {
				return out, e
			}
			if e := code.Write16(0xa2a, cursor); e != nil {
				return out, e
			}
			if e := m.Write32(0x140, 0); e != nil {
				return out, e
			}
			s.PC = 0x45d8
		case 0x45d8:
			done, e := child(0x18474, 0x45de)
			if e != nil || !done {
				return out, e
			}
		case 0x45de:
			c.D = s.Saved
			s.Finished = true
			out.Complete = true
			return out, nil
		case 0x45fe, 0x460e, 0x4650, 0x46a0, 0x4700:
			command := uint8(110)
			switch s.PC {
			case 0x460e:
				command = 108
			case 0x4650:
				command = 124
			case 0x46a0:
				command = 116
			case 0x4700:
				command = 112
			}
			if e := queue(command); e != nil {
				return out, e
			}
			s.PC = 0x45b8
		case 0x461e:
			game, e := m.Read16(0xeb44)
			if e != nil {
				return out, e
			}
			if game == 2 {
				s.PC = 0x4486
			} else if game == 6 {
				s.PC = 0x4650
			} else {
				c.Word(0, 2)
				profile, e := m.Read16(0xeb42)
				if e != nil {
					return out, e
				}
				if profile != 1 {
					c.Word(0, 1)
				}
				s.PC = 0x4646
			}
		case 0x4646:
			done, e := child(0x111ae, 0x4486)
			if e != nil || !done {
				return out, e
			}
		case 0x4660, 0x4678:
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(0, profile)
			c.D[0] = uint32(uint16(c.D[0])) * 314
			at := 0xe76a + int(int16(c.D[0])) + 0x1a
			value := uint16(18)
			if s.PC == 0x4678 {
				value = 4
			}
			c.Word(0, value)
			old, e := m.Read16(at)
			if e != nil {
				return out, e
			}
			if old == value {
				c.Word(0, 2)
			}
			if e := m.Write16(at, uint16(c.D[0])); e != nil {
				return out, e
			}
			s.PC = 0x4486
		case 0x46b0:
			game, e := m.Read16(0xeb44)
			if e != nil {
				return out, e
			}
			if game == 2 || game == 6 {
				s.PC = 0x45b8
			} else {
				s.PC = 0x46c4
			}
		case 0x46c4:
			done, e := child(0x4984, 0x45b8)
			if e != nil || !done {
				return out, e
			}
		case 0x46ce:
			game, e := m.Read16(0xeb44)
			if e != nil {
				return out, e
			}
			if game != 2 {
				paint, e := m.Read16(0xf0e)
				if e != nil {
					return out, e
				}
				if e := m.Write16(0xf0e, ^paint); e != nil {
					return out, e
				}
			}
			s.PC = 0x4486
		case 0x46e4:
			done, e := child(0x471c, 0x45b8)
			if e != nil || !done {
				return out, e
			}
		case 0x46ee:
			s.DialogSaved = c.D
			s.DialogPhase = 0
			s.PC = 0x33b2
		case 0x33b2:
			switch s.DialogPhase {
			case 0:
				c.D[3] = 1
				if e := b.compile(caddr(0x84a2).Address, nil); e != nil {
					return out, e
				}
				if e := copyScreen(); e != nil {
					return out, e
				}
				flag, e := m.Read16(0x3b0)
				if e != nil {
					return out, e
				}
				s.DialogFlag = flag
				s.DialogPhase = 1
			case 1:
				if s.DialogFlag == 0 {
					done, e := child(0x102e4, 0x33b2)
					if e != nil || !done {
						return out, e
					}
				}
				s.DialogPhase = 2
			case 2:
				if e := draw(); e != nil {
					return out, e
				}
				if _, e := b.click(); e != nil {
					return out, e
				}
				offset, e := code.Read16(0x341a + int(int16(c.D[0])))
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
					return out, fmt.Errorf("native About dispatch%x unavailable", target)
				}
				s.DialogPhase = 3
			case 3:
				if s.DialogFlag == 0 {
					done, e := child(0x102e4, 0x33b2)
					if e != nil || !done {
						return out, e
					}
				}
				c.D = s.DialogSaved
				s.PC = 0x4486
			}
		default:
			return out, fmt.Errorf("native in-game PC%x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native in-game transition budget exceeded")
}
