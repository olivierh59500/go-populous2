package populous2

// DrawNativeProgressionPortrait is the actual $baee entry; the $baa8
// background copy is a distinct caller and is not repeated here.
func DrawNativeProgressionPortrait(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress) error {
	c := cb.Frame
	c.D[3] = 2
	for part := 0; part < 3; part++ {
		c.D[0] = 0
		a[3].Address--
		variant, e := cb.RAM.Read8(int(a[3].Address))
		if e != nil {
			return e
		}
		c.Byte(0, variant)
		bank, e := cb.RAM.Read16(int(a[2].Address))
		if e != nil {
			return e
		}
		a[2].Address += 2
		c.Word(0, uint16(c.D[0])+bank)
		c.D[0] = uint32(uint16(c.D[0])) * 12
		descriptor := uint32(int64(cb.CodeBase+0x212ba) + int64(int16(c.D[0])))
		a[4] = NativeRequesterAddress{Address: descriptor, Code: true}
		x, e := cb.RAM.Read16(int(a[2].Address))
		if e != nil {
			return e
		}
		a[2].Address += 2
		y, e := cb.RAM.Read16(int(a[2].Address))
		if e != nil {
			return e
		}
		a[2].Address += 2
		c.Word(0, x)
		c.Word(1, y)
		source, e := cb.RAM.Read32(int(descriptor))
		if e != nil {
			return e
		}
		a[1] = NativeRequesterAddress{Address: source, Absolute: true}
		height, e := cb.RAM.Read16(int(descriptor) + 6)
		if e != nil {
			return e
		}
		c.Word(2, height)
		c.Word(4, height)
		c.Word(4, -uint16(c.D[4])+16)
		if int16(c.D[4]) > 0 {
			c.Word(1, uint16(c.D[1])+uint16(c.D[4]))
		}
		routine, e := cb.RAM.Read32(int(descriptor) + 8)
		if e != nil {
			return e
		}
		a[4] = NativeRequesterAddress{Address: routine, Code: true}
		saved3, savedA0, savedA2, savedA3 := c.D[3], a[0], a[2], a[3]
		if e = campaignChildSprite(r, cb, a, routine); e != nil {
			return e
		}
		c.D[3] = saved3
		a[0], a[2], a[3] = savedA0, savedA2, savedA3
		c.Word(3, uint16(c.D[3])-1)
	}
	return nil
}
