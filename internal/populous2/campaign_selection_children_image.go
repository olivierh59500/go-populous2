package populous2

import "fmt"

// DrawNativeCampaignImage is $ee32 over mutable CODE and physical descriptor
// planes. Its direct sound-counter writes share the real CODE audio bank.
func DrawNativeCampaignImage(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress, hardware ...*NativeCampaignBlitterState) error {
	c := cb.Frame
	saved0, saved1, savedA3 := c.D[0], c.D[1], a[3]
	defer func() { c.D[0], c.D[1], a[3] = saved0, saved1, savedA3 }()
	c.D[2] &= 0xffff
	if c.D[2] >= 0x2c38 {
		return nil
	}
	a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x23d1a + c.D[2], Code: true}
	c.Word(2, uint16(c.D[2])&3)
	if uint16(c.D[2]) != 0 {
		return nil
	}
	cue, e := cb.RAM.Read16(int(a[0].Address) + 2)
	if e != nil {
		return e
	}
	c.D[2] = uint32(cue)
	if cue != 0 && cue < 0x532 {
		a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x185a8, Code: true}
		value, e := cb.RAM.Read16(int(a[1].Address) + int(int16(c.D[2])))
		if e != nil {
			return e
		}
		if e = cb.RAM.Write16(int(a[1].Address)+int(int16(c.D[2])), value+1); e != nil {
			return e
		}
	}
	image, e := cb.RAM.Read16(int(a[0].Address))
	if e != nil {
		return e
	}
	c.Word(0, image)
	if int16(c.D[0]) < 0 {
		return nil
	}
	for layers := 0; layers < 65536; layers++ {
		c.Word(0, uint16(c.D[0])*2)
		c.D[0] &= 0xffff
		a[2] = NativeRequesterAddress{Address: cb.CodeBase + 0x26956 + c.D[0], Code: true}
		x, e := cb.RAM.Read8(int(a[2].Address))
		if e != nil {
			return e
		}
		a[2].Address++
		y, e := cb.RAM.Read8(int(a[2].Address))
		if e != nil {
			return e
		}
		a[2].Address++
		c.Byte(0, x)
		c.Byte(1, y)
		c.ExtendWord(0)
		c.ExtendWord(1)
		offset, e := cb.RAM.Read16(int(a[2].Address))
		if e != nil {
			return e
		}
		a[2].Address += 2
		a[4] = NativeRequesterAddress{Address: uint32(int64(cb.CodeBase+0x21626) + int64(int16(offset))), Code: true}
		source, e := cb.RAM.Read32(int(a[4].Address))
		if e != nil {
			return e
		}
		a[4].Address += 4
		a[1] = NativeRequesterAddress{Address: source, Absolute: true}
		half, e := cb.RAM.Read16(int(a[4].Address))
		if e != nil {
			return e
		}
		a[4].Address += 2
		c.Word(0, uint16(c.D[0])-half)
		height, e := cb.RAM.Read16(int(a[4].Address))
		if e != nil {
			return e
		}
		a[4].Address += 2
		c.Word(2, height)
		routine, e := cb.RAM.Read32(int(a[4].Address))
		if e != nil {
			return e
		}
		a[4] = NativeRequesterAddress{Address: routine, Code: true}
		c.Word(1, uint16(c.D[1])-height)
		c.Word(0, uint16(c.D[0])+uint16(saved0))
		c.Word(1, uint16(c.D[1])+uint16(saved1))
		if e = cb.Code.Write16(0xeee0, uint16(c.D[1])); e != nil {
			return e
		}
		next, e := cb.RAM.Read16(int(a[2].Address))
		if e != nil {
			return e
		}
		target, e := cb.Memory.Read32(0x1e)
		if e != nil {
			return e
		}
		a[0] = NativeRequesterAddress{Address: target, Chip: true}
		if routine == cb.CodeBase+0xf0ee || routine == cb.CodeBase+0xf3a0 {
			if e = campaignChildSprite(r, cb, a, routine, hardware...); e != nil {
				return e
			}
		} else if routine == cb.CodeBase+0xef5c {
			if e = campaignChildSoftwareSprite(cb, a); e != nil {
				return e
			}
		}
		c.Word(0, next)
		if next == 0 {
			return nil
		}
	}
	return fmt.Errorf("native campaign image chain did not terminate")
}
