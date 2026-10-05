package populous2

import "fmt"

// NativeRequesterAddress keeps CODE and BSS distinct when the World uses
// relative numeric labels for both. CPU callers may retain relocated bases.
type NativeRequesterAddress struct {
	Address  uint32
	Code     bool
	Chip     bool
	Absolute bool // Original physical address, including a selected null row.
}

type nativeRequesterFrameBacking struct {
	Code, Memory FollowerCleanupMemory
	CodeBase     uint32
	Frame        *NativeFrameRegisterContext
	Bitmap       func(uint32) ([]byte, error)
	Sound        func(uint16, *NativeFrameRegisterContext) error
	ReadAbsolute func(uint32) (uint8, error)
}

func (b nativeRequesterFrameBacking) read(p NativeRequesterAddress) (uint8, error) {
	if p.Absolute {
		if b.ReadAbsolute == nil {
			return 0, fmt.Errorf("native requester physical RAM reader missing")
		}
		return b.ReadAbsolute(p.Address)
	}
	if p.Code {
		return b.Code.Read8(int(int64(p.Address) - int64(b.CodeBase)))
	}
	return b.Memory.Read8(int(int64(p.Address) - int64(b.Frame.AddressBase)))
}

// compile follows $4eb6 with the actual source pointer and full register
// writes. Workspace bytes beyond the written terminator remain untouched.
func (b nativeRequesterFrameBacking) compile(definition uint32, parameters []NativeRequesterAddress) error {
	c, code := b.Frame, b.Code
	at := int(int64(definition) - int64(b.CodeBase))
	c.D[1], c.D[2] = 0, 0
	for _, target := range []int{0xab50, 0xab52} {
		value, e := code.Read16(at)
		if e != nil {
			return e
		}
		if e := code.Write16(target, value); e != nil {
			return e
		}
		at += 2
	}
	out := 0xab58
	if uint16(c.D[3]) == 0 {
		out = at
	}
	c.D[0] = uint32(out - 0xab4e)
	if e := code.Write16(0xab4e, uint16(c.D[0])); e != nil {
		return e
	}
	param := 0
	next := func() NativeRequesterAddress {
		if param >= len(parameters) {
			param++
			return NativeRequesterAddress{}
		}
		p := parameters[param]
		param++
		return p
	}
	write := func(v uint8) error { e := code.Write8(out, v); out++; return e }
	for operations := 0; operations < 65536; operations++ {
		c.Word(1, uint16(c.D[1])+1)
		c.D[0] = 0
		v, e := code.Read8(at)
		if e != nil {
			return e
		}
		at++
		c.Byte(0, v)
		if v == 0 {
			c.Word(1, uint16(c.D[1])-1)
			if e := code.Write16(0xab54, uint16(c.D[1])); e != nil {
				return e
			}
			c.Word(2, uint16(c.D[2])+8)
			if e := code.Write16(0xab56, uint16(c.D[2])); e != nil {
				return e
			}
			return code.Write8(out, 0)
		}
		if int8(v) < 0 {
			c.Word(0, uint16(c.D[0])-0x25)
			v = uint8(c.D[0])
		}
		if v == '{' {
			p := next()
			c.D[0] = p.Address
			if p.Address == 0 {
				if e := write(' '); e != nil {
					return e
				}
				continue
			}
			v, e = b.read(p)
			if e != nil {
				return e
			}
			if v == 0 {
				if e := write(' '); e != nil {
					return e
				}
				continue
			}
			at--
			for {
				v, e = b.read(p)
				if e != nil {
					return e
				}
				if v == 0 {
					break
				}
				at++
				p.Address++
				if e := write(v); e != nil {
					return e
				}
			}
			continue
		}
		if e := write(v); e != nil {
			return e
		}
		if v == 'v' && param < len(parameters) && parameters[param].Address != 0 {
			p := next()
			c.D[0] = p.Address
			end := at
			c.D[3] = 0xffffffff
			for {
				c.Word(3, uint16(c.D[3])+1)
				v, e := code.Read8(end)
				if e != nil {
					return e
				}
				if v == 'w' || v == 0x9c {
					break
				}
				end++
			}
			cursor := p
			c.D[4] = 0xffffffff
			for {
				c.Word(4, uint16(c.D[4])+1)
				v, e := b.read(cursor)
				if e != nil {
					return e
				}
				cursor.Address++
				if v == 0 {
					break
				}
			}
			c.Word(4, uint16(c.D[4])-uint16(c.D[3]))
			if int16(c.D[4]) > 0 {
				p.Address += uint32(int32(int16(c.D[4])))
			}
			at += int(int16(c.D[3]))
			c.Word(3, uint16(c.D[3])-1)
			for {
				v, e := b.read(p)
				if e != nil {
					return e
				}
				p.Address++
				if v == 0 {
					p.Address--
					v = 'k'
				}
				if e := write(v); e != nil {
					return e
				}
				c.Word(3, uint16(c.D[3])-1)
				if uint16(c.D[3]) == 0xffff {
					break
				}
			}
		} else if v == '\n' {
			c.Word(2, uint16(c.D[2])+8)
			c.D[1] = 0
		}
	}
	return fmt.Errorf("native requester compiler did not reach a terminator")
}

