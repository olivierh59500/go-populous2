package populous2

import "fmt"

// NativeFileTextModal retains the actual $4bba/$4c14 pointer cursor. The
// source's FF sentinel belongs to CODE memory; no typed string limit replaces
// it. Pending $786 waits keep the temporary display bytes and full registers.
type NativeFileTextModal struct {
	Active, Waiting       bool
	Field, Caret, Display int
	Capacity              uint16
	Saved                 [8]uint32
	Tail                  uint8
	Target                uint32
	Finish                uint8
}

func (s *NativeFileTextModal) begin(b nativeRequesterFrameBacking, field, clickEnd int) error {
	c := b.Frame
	marker := clickEnd
	for {
		v, e := b.Code.Read8(marker)
		if e != nil {
			return e
		}
		if v == 'v' {
			break
		}
		marker--
	}
	start, e := b.Code.Read16(0xab4e)
	if e != nil {
		return e
	}
	c.D[0] = uint32(marker - (0xab4e + int(int16(start))))
	c.Word(0, uint16(c.D[0])+1)
	width, e := b.Code.Read16(0xab54)
	if e != nil {
		return e
	}
	c.Word(2, width+1)
	if e := frameDivide(c, 0, uint16(c.D[2])); e != nil {
		return e
	}
	c.Word(2, uint16(c.D[0]))
	c.Swap(0)
	column, e := b.Code.Read16(0xab50)
	if e != nil {
		return e
	}
	row, e := b.Code.Read16(0xab52)
	if e != nil {
		return e
	}
	c.Word(0, uint16(c.D[0])+column)
	c.Word(1, uint16(c.D[0]))
	c.D[2] = uint32(uint16(c.D[2])) * 8
	c.Word(2, uint16(c.D[2])+row)
	c.D[3] = 0xffffffff
	display, end := marker+1, marker+1
	for {
		c.Word(3, uint16(c.D[3])+1)
		v, e := b.Code.Read8(end)
		if e != nil {
			return e
		}
		end++
		if v == 'w' {
			break
		}
	}
	caret := field
	c.D[4] = 0xffffffff
	for {
		c.Word(4, uint16(c.D[4])+1)
		v, e := b.Code.Read8(caret)
		if e != nil {
			return e
		}
		caret++
		if v == 0 {
			break
		}
	}
	caret--
	if int16(c.D[4]) >= int16(c.D[3]) {
		c.Word(4, uint16(c.D[3])-1)
	}
	*s = NativeFileTextModal{Active: true, Field: field, Caret: caret, Display: display, Capacity: uint16(c.D[3])}
	return nil
}

func (s *NativeFileTextModal) prepare(b nativeRequesterFrameBacking) error {
	c := b.Frame
	s.Saved = c.D
	at, out := s.Caret-int(int16(c.D[4])), s.Display
	c.D[6] = 0
	for {
		c.Word(0, 'k')
		v, e := b.Code.Read8(at)
		if e != nil {
			return e
		}
		if v != 0 {
			c.Byte(0, v)
			at++
		}
		if uint16(c.D[6]) == uint16(c.D[5]) {
			phase, e := b.Memory.Read8(0xf)
			if e != nil {
				return e
			}
			if phase&16 != 0 {
				c.Byte(0, 'm')
			}
		}
		if e := b.Code.Write8(out, uint8(c.D[0])); e != nil {
			return e
		}
		out++
		c.Word(6, uint16(c.D[6])+1)
		if uint16(c.D[6]) == uint16(c.D[3]) {
			break
		}
		if uint16(c.D[6]) == 0 {
			return fmt.Errorf("native file modal display exceeded CODE word loop")
		}
	}
	tail, e := b.Code.Read8(out)
	if e != nil {
		return e
	}
	s.Tail = tail
	c.Byte(0, tail)
	if e := b.Code.Write8(out, 0); e != nil {
		return e
	}
	s.Target, e = b.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	c.Word(0, uint16(c.D[1]))
	c.Word(1, uint16(c.D[2]))
	s.Waiting = true
	return nil
}

func fileFrameSwap(b nativeRequesterFrameBacking, presentation *NativeFramePresentationState) error {
	if presentation == nil {
		return fmt.Errorf("native file requester presentation missing")
	}
	if _, e := presentation.Swap(b.Frame); e != nil {
		return e
	}
	if e := b.Code.Write32(0x77a, presentation.CopperSelector); e != nil {
		return e
	}
	return b.Code.Write32(0x77e, presentation.SpritePatchPointer)
}

