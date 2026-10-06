package populous2

import "fmt"

// RunNativeProgressionAnimation executes $106f8 against the actual shared
// screen, Copper, resource and mutable CODE owners. D0/A4/A5 are caller state;
// no typed animation cursor, row clamp or independent image buffer replaces it.
func RunNativeProgressionAnimation(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return fmt.Errorf("native progression animation backing missing")
	}
	s := nativeProgressionAnimation{cb: cb, c: cb.Frame, a: a}
	switch s.c.D[0] {
	case nativeAnimationInit:
		value, e := s.long(4)
		if e != nil {
			return e
		}
		a[5] = NativeRequesterAddress{Address: value, Absolute: true}
		if e = s.initial(); e != nil {
			return e
		}
		s.c.D[0] = nativeAnimationFrame
	case nativeAnimationFrame:
		if e := s.copyScreens(); e != nil {
			return e
		}
		if e := s.delta(); e != nil {
			return e
		}
		s.c.D[0] = a[5].Address
		if uint16(s.c.D[0]) != 0 {
			a[5] = NativeRequesterAddress{Absolute: true}
			s.c.Word(0, 0)
			s.c.Swap(0)
		} else {
			a[5] = a[4]
			s.c.D[1] = s.c.D[0]
			s.c.Swap(1)
			s.c.Word(0, uint16(s.c.D[1]))
		}
	case nativeAnimationStop:
	default:
		saved := s.c.D[0]
		if e := s.delta(); e != nil {
			return e
		}
		s.c.D[0] = saved
		s.c.Word(0, uint16(s.c.D[0])-1)
		if uint16(s.c.D[0]) == 1 {
			s.c.Swap(0)
			if uint16(s.c.D[0]) == 0 {
				s.c.D[0] = nativeAnimationStop
				if e := s.copyScreens(); e != nil {
					return e
				}
			} else {
				s.c.Word(1, uint16(s.c.D[0]))
				s.c.Swap(0)
				s.c.Word(0, uint16(s.c.D[1]))
				a[4] = a[5]
			}
		}
	}
	return nil
}

type nativeProgressionAnimation struct {
	cb   NativeStartupResetFrameCallbacks
	c    *NativeFrameRegisterContext
	a    *[7]NativeRequesterAddress
	work int
}

