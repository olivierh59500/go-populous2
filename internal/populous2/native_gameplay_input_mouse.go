package populous2

import "fmt"

func (s *NativeGameplayInputState) advanceMouse(cb NativeGameplayInputCallbacks, out NativeGameplayInputStep, child func(int, int) (bool, error)) (NativeGameplayInputStep, error) {
	c, m, code := cb.Frame, cb.Memory, cb.Code
	w := func(at int) (uint16, error) { return m.Read16(at) }
	add := func(at int, value uint16) error {
		v, e := w(at)
		if e != nil {
			return e
		}
		return m.Write16(at, v+value)
	}
	command := func(value uint8) error {
		p, e := m.Read32(0xeb6a)
		if e != nil {
			return e
		}
		s.A[0] = NativeRequesterAddress{Address: p}
		return cb.RAM.Write8(int(p)+1, value)
	}
	asr := func(value uint16, count uint) uint16 { return uint16(int16(value) >> count) }
	for transitions := 0; transitions < 512; transitions++ {
		switch s.PC {
		case 0x1560:
			view, e := w(0xf0c)
			if e != nil {
				return out, e
			}
			if view != 8 {
				s.PC = 0x1804
				continue
			}
			left, e := w(0x140)
			if e != nil {
				return out, e
			}
			at := 0x134
			if left == 0 {
				right, e := w(0x146)
				if e != nil {
					return out, e
				}
				if right == 0 {
					s.PC = 0x1804
					continue
				}
				at = 0x138
			}
			x, e := w(at)
			if e != nil {
				return out, e
			}
			y, e := w(at + 2)
			if e != nil {
				return out, e
			}
			c.RestoreWord(6, x)
			c.RestoreWord(7, y)
			c.Word(0, asr(uint16(c.D[6])-68, 1))
			c.Word(1, uint16(c.D[7])-4)
			c.D[3] = 0
			c.Word(3, uint16(c.D[1])-uint16(c.D[0]))
			c.D[2] = 0
			c.Word(2, uint16(c.D[0])+uint16(c.D[1]))
			if int16(c.D[2]) < 0 || int16(c.D[3]) < 0 {
				s.PC = 0x1612
				continue
			}
			c.D[0] = 64
			if int16(c.D[2]) >= 64 || int16(c.D[3]) >= 64 {
				s.PC = 0x1612
				continue
			}
			c.Word(2, uint16(c.D[2])-3)
			c.Word(3, uint16(c.D[3])-3)
			s.PC = 0x15c6
		case 0x15c6:
			left, e := w(0x140)
			if e != nil {
				return out, e
			}
			if left == 0 {
				if e = m.Write16(0x142, 0); e != nil {
					return out, e
				}
				if e = m.Write16(0x5f44, uint16(c.D[2])); e != nil {
					return out, e
				}
				if e = m.Write16(0x5f46, uint16(c.D[3])); e != nil {
					return out, e
				}
				s.PC = 0x1804
				continue
			}
			if e = m.Write16(0x140, 0); e != nil {
				return out, e
			}
			for _, v := range []struct{ reg, at int }{{2, 0x5f44}, {3, 0x5f46}} {
				camera, e := w(v.at)
				if e != nil {
					return out, e
				}
				before := uint16(c.D[v.reg])
				difference := before - camera
				c.Word(v.reg, difference)
				signedLess := (int16(before) < int16(camera))
				c.D[0] = 1
				if !signedLess {
					if int16(before) > int16(camera) {
						c.Byte(0, 255)
					} else {
						c.Byte(0, 0)
					}
					c.Word(0, uint16(int16(int8(c.D[0]))))
				}
				if e = m.Write16(v.at, camera-uint16(c.D[0])); e != nil {
					return out, e
				}
			}
			s.PC = 0x1804
		case 0x1612:
			c.Word(2, uint16(c.D[2])+6)
			if int16(c.D[2]) < 0 {
				s.PC = 0x1662
				continue
			}
			c.Word(3, uint16(c.D[3])+6)
			if int16(c.D[3]) < 0 {
				s.PC = 0x1662
				continue
			}
			c.D[2] = uint32(uint16(c.D[2])) * 0xa3e
			c.Swap(2)
			c.D[3] = uint32(uint16(c.D[3])) * 0xa3e
			c.Swap(3)
			if int16(c.D[2]) <= 2 && int16(c.D[3]) <= 2 {
				if e := m.Write16(0x140, 0); e != nil {
					return out, e
				}
				if e := m.Write16(0x142, 0); e != nil {
					return out, e
				}
				c.Word(2, uint16(c.D[2])-1)
				c.Word(3, uint16(c.D[3])-1)
				if e := code.Write16(0x1be4, uint16(c.D[2])); e != nil {
					return out, e
				}
				if e := code.Write16(0x1be6, uint16(c.D[3])); e != nil {
					return out, e
				}
				if e := add(0x5f44, uint16(c.D[2])); e != nil {
					return out, e
				}
				if e := add(0x5f46, uint16(c.D[3])); e != nil {
					return out, e
				}
			}
			s.PC = 0x1662
		case 0x1662:
			c.Word(0, asr(uint16(c.D[6])-317, 1))
			c.Word(1, uint16(c.D[7])-140)
			c.Word(3, uint16(c.D[1])-uint16(c.D[0]))
			c.Word(2, uint16(c.D[0])+uint16(c.D[1]))
			c.Word(0, uint16(c.D[2]))
			if int16(c.D[0]) < 0 {
				s.PC = 0x17e4
				continue
			}
			c.Word(1, uint16(c.D[3]))
			if int16(c.D[1]) < 0 {
				s.PC = 0x17e4
				continue
			}
			c.Word(0, asr(uint16(c.D[0]), 4))
			c.Word(1, asr(uint16(c.D[1]), 4)<<2)
			c.Word(0, uint16(c.D[0])+uint16(c.D[1]))
			if int16(c.D[0]) < 0 || int16(c.D[0]) > 24 {
				s.PC = 0x17e4
				continue
			}
			c.Word(0, uint16(c.D[0])*2)
			offset, e := code.Read16(0x16a2 + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			c.Word(0, offset)
			s.PC = 0x16a2 + int(int16(offset))
		case 0x16d4, 0x16fc, 0x1716, 0x1730, 0x174a, 0x17c0, 0x17c2:
			if s.PC == 0x17c0 {
				s.PC = 0x17c2
			}
			c.Word(0, 0x1cc)
			s.PC += 4
		case 0x16d8, 0x1700, 0x171a, 0x1734, 0x174e, 0x17c6:
			pc := s.PC
			done, e := child(0x184f6, pc+6)
			if e != nil || !done {
				return out, e
			}
		case 0x16de:
			right, e := w(0x142)
			if e != nil {
				return out, e
			}
			if right != 0 {
				s.PC = 0x16e6
			} else {
				if e = m.Write16(0xeb18, 12); e != nil {
					return out, e
				}
				s.PC = 0x17d8
			}
		case 0x16e6:
			done, e := child(0x2914, 0x17d8)
			if e != nil || !done {
				return out, e
			}
		case 0x1706, 0x1720, 0x173a, 0x17cc:
			value := uint8(118)
			switch s.PC {
			case 0x1706:
				value = 14
			case 0x1720:
				value = 18
			case 0x173a:
				value = 20
			}
			if e := command(value); e != nil {
				return out, e
			}
			s.PC = 0x17d8
		case 0x1754:
			right, e := w(0x146)
			if e != nil {
				return out, e
			}
			if right == 0 {
				if e = command(16); e != nil {
					return out, e
				}
				s.PC = 0x17d8
				continue
			}
			if e = m.Write16(0x142, 0); e != nil {
				return out, e
			}
			owner, e := w(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(0, owner)
			c.D[0] = uint32(uint16(c.D[0])) * 314
			s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe76a) + int64(int16(c.D[0])))}
			value, e := cb.RAM.Read16(int(s.A[1].Address) + 8)
			if e != nil {
				return out, e
			}
			c.Word(0, value)
			if value != 0 {
				s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0x76c0) + int64(int16(value)))}
				s.PC = 0x1782
			} else {
				value, e = cb.RAM.Read16(int(s.A[1].Address) + 10)
				if e != nil {
					return out, e
				}
				c.Word(0, value)
				s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0x76c0) + int64(int16(value)))}
				s.PC = 0x1796
			}
		case 0x1782:
			done, e := child(0x29d2, 0x1796)
			if e != nil || !done {
				return out, e
			}
		case 0x1796:
			c.D[0] = 0
			value, e := cb.RAM.Read8(int(s.A[1].Address) + 6)
			if e != nil {
				return out, e
			}
			c.Byte(0, value)
			c.Word(0, uint16(c.D[0])-4)
			if e = m.Write16(0x5f44, uint16(c.D[0])); e != nil {
				return out, e
			}
			value, e = cb.RAM.Read8(int(s.A[1].Address) + 8)
			if e != nil {
				return out, e
			}
			c.Byte(0, value)
			c.Word(0, uint16(c.D[0])-4)
			if e = m.Write16(0x5f46, uint16(c.D[0])); e != nil {
				return out, e
			}
			s.PC = 0x17d8
		case 0x17d8:
			if e := m.Write16(0x142, 0); e != nil {
				return out, e
			}
			if e := m.Write16(0x140, 0); e != nil {
				return out, e
			}
			s.PC = 0x17e4
		case 0x17e4:
			done, e := child(0x23ee, 0x17ea)
			if e != nil || !done {
				return out, e
			}
		case 0x17ea:
			if s.ChildZero {
				s.PC = 0x1804
			} else {
				if e := m.Write16(0x140, 0); e != nil {
					return out, e
				}
				if e := m.Write16(0x142, 0); e != nil {
					return out, e
				}
				if e := m.Write32(0x5f48, 0); e != nil {
					return out, e
				}
				s.PC = 0x1bd0
			}
		case 0x1804:
			if e := m.Write32(0x5f48, 0); e != nil {
				return out, e
			}
			if e := m.Write32(0x5f4c, 0); e != nil {
				return out, e
			}
			x, e := w(0x138)
			if e != nil {
				return out, e
			}
			y, e := w(0x13a)
			if e != nil {
				return out, e
			}
			c.RestoreWord(6, x)
			c.RestoreWord(7, y)
			px, e := code.Read16(0xe458)
			if e != nil {
				return out, e
			}
			c.Word(6, uint16(c.D[6])-px)
			view, e := w(0xf0c)
			if e != nil {
				return out, e
			}
			c.Word(0, view)
			c.Word(0, uint16(c.D[0])<<4)
			py, e := code.Read16(0xe45a)
			if e != nil {
				return out, e
			}
			c.Word(0, uint16(c.D[0])+py+2)
			c.Word(7, uint16(c.D[7])-uint16(c.D[0]))
			c.Word(6, asr(uint16(c.D[6]), 1))
			c.Word(0, uint16(c.D[6])+uint16(c.D[7]))
			if int16(c.D[0]) > 0 || int16(c.D[7]) > int16(c.D[6]) {
				s.PC = 0x1bd0
				continue
			}
			s.PC = 0x1840
		case 0x1840:
			x, e := w(0x138)
			if e != nil {
				return out, e
			}
			y, e := w(0x13a)
			if e != nil {
				return out, e
			}
			c.RestoreWord(6, x)
			c.RestoreWord(7, y)
			x, e = w(0x5f44)
			if e != nil {
				return out, e
			}
			y, e = w(0x5f46)
			if e != nil {
				return out, e
			}
			c.RestoreWord(0, x)
			c.RestoreWord(1, y)
			view, e := w(0xf0c)
			if e != nil {
				return out, e
			}
			c.Word(2, asr(view, 1)-4)
			c.Word(0, uint16(c.D[0])-uint16(c.D[2]))
			c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
			c.Word(6, uint16(c.D[6])+8)
			if e = m.Write16(0x5f48, uint16(c.D[6])); e != nil {
				return out, e
			}
			if e = m.Write16(0x5f4a, 0); e != nil {
				return out, e
			}
			c.Word(2, 0-(view<<4))
			px, e := code.Read16(0xe458)
			if e != nil {
				return out, e
			}
			c.Word(2, uint16(c.D[2])+px)
			c.Word(6, uint16(c.D[6])-uint16(c.D[2]))
			if int16(c.D[6]) < 0 {
				s.PC = 0x1bd0
				continue
			}
			c.Word(6, uint16(c.D[6])>>4)
			if int16(c.D[6]) < int16(view) {
				c.Word(2, uint16(c.D[6]))
				c.Word(3, view)
				c.Word(4, uint16(c.D[2]))
			} else {
				c.Word(2, view)
				c.Word(3, view*2)
				if int16(c.D[3]) < int16(c.D[6]) {
					s.PC = 0x1bd0
					continue
				}
				c.Word(3, uint16(c.D[3])-uint16(c.D[6]))
				c.Word(4, uint16(c.D[3]))
			}
			c.Word(5, uint16(c.D[4])<<3)
			py, e := code.Read16(0xe45a)
			if e != nil {
				return out, e
			}
			c.Word(5, uint16(c.D[5])+(view<<3)+py-4)
			c.Word(0, uint16(c.D[0])+uint16(c.D[2]))
			c.Word(1, uint16(c.D[1])+uint16(c.D[3]))
			s.PC = 0x18ca
		case 0x18ca:
			done, e := child(0xd2b4, 0x18d0)
			if e != nil || !done {
				return out, e
			}
		case 0x18d0:
			c.Word(2, 0-(uint16(c.D[2])<<3)+uint16(c.D[5]))
			c.Word(3, uint16(c.D[2]))
			before, d7 := uint16(c.D[2]), uint16(c.D[7])
			c.Word(2, before-d7)
			if int16(before) < int16(d7) {
				s.PC = 0x18ec
				continue
			}
			c.Word(0, uint16(c.D[0])-1)
			c.Word(1, uint16(c.D[1])-1)
			c.Word(5, uint16(c.D[5])-16)
			c.Word(4, uint16(c.D[4])-1)
			if uint16(c.D[4]) != 0xffff {
				s.PC = 0x18ca
			} else {
				s.PC = 0x1bd0
			}
		case 0x18ec:
			if uint16(c.D[0]) >= 65 || uint16(c.D[1]) >= 65 {
				s.PC = 0x1bd0
				continue
			}
			if e := m.Write16(0x5f4a, uint16(c.D[3])); e != nil {
				return out, e
			}
			if e := m.Write16(0x5f4c, uint16(c.D[0])); e != nil {
				return out, e
			}
			if e := m.Write16(0x5f4e, uint16(c.D[1])); e != nil {
				return out, e
			}
			tool, e := w(0xeb18)
			if e != nil {
				return out, e
			}
			if tool == 50 || tool == 30 {
				held, e := w(0x144)
				if e != nil {
					return out, e
				}
				if held != 0 {
					s.PC = 0x194e
					continue
				}
				if tool == 50 {
					if e = m.Write16(0xeb18, 2); e != nil {
						return out, e
					}
				} else {
					if e = m.Write16(0xeb18, 32); e != nil {
						return out, e
					}
					s.PC = 0x194e
					continue
				}
			}
			s.PC = 0x1944
		case 0x1944:
			left, e := w(0x140)
			if e != nil {
				return out, e
			}
			if left == 0 {
				s.PC = 0x1aaa
			} else {
				s.PC = 0x194e
			}
		case 0x194e:
			if e := m.Write16(0x140, 0); e != nil {
				return out, e
			}
			edit, e := w(0xf0e)
			if e != nil {
				return out, e
			}
			tool, e := w(0xf10)
			if e != nil {
				return out, e
			}
			if edit != 0 && tool != 0 {
				s.A[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x20ad6, Code: true}
				value, e := code.Read16(0x20ad6 + int(int16(tool)))
				if e != nil {
					return out, e
				}
				c.Word(2, value)
				s.PC = 0x1a98
				continue
			}
			tool, e = w(0xeb18)
			if e != nil {
				return out, e
			}
			owner, e := w(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(2, tool)
			c.Word(3, owner)
			branch, e := code.Read16(0x1986 + int(int16(tool)))
			if e != nil {
				return out, e
			}
			c.Word(4, branch)
			s.PC = 0x1986 + int(int16(branch))
		case 0x19d8, 0x1b32:
			owner, e := w(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(3, owner)
			next := 0x19e4
			if s.PC == 0x1b32 {
				next = 0x1b3e
			}
			done, e := child(0xd91a, next)
			if e != nil || !done {
				return out, e
			}
		case 0x19e4:
			if uint16(c.D[3]) == 9 {
				s.PC = 0x1aaa
			} else if int16(c.D[3]) <= 0 {
				s.PC = 0x1a20
			} else {
				s.PC = 0x1aaa
			}
		case 0x19f4:
			owner, e := w(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(3, owner)
			c.D[3] = uint32(uint16(c.D[3])) * 314
			s.A[0] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe76a) + int64(int16(c.D[3])))}
			value, e := cb.RAM.Read8(int(s.A[0].Address) + 0x4b)
			if e != nil {
				return out, e
			}
			if value&1 == 0 {
				s.PC = 0x1aaa
			} else {
				s.PC = 0x1a28
			}
		case 0x1a12:
			clock, e := w(0xf42)
			if e != nil {
				return out, e
			}
			c.Word(3, (clock&0x18)<<3)
			c.Word(0, uint16(c.D[0])|uint16(c.D[3]))
			s.PC = 0x1a20
		case 0x1a20:
			if e := m.Write16(0xeb18, 2); e != nil {
				return out, e
			}
			s.PC = 0x1a28
		case 0x1a28, 0x1a52, 0x1a78, 0x1b54:
			owner, e := w(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(3, owner)
			next := map[int]int{0x1a28: 0x1a34, 0x1a52: 0x1a5e, 0x1a78: 0x1a84, 0x1b54: 0x1b60}[s.PC]
			done, e := child(0x147e0, next)
			if e != nil || !done {
				return out, e
			}
		case 0x1a34, 0x1a5e, 0x1a84, 0x1b60:
			pc := s.PC
			if s.ChildZero {
				if pc == 0x1b60 {
					s.PC = 0x1bbe
				} else {
					s.PC = 0x1a98
				}
				continue
			}
			switch pc {
			case 0x1a34:
				s.PC = 0x1aaa
			case 0x1a5e:
				if e := m.Write16(0xeb18, 2); e != nil {
					return out, e
				}
				s.PC = 0x1a98
			case 0x1a84:
				c.Word(2, 32)
				if e := m.Write16(0xeb18, 2); e != nil {
					return out, e
				}
				s.PC = 0x1a98
			case 0x1b60:
				s.PC = 0x1bd0
			}
		case 0x1a3c:
			if e := m.Write16(0xeb18, 2); e != nil {
				return out, e
			}
			s.PC = 0x1a98
		case 0x1a48, 0x1a6e:
			value := uint16(50)
			if s.PC == 0x1a6e {
				value = 30
			}
			c.Word(2, value)
			if e := m.Write16(0xeb18, value); e != nil {
				return out, e
			}
			if value == 50 {
				s.PC = 0x1a52
			} else {
				s.PC = 0x1a78
			}
		case 0x1a88:
			c.Word(2, 32)
			if e := m.Write16(0xeb18, 2); e != nil {
				return out, e
			}
			s.PC = 0x1a98
		case 0x1a98, 0x1bbe:
			if e := command(uint8(c.D[2])); e != nil {
				return out, e
			}
			if e := cb.RAM.Write8(int(s.A[0].Address)+2, uint8(c.D[0])); e != nil {
				return out, e
			}
			if e := cb.RAM.Write8(int(s.A[0].Address)+3, uint8(c.D[1])); e != nil {
				return out, e
			}
			if s.PC == 0x1a98 {
				s.PC = 0x1aaa
			} else {
				s.PC = 0x1bd0
			}
		case 0x1aaa:
			right, e := w(0x142)
			if e != nil {
				return out, e
			}
			if right == 0 {
				s.PC = 0x1bd0
				continue
			}
			if e = m.Write16(0x142, 0); e != nil {
				return out, e
			}
			edit, e := w(0xf0e)
			if e != nil {
				return out, e
			}
			tool, e := w(0xf10)
			if e != nil {
				return out, e
			}
			if edit != 0 && tool != 0 {
				c.Byte(2, 102)
				s.PC = 0x1bbe
				continue
			}
			tool, e = w(0xeb18)
			if e != nil {
				return out, e
			}
			c.Word(2, tool)
			branch, e := code.Read16(0x1ae0 + int(int16(tool)))
			if e != nil {
				return out, e
			}
			c.Word(4, branch)
			s.PC = 0x1ae0 + int(int16(branch))
		case 0x1b3e:
			if uint16(c.D[3]) == 9 {
				s.PC = 0x1bd0
			} else if int16(c.D[3]) <= 1 {
				c.Byte(2, 4)
				s.PC = 0x1b54
			} else {
				s.PC = 0x1bd0
			}
		case 0x1b68:
			c.Word(3, uint16(c.D[1])<<8)
			c.Byte(3, uint8(c.D[0]))
			c.Byte(3, uint8(c.D[3])*4)
			s.A[2] = NativeRequesterAddress{Address: c.AddressBase + 0xf44}
			s.A[4] = NativeRequesterAddress{Address: cb.CodeBase + 0x33312, Code: true}
			tile, e := cb.RAM.Read8(int(s.A[2].Address) + int(int16(c.D[3])) + 1)
			if e != nil {
				return out, e
			}
			c.Byte(3, tile)
			c.Word(3, uint16(c.D[3])&255)
			c.Word(3, uint16(c.D[3])*2)
			value, e := code.Read8(0x33312 + int(int16(c.D[3])) + 1)
			if e != nil {
				return out, e
			}
			if value&64 != 0 {
				c.Byte(2, 44)
				s.PC = 0x1bbe
			} else {
				if e = m.Write16(0xeb18, 2); e != nil {
					return out, e
				}
				s.PC = 0x1bd0
			}
		case 0x1b98:
			if e := m.Write16(0xeb18, 2); e != nil {
				return out, e
			}
			s.PC = 0x1bd0
		case 0x1ba4:
			if e := m.Write16(0xeb18, 2); e != nil {
				return out, e
			}
			s.PC = 0x1bbe
		case 0x1bb0:
			c.Word(2, 32)
			if e := m.Write16(0xeb18, 32); e != nil {
				return out, e
			}
			s.PC = 0x1bbe
		case 0x1bd0:
			value, e := w(0x3aa)
			if e != nil {
				return out, e
			}
			s.Finished = true
			s.PC = 0
			out.Complete = true
			out.ExitRequested = value != 0
			return out, nil
		default:
			return out, fmt.Errorf("native gameplay mouse PC%x unsupported", s.PC)
		}
	}
	return out, fmt.Errorf("native gameplay mouse transitions exceeded")
}
