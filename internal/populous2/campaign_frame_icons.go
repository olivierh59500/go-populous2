package populous2

import "fmt"

// DrawNativeCampaignIcons is complete $3ee6. Prepared mask/color bytes come
// from the live physical descriptor source. Its saved geometry and A3/A4
// survive each primitive, while D2/D5-D7 and A0-A2 retain actual draw outputs.
func DrawNativeCampaignIcons(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if r == nil || a == nil || cb.Frame == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.RAM) {
		return fmt.Errorf("native campaign icon backing missing")
	}
	c := cb.Frame
	a[3] = NativeRequesterAddress{Address: a[6].Address + 0x70, Absolute: true}
	a[4] = NativeRequesterAddress{Address: cb.CodeBase + 0x21102, Code: true}
	c.D[4] = 30
	for row := 0; row < 6; row++ {
		c.Word(0, uint16(c.D[4]))
		c.D[1] = 30
		c.D[3] = 5
		for col := 0; col < 6; col++ {
			offset, e := cb.RAM.Read16(int(a[4].Address))
			if e != nil {
				return e
			}
			a[4].Address += 2
			descriptor := uint32(int64(cb.CodeBase+0x214b2) + int64(int16(offset)))
			a[2] = NativeRequesterAddress{Address: descriptor, Code: true}
			flag, e := cb.RAM.Read8(int(a[3].Address))
			if e != nil {
				return e
			}
			a[3].Address++
			if int8(flag) > 0 {
				target, e := cb.Memory.Read32(0x1e)
				if e != nil {
					return e
				}
				a[0] = NativeRequesterAddress{Address: target, Chip: true}
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
				routine, e := cb.RAM.Read32(int(descriptor) + 8)
				if e != nil {
					return e
				}
				a[2] = NativeRequesterAddress{Address: routine, Code: true}
				if routine != cb.CodeBase+0xf3a0 {
					return fmt.Errorf("native campaign icon procedure%x requires its actual body", routine)
				}
				pixels := make([]byte, int(height)*20)
				for i := range pixels {
					v, e := cb.RAM.Read8(int(source) + i)
					if e != nil {
						return e
					}
					pixels[i] = v
				}
				bitmap, e := cb.Bitmap(target)
				if e != nil {
					return e
				}
				x, y := int16(c.D[0]), int16(c.D[1])
				saved := c.D
				savedA3, savedA4 := a[3], a[4]
				if e = r.primitiveRegisters(0xf3a0, c); e != nil {
					return e
				}
				if e = (&NativePreparedSprite{Width: 32, Height: int(height), Planes: pixels}).Paint(NativePresentationSprite{X: x, Y: y, HalfWidth: 16, Height: int16(height), Routine: 0xf3a0}, bitmap); e != nil {
					return e
				}
				// All source world-icon positions are positive, unclipped coordinates.
				// Retain the four-plane primitive's final physical pointers as well.
				if x < 0 || x >= 288 || y < 0 || int(y)+int(height) > 200 {
					return fmt.Errorf("native campaign icon pointer continuation needs clipped source boundary")
				}
				a[0].Address = target + uint32(int(y)*40+(int(x)&^15)/8+24000)
				a[1] = NativeRequesterAddress{Address: source, Absolute: true}
				a[2] = NativeRequesterAddress{Address: source + uint32(height)*16, Absolute: true}
				c.D[0], c.D[1], c.D[3], c.D[4] = saved[0], saved[1], saved[3], saved[4]
				a[3], a[4] = savedA3, savedA4
			}
			c.Word(0, uint16(c.D[0])+16)
			c.Word(1, uint16(c.D[1])+8)
			c.Word(3, uint16(c.D[3])-1)
		}
		c.Word(4, uint16(c.D[4])+32)
	}
	return nil
}