func (s *nativeProgressionAnimation) limit() error {
	s.work++
	if s.work > 4000000 {
		return fmt.Errorf("native animation exceeded bounded source work")
	}
	return nil
}
func (s *nativeProgressionAnimation) byte(reg int) (byte, error) {
	if e := s.limit(); e != nil {
		return 0, e
	}
	v, e := s.cb.RAM.Read8(int(s.a[reg].Address))
	if e == nil {
		s.a[reg].Address++
	}
	return v, e
}
func (s *nativeProgressionAnimation) word(reg int) (uint16, error) {
	if e := s.limit(); e != nil {
		return 0, e
	}
	v, e := s.cb.RAM.Read16(int(s.a[reg].Address))
	if e == nil {
		s.a[reg].Address += 2
	}
	return v, e
}
func (s *nativeProgressionAnimation) long(reg int) (uint32, error) {
	if e := s.limit(); e != nil {
		return 0, e
	}
	v, e := s.cb.RAM.Read32(int(s.a[reg].Address))
	if e == nil {
		s.a[reg].Address += 4
	}
	return v, e
}
func (s *nativeProgressionAnimation) writeByte(reg int, value byte, advance bool) error {
	if e := s.limit(); e != nil {
		return e
	}
	if e := s.cb.RAM.Write8(int(s.a[reg].Address), value); e != nil {
		return e
	}
	if advance {
		s.a[reg].Address++
	}
	return nil
}
func (s *nativeProgressionAnimation) dbf(reg int) bool {
	s.c.Word(reg, uint16(s.c.D[reg])-1)
	return uint16(s.c.D[reg]) != 0xffff
}
func (s *nativeProgressionAnimation) copyScreens() error {
	s.c.Word(1, 0x1f3f)
	front, e := s.cb.Memory.Read32(0x1a)
	if e != nil {
		return e
	}
	back, e := s.cb.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	s.a[0], s.a[1] = NativeRequesterAddress{Address: front, Chip: true}, NativeRequesterAddress{Address: back, Chip: true}
	for {
		v, e := s.long(0)
		if e != nil {
			return e
		}
		if e = s.cb.RAM.Write32(int(s.a[1].Address), v); e != nil {
			return e
		}
		s.a[1].Address += 4
		if !s.dbf(1) {
			break
		}
	}
	return nil
}
func (s *nativeProgressionAnimation) palette() error {
	for i, operand := range []int{0x107d2, 0x107d8} {
		p, e := s.cb.Code.Read32(operand)
		if e != nil {
			return e
		}
		value, e := s.cb.RAM.Read32(int(p))
		if e != nil {
			return e
		}
		s.a[i+1] = NativeRequesterAddress{Address: value, Chip: true}
	}
	s.c.Word(0, 0x180)
	for _, reg := range []int{1, 2} {
		for {
			v, e := s.word(reg)
			if e != nil {
				return e
			}
			if v == uint16(s.c.D[0]) {
				break
			}
		}
	}
	s.c.D[1] = 15
	for {
		v, e := s.word(4)
		if e != nil {
			return e
		}
		s.c.Word(0, v)
		for _, reg := range []int{1, 2} {
			if e = s.cb.RAM.Write16(int(s.a[reg].Address), v); e != nil {
				return e
			}
			s.a[reg].Address += 4
		}
		if !s.dbf(1) {
			break
		}
	}
	return nil
}
func (s *nativeProgressionAnimation) initial() error {
	if e := s.palette(); e != nil {
		return e
	}
	target, e := s.cb.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	s.a[0] = NativeRequesterAddress{Address: target, Chip: true}
	s.c.D[7] = 0x39
	s.c.Byte(7, 0-uint8(s.c.D[7]))
	for {
		s.c.D[5] = 3
		for {
			s.c.D[6] = 40
			for {
				s.c.D[0] = 0
				ctl, e := s.byte(4)
				if e != nil {
					return e
				}
				s.c.Byte(0, ctl)
				repeat := int8(ctl) < 0
				if repeat {
					s.c.Byte(0, 0-uint8(s.c.D[0]))
				}
				s.c.Word(6, uint16(s.c.D[6])-uint16(s.c.D[0])-1)
				if repeat {
					v, e := s.byte(4)
					if e != nil {
						return e
					}
					s.c.Byte(1, v)
				}
				for {
					v := uint8(s.c.D[1])
					if !repeat {
						v, e = s.byte(4)
						if e != nil {
							return e
						}
					}
					if e = s.writeByte(0, v, true); e != nil {
						return e
					}
					if !s.dbf(0) {
						break
					}
				}
				if uint16(s.c.D[6]) == 0 {
					break
				}
			}
			s.a[0].Address += 0x1f18
			if !s.dbf(5) {
				break
			}
		}
		s.a[0].Address += 0xffff8328
		if !s.dbf(7) {
			break
		}
	}
	return nil
}
func (s *nativeProgressionAnimation) delta() error {
	target, e := s.cb.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	s.a[0] = NativeRequesterAddress{Address: target, Chip: true}
	s.c.D[2], s.c.D[7] = 40, 3
	for {
		first, e := s.cb.RAM.Read8(int(s.a[4].Address))
		if e != nil {
			return e
		}
		if first == 0xff {
			s.a[4].Address++
			s.a[0].Address += 0x1f40
		} else {
			s.c.D[6] = 39
			for {
				s.a[3] = s.a[0]
				s.c.D[5] = 0
				n, e := s.byte(4)
				if e != nil {
					return e
				}
				s.c.Byte(5, n)
				if n != 0 {
					s.c.Word(5, uint16(s.c.D[5])-1)
					for {
						s.c.D[0] = 0
						ctl, e := s.byte(4)
						if e != nil {
							return e
						}
						s.c.Byte(0, ctl)
						switch {
						case int8(ctl) > 0:
							s.c.Word(0, uint16(s.c.D[0])*2)
							offset, e := s.cb.Code.Read16(0x10880 + int(int16(s.c.D[0])))
							if e != nil {
								return e
							}
							s.a[3].Address += uint32(int32(int16(offset)))
						case ctl == 0:
							s.c.D[4] = 0
							n, e := s.byte(4)
							if e != nil {
								return e
							}
							s.c.Byte(4, n)
							s.c.Word(4, uint16(s.c.D[4])-1)
							v, e := s.byte(4)
							if e != nil {
								return e
							}
							s.c.Byte(3, v)
							for {
								if e = s.writeByte(3, uint8(s.c.D[3]), false); e != nil {
									return e
								}
								s.a[3].Address += uint32(int32(int16(s.c.D[2])))
								if !s.dbf(4) {
									break
								}
							}
						default:
							s.c.Word(0, uint16(s.c.D[0])&0x7f)
							s.c.Word(0, uint16(s.c.D[0])-1)
							for {
								v, e := s.byte(4)
								if e != nil {
									return e
								}
								if e = s.writeByte(3, v, false); e != nil {
									return e
								}
								s.a[3].Address += uint32(int32(int16(s.c.D[2])))
								if !s.dbf(0) {
									break
								}
							}
						}
						if !s.dbf(5) {
							break
						}
					}
				}
				s.a[0].Address++
				if !s.dbf(6) {
					break
				}
			}
			s.a[0].Address += 0x1f18
		}
		if !s.dbf(7) {
			break
		}
	}
	return nil
}