func (s *NativeFileTextModal) advance(b nativeRequesterFrameBacking, presentation *NativeFramePresentationState, keys NativeInputRules) (bool, error) {
	if s == nil || !s.Active || presentation == nil {
		return false, fmt.Errorf("native file modal live state missing")
	}
	c := b.Frame
	for {
		if s.Waiting {
			ready, e := b.Memory.Read16(0xa)
			if e != nil {
				return false, e
			}
			if ready == 0 {
				return false, nil
			}
			if e := b.text(s.Target, s.Display); e != nil {
				return false, e
			}
			if e := fileFrameSwap(b, presentation); e != nil {
				return false, e
			}
			if e := b.Code.Write8(s.Display+int(s.Capacity), s.Tail); e != nil {
				return false, e
			}
			c.D = s.Saved
			s.Waiting = false
			if s.Finish != 0 {
				if s.Finish == 1 {
					c.D[0] = 0
				}
				s.Active = false
				return true, nil
			}
		}
		left, e := b.Memory.Read16(0x140)
		if e != nil {
			return false, e
		}
		right, e := b.Memory.Read16(0x142)
		if e != nil {
			return false, e
		}
		if left != 0 || right != 0 {
			saved := c.D
			_, e := b.click()
			if e != nil {
				return false, e
			}
			d0 := c.D[0]
			c.D = saved
			c.D[0] = d0
			if uint16(d0) != 0 {
				c.D[5] = 0xffffffff
				s.Finish = 2
				if e := s.prepare(b); e != nil {
					return false, e
				}
				continue
			}
		}
		leftKey, e := b.Memory.Read8(0x93)
		if e != nil {
			return false, e
		}
		rightKey, e := b.Memory.Read8(0x95)
		if e != nil {
			return false, e
		}
		if leftKey != 0 {
			if e := b.Memory.Write8(0x93, 0); e != nil {
				return false, e
			}
			if s.Field != s.Caret {
				s.Caret--
				c.Word(4, uint16(c.D[4])-1)
				if int16(c.D[4]) < 0 {
					c.D[4] = 0
				}
			}
		} else if rightKey != 0 {
			if e := b.Memory.Write8(0x95, 0); e != nil {
				return false, e
			}
			v, e := b.Code.Read8(s.Caret)
			if e != nil {
				return false, e
			}
			if v != 0 {
				s.Caret++
				c.Word(4, uint16(c.D[4])+1)
				if uint16(c.D[4]) == uint16(c.D[3]) {
					c.Word(4, uint16(c.D[4])-1)
				}
			}
		} else {
			key, e := keys.Character(&presentation.Input, &c.D)
			if e != nil {
				return false, e
			}
			switch key {
			case 0:
			case 13:
				c.D[5] = 0xffffffff
				s.Finish = 1
				if e := s.prepare(b); e != nil {
					return false, e
				}
				continue
			case 8:
				if s.Field != s.Caret {
					from, to := s.Caret, s.Caret-1
					s.Caret--
					for {
						v, e := b.Code.Read8(from)
						if e != nil {
							return false, e
						}
						from++
						if e := b.Code.Write8(to, v); e != nil {
							return false, e
						}
						to++
						if v == 0 {
							break
						}
					}
					c.Word(4, uint16(c.D[4])-1)
					if int16(c.D[4]) < 0 {
						c.D[4] = 0
					}
				}
			default:
				end := s.Caret
				for {
					v, e := b.Code.Read8(end)
					if e != nil {
						return false, e
					}
					end++
					if v == 0 {
						break
					}
				}
				v, e := b.Code.Read8(end)
				if e != nil {
					return false, e
				}
				if v != 0xff {
					for from := end - 1; from >= s.Caret; from-- {
						v, e := b.Code.Read8(from)
						if e != nil {
							return false, e
						}
						if e := b.Code.Write8(from+1, v); e != nil {
							return false, e
						}
					}
					if e := b.Code.Write8(s.Caret, key); e != nil {
						return false, e
					}
					s.Caret++
					c.Word(4, uint16(c.D[4])+1)
					if uint16(c.D[4]) == uint16(c.D[3]) {
						c.Word(4, uint16(c.D[4])-1)
					}
				}
			}
		}
		c.Word(5, uint16(c.D[4]))
		if e := s.prepare(b); e != nil {
			return false, e
		}
	}
}
