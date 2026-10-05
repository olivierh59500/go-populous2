package populous2

import "fmt"

// AdvanceNativeCampaignPreview is the actual $5278 dispatch and every
// $52f4..$5526 body. Mutable counter/animation words are shared CODE bytes.
func AdvanceNativeCampaignPreview(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress, hardware *NativeCampaignBlitterState) error {
	c := cb.Frame
	offset, e := cb.Code.Read16(0x5528)
	if e != nil {
		return e
	}
	c.Word(0, offset)
	branch, e := cb.Code.Read16(0x52ac + int(int16(c.D[0])))
	if e != nil {
		return e
	}
	c.Word(0, branch)
	pc := 0x52ac + int(int16(branch))
	word := func(v uint16) { c.Word(2, v) }
	counter := func(mask, first uint16) error {
		value, e := cb.Code.Read16(0x552a)
		if e != nil {
			return e
		}
		value++
		if e = cb.Code.Write16(0x552a, value); e != nil {
			return e
		}
		word(value&mask + first)
		return nil
	}
	image := false
	switch pc {
	case 0x5526, 0x53cc, 0x53e0:
		return nil
	case 0x52f4:
		word(0x1d4)
		image = true
	case 0x52fc:
		word(0x870)
		image = true
	case 0x5304:
		word(0xddc)
		image = true
	case 0x530c:
		word(0x25c)
		image = true
	case 0x5314:
		word(0xf5)
	case 0x531c:
		if e = counter(3, 0xa8); e != nil {
			return e
		}
	case 0x5336:
		if e = counter(3, 0x91); e != nil {
			return e
		}
	case 0x5350:
		word(0x2954)
		image = true
	case 0x5358:
		if e = counter(15, 0xc9); e != nil {
			return e
		}
	case 0x5372:
		word(0x5cc)
		image = true
	case 0x537a:
		if e = counter(3, 0); e != nil {
			return e
		}
		v, e := cb.Code.Read8(0x5398 + int(int16(c.D[2])))
		if e != nil {
			return e
		}
		c.Byte(2, v)
		c.Word(2, uint16(c.D[2])&0xff)
	case 0x539c:
		word(0xaf0)
		image = true
	case 0x53a4:
		word(0x934)
		image = true
	case 0x53ac:
		word(0xce4)
		image = true
	case 0x53b4:
		word(0x4c8)
		image = true
	case 0x53bc:
		word(0xce4)
		image = true
	case 0x53c4:
		word(0x9f0)
		image = true
	case 0x53d0:
		word(0x4b8)
		image = true
	case 0x53d8:
		word(0x81c)
		image = true
	case 0x53e4:
		word(0x28b4)
		image = true
	case 0x53ec:
		if e = counter(3, 0); e != nil {
			return e
		}
		c.Word(2, uint16(c.D[2])*2)
		tile, e := cb.Code.Read16(0x545c + int(int16(c.D[2])))
		if e != nil {
			return e
		}
		c.Word(2, tile)
		base, e := cb.Code.Read32(0x5406)
		if e != nil {
			return e
		}
		a[2] = NativeRequesterAddress{Address: base, Absolute: true}
		a[3] = NativeRequesterAddress{Address: uint32(int64(base) + int64(int16(c.D[2]))), Absolute: true}
		if e = campaignPreviewDestination(cb, a); e != nil {
			return e
		}
		a[4] = NativeRequesterAddress{Address: cb.CodeBase + 0x5464, Code: true}
		for {
			delta, e := cb.RAM.Read16(int(a[4].Address))
			if e != nil {
				return e
			}
			a[4].Address += 2
			c.Word(0, delta)
			if delta == 0xff9d {
				return nil
			}
			a[6].Address = uint32(int64(a[6].Address) + int64(int16(c.D[0])))
			saved := *a
			for i := 0; i < 3; i++ {
				if e = campaignChildTileHalf(cb, a, hardware); e != nil {
					return e
				}
			}
			a[2], a[3], a[4], a[6] = saved[2], saved[3], saved[4], saved[6]
			a[3].Address += 12
		}
	case 0x546e:
		word(0xe0)
	case 0x5476:
		value, e := cb.Code.Read16(0x552a)
		if e != nil {
			return e
		}
		value &= 1
		if e = cb.Code.Write16(0x552a, value); e != nil {
			return e
		}
		word(value&1 + 0x8f)
	case 0x5490:
		word(0xc60)
		image = true
	case 0x5498:
		word(0xd3c)
		image = true
	default:
		return fmt.Errorf("native campaign preview body%x unsupported", pc)
	}
	if image {
		value, e := cb.Code.Read16(0x552a)
		if e != nil {
			return e
		}
		if value == 0xffff {
			value = uint16(c.D[2])
			if e = cb.Code.Write16(0x552a, value); e != nil {
				return e
			}
		}
		word(value + 4)
		a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x23d1a, Code: true}
		marker, e := cb.Code.Read16(0x23d1a + int(int16(c.D[2])))
		if e != nil {
			return e
		}
		if int16(marker) <= 0 {
			c.Word(2, uint16(c.D[2])+marker)
		}
		if e = cb.Code.Write16(0x552a, uint16(c.D[2])); e != nil {
			return e
		}
		x, e := cb.Code.Read16(0x552c)
		if e != nil {
			return e
		}
		y, e := cb.Code.Read16(0x552e)
		if e != nil {
			return e
		}
		c.Word(0, x)
		c.Word(1, y+16)
		return DrawNativeCampaignImage(r, cb, a, hardware)
	}
	base, e := cb.Code.Read32(0x54a4)
	if e != nil {
		return e
	}
	a[2] = NativeRequesterAddress{Address: base, Absolute: true}
	c.D[2] = uint32(uint16(c.D[2])) * 12
	a[3] = NativeRequesterAddress{Address: uint32(int64(base) + int64(int16(c.D[2]))), Absolute: true}
	if e = campaignPreviewDestination(cb, a); e != nil {
		return e
	}
	for i := 0; i < 3; i++ {
		if e = campaignChildTileHalf(cb, a, hardware); e != nil {
			return e
		}
	}
	return nil
}

func campaignPreviewDestination(cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress) error {
	target, e := cb.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	a[6] = NativeRequesterAddress{Address: target, Chip: true}
	x, e := cb.Code.Read16(0x552c)
	if e != nil {
		return e
	}
	cb.Frame.Word(0, x>>3)
	a[6].Address = uint32(int64(a[6].Address) + int64(int16(cb.Frame.D[0])))
	y, e := cb.Code.Read16(0x552e)
	if e != nil {
		return e
	}
	cb.Frame.Word(0, y)
	cb.Frame.D[0] = uint32(uint16(cb.Frame.D[0])) * 40
	a[6].Address = uint32(int64(a[6].Address) + int64(int16(cb.Frame.D[0])))
	return nil
}
