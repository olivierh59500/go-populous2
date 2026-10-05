package populous2

import "math/bits"

// campaignChildSoftwareSprite is the original $ef5c interleaved software
// compositor, including its three shift paths and surviving D/A writes.
func campaignChildSoftwareSprite(cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress) error {
	c := cb.Frame
	m := cb.RAM
	rw := func(address uint32) (uint16, error) { return m.Read16(int(address)) }
	ww := func(address uint32, v uint16) error { return m.Write16(int(address), v) }
	next := func() (byte, error) {
		v, e := m.Read8(int(a[1].Address))
		if e == nil {
			a[1].Address++
		}
		return v, e
	}
	if int16(c.D[1]) < 0 {
		c.Word(1, -uint16(c.D[1]))
		c.Word(2, uint16(c.D[2])-uint16(c.D[1]))
		if int16(c.D[2]) <= 0 {
			return nil
		}
		c.Word(7, uint16(c.D[1]))
		c.Word(1, uint16(c.D[1])*4)
		c.Word(1, uint16(c.D[1])+uint16(c.D[7]))
		a[1].Address = uint32(int64(a[1].Address) + int64(int16(c.D[1])))
	} else {
		c.D[6] = 200
		if int16(c.D[1]) >= 200 {
			return nil
		}
		c.Word(7, uint16(c.D[1])*40)
		a[0].Address = uint32(int64(a[0].Address) + int64(int16(c.D[7])))
		c.Word(1, uint16(c.D[1])+uint16(c.D[2])-200)
		if int16(c.D[1]) >= 0 {
			c.Word(2, uint16(c.D[2])-uint16(c.D[1]))
		}
	}
	c.Word(2, uint16(c.D[2])-1)
	c.Word(1, uint16(c.D[0]))
	c.Word(0, (uint16(c.D[0])>>3)&0xfffe)
	a[0].Address = uint32(int64(a[0].Address) + int64(int16(c.D[0])))
	c.Word(1, uint16(c.D[1])&15)
	shift := uint16(c.D[1])
	rows := uint16(c.D[2])
	if shift < 8 {
		c.D[0] = 8
		c.Word(0, uint16(c.D[0])-shift)
		c.Word(5, 38)
		c.Word(1, rows)
		c.D[2] = 0xffffffff
		for {
			c.D[2] = 0xffffffff
			v, e := next()
			if e != nil {
				return e
			}
			c.Byte(2, ^v)
			c.Word(2, bits.RotateLeft16(uint16(c.D[2]), int(uint16(c.D[0])&63)))
			for plane := 0; plane < 4; plane++ {
				address := a[0].Address + uint32(plane)*8000
				old, e := rw(address)
				if e != nil {
					return e
				}
				c.Word(3, old&uint16(c.D[2]))
				c.Word(4, 0)
				v, e := next()
				if e != nil {
					return e
				}
				c.Byte(4, v)
				c.Word(4, bits.RotateLeft16(uint16(c.D[4]), int(uint16(c.D[0])&63)))
				c.Word(3, uint16(c.D[3])|uint16(c.D[4]))
				if e = ww(address, uint16(c.D[3])); e != nil {
					return e
				}
			}
			a[0].Address += 40
			c.Word(1, uint16(c.D[1])-1)
			if uint16(c.D[1]) == 0xffff {
				return nil
			}
		}
	} else if shift == 8 {
		c.Word(5, 38)
		c.Word(1, rows)
		for {
			c.D[2] = 0xffffffff
			v, e := next()
			if e != nil {
				return e
			}
			c.Byte(2, v)
			for plane := 0; plane < 4; plane++ {
				address := a[0].Address + uint32(plane)*8000
				old, e := rw(address)
				if e != nil {
					return e
				}
				c.Word(3, old)
				c.Byte(3, uint8(c.D[3])&uint8(c.D[2]))
				v, e := next()
				if e != nil {
					return e
				}
				c.Byte(4, v)
				c.Byte(3, uint8(c.D[3])|uint8(c.D[4]))
				if e = ww(address, uint16(c.D[3])); e != nil {
					return e
				}
			}
			a[0].Address += 40
			c.Word(1, uint16(c.D[1])-1)
			if uint16(c.D[1]) == 0xffff {
				return nil
			}
		}
	}
	c.D[1] &^= 8
	c.Word(0, rows)
	for {
		c.D[2] = 0xffffffff
		v, e := next()
		if e != nil {
			return e
		}
		c.Byte(2, v)
		c.Word(2, bits.RotateLeft16(uint16(c.D[2]), -int(uint16(c.D[1])&63)))
		for i := 4; i <= 7; i++ {
			c.Word(i, 0)
			v, e := next()
			if e != nil {
				return e
			}
			c.Byte(i, v)
			c.Word(i, bits.RotateLeft16(uint16(c.D[i]), -int(uint16(c.D[1])&63)))
		}
		for plane := 0; plane < 4; plane++ {
			address := a[0].Address + uint32(plane)*8000
			old, e := rw(address)
			if e != nil {
				return e
			}
			c.Word(3, old)
			c.Byte(3, uint8(c.D[3])&uint8(c.D[2]))
			c.Byte(3, uint8(c.D[3])|uint8(c.D[4+plane]))
			if e = ww(address, uint16(c.D[3])); e != nil {
				return e
			}
		}
		c.Byte(2, 0xff)
		for i := 4; i <= 7; i++ {
			c.Byte(i, 0)
		}
		for plane := 0; plane < 4; plane++ {
			address := a[0].Address + 2 + uint32(plane)*8000
			old, e := rw(address)
			if e != nil {
				return e
			}
			c.Word(3, old&uint16(c.D[2]))
			c.Word(3, uint16(c.D[3])|uint16(c.D[4+plane]))
			if e = ww(address, uint16(c.D[3])); e != nil {
				return e
			}
		}
		a[0].Address += 40
		c.Word(0, uint16(c.D[0])-1)
		if uint16(c.D[0]) == 0xffff {
			return nil
		}
	}
}