// click translates $4d9c/$4dac, including signed MOVEM.W inputs, raw marker
// table reads, retained workspace radio mutations and the real sound child.
func (b nativeRequesterFrameBacking) click() (end uint32, failure error) {
	c, m, code := b.Frame, b.Memory, b.Code
	pressed, e := m.Read16(0x140)
	if e != nil {
		return 0, e
	}
	zero := func() (uint32, error) { c.D[0] = 0; return end, nil }
	defer func() {
		if failure == nil {
			failure = m.Write16(0x140, 0)
		}
	}()
	if pressed == 0 {
		return zero()
	}
	x, e := m.Read16(0x134)
	if e != nil {
		return 0, e
	}
	y, e := m.Read16(0x136)
	if e != nil {
		return 0, e
	}
	c.RestoreWord(0, x)
	c.RestoreWord(1, y)
	c.Word(0, uint16(c.D[0])>>3)
	w := [5]uint16{}
	for i := range w {
		w[i], e = code.Read16(0xab4e + i*2)
		if e != nil {
			return 0, e
		}
	}
	c.Word(0, uint16(c.D[0])-w[1])
	if int16(c.D[0]) < 0 {
		return zero()
	}
	c.Word(0, uint16(c.D[0])-w[3])
	if int16(c.D[0]) >= 0 {
		return zero()
	}
	c.Word(1, uint16(c.D[1])-w[2])
	if int16(c.D[1]) < 0 {
		return zero()
	}
	c.Word(1, uint16(c.D[1])-w[4])
	if int16(c.D[1]) > 0 {
		return zero()
	}
	if e := m.Write16(0x140, 0); e != nil {
		return 0, e
	}
	c.Word(0, uint16(c.D[0])+w[3])
	c.Word(1, uint16(c.D[1])+w[4])
	c.D[1] = uint32(int32(int16(c.D[1])))
	if e := frameDivide(c, 1, 8); e != nil {
		return 0, e
	}
	c.Word(2, w[3])
	c.Word(2, uint16(c.D[2])+1)
	c.D[1] = uint32(uint16(c.D[1])) * uint32(uint16(c.D[2]))
	start := 0xab4e + int(int16(w[0]))
	row := start + int(int16(c.D[1]))
	target := row + int(int16(c.D[0]))
	c.D[1] = 0
	marker := func(at int) (uint8, error) {
		c.D[0] = 0
		v, e := code.Read8(at)
		if e != nil {
			return 0, e
		}
		c.Byte(0, v)
		if int8(v) <= 0x5a {
			return v, nil
		}
		c.Word(0, uint16(c.D[0])-0x5b)
		m, e := code.Read8(0x4e92 + int(int16(c.D[0])))
		if e != nil {
			return 0, e
		}
		c.Byte(0, m)
		return v, nil
	}
	for at := row; at <= target; at++ {
		v, e := marker(at)
		if e != nil {
			return 0, e
		}
		if int8(v) > 0x5a && uint8(c.D[0]) != 0 {
			c.Byte(1, v)
			if int8(c.D[0]) < 0 {
				c.Word(1, 0)
			}
		}
	}
	if uint16(c.D[1]) == 0 {
		return zero()
	}
	c.D[2] = 0
	selected := -1
	for at := start; at <= target; at++ {
		v, e := marker(at)
		if e != nil {
			return 0, e
		}
		if int8(v) > 0x5a && int8(c.D[0]) > 0 {
			c.Word(2, uint16(c.D[2])+2)
			selected = at
		}
	}
	end = b.CodeBase + uint32(target+1)
	if selected < 0 {
		return 0, fmt.Errorf("native requester active marker pointer missing")
	}
	v, e := code.Read8(selected)
	if e != nil {
		return 0, e
	}
	if v == 'c' || v == 'd' {
		if e := code.Write8(selected, v^7); e != nil {
			return 0, e
		}
	}
	c.Word(0, 0x1d6)
	if b.Sound == nil {
		return 0, fmt.Errorf("native requester sound184f6 missing")
	}
	if e := b.Sound(0x1d6, c); e != nil {
		return 0, e
	}
	c.Word(0, uint16(c.D[2]))
	return end, nil
}

// text is $509a's actual planar byte writer. Adjacent CODE glyph reads and
// signed destination arithmetic remain raw; unavailable backing is an error.
func (b nativeRequesterFrameBacking) text(target uint32, text int) error {
	c := b.Frame
	if b.Bitmap == nil {
		return fmt.Errorf("native requester bitmap resolver missing")
	}
	bitmap, e := b.Bitmap(target)
	if e != nil {
		return e
	}
	saved6, saved7 := c.D[6], c.D[7]
	c.D[1] = uint32(uint16(c.D[1])) * 40
	c.Word(6, uint16(c.D[0]))
	c.Word(7, uint16(c.D[6]))
	base := int(int16(c.D[1])) + int(int16(c.D[0]))
	at := base
	for operations := 0; operations < 65536; operations++ {
		c.D[0] = 0
		v, e := b.Code.Read8(text)
		if e != nil {
			return e
		}
		text++
		c.Byte(0, v)
		if v == 0 {
			c.D[6], c.D[7] = saved6, saved7
			return nil
		}
		c.Word(0, uint16(c.D[0])-0x20)
		if int16(c.D[0]) == -22 {
			c.Word(7, uint16(c.D[6]))
			c.Word(1, uint16(c.D[1])+40)
			if int16(c.D[1]) >= 8000 {
				return fmt.Errorf("native requester text reached illegal5190 height")
			}
			base += 320
			at = base
			continue
		}
		c.Word(0, uint16(c.D[0])<<5)
		if int16(c.D[7]) >= 40 {
			c.D[6], c.D[7] = saved6, saved7
			return nil
		}
		source := 0x33c68 + int(int16(c.D[0]))
		for row := range 8 {
			for plane := range 4 {
				p := at + row*40 + plane*8000
				if p < 0 || p >= len(bitmap) {
					return fmt.Errorf("native requester text destination outside retained RAM")
				}
				bitmap[p], e = b.Code.Read8(source + row*4 + plane)
				if e != nil {
					return e
				}
			}
		}
		at++
		c.Word(7, uint16(c.D[7])+1)
	}
	return fmt.Errorf("native requester text did not reach a terminator")
}
