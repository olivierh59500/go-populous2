package populous2

import (
	"encoding/binary"
	"fmt"
)

func campaignChildSprite(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress, routine uint32, hardware ...*NativeCampaignBlitterState) error {
	c := cb.Frame
	source, target := a[1].Address, a[0].Address
	x, y, height := int16(c.D[0]), int16(c.D[1]), uint16(c.D[2])
	width := 16
	factor := uint32(2)
	if routine == cb.CodeBase+0xf3a0 {
		width = 32
		factor = 4
	} else if routine != cb.CodeBase+0xf0ee {
		return fmt.Errorf("native campaign sprite routine%x requires its actual body", routine)
	}
	data := make([]byte, int(height)*width/8*5)
	for i := range data {
		v, e := cb.RAM.Read8(int(source) + i)
		if e != nil {
			return e
		}
		data[i] = v
	}
	bitmap, e := cb.Bitmap(target)
	if e != nil {
		return e
	}
	if e = r.primitiveRegisters(routine-cb.CodeBase, c); e != nil {
		return e
	}
	hardwareVisible := x > -int16(width) && x < 320 && y < 200 && int(y)+int(height) > 0
	if hardwareVisible && len(hardware) != 0 && hardware[0] != nil {
		h := hardware[0]
		if e = h.Write32(0xdff040, c.D[0], cb.RAM); e != nil {
			return e
		}
		if e = h.Write32(0xdff044, c.D[1], cb.RAM); e != nil {
			return e
		}
		modSource := uint16(0xfffe)
		modDest := uint16(36)
		if width == 32 {
			modSource = 0xfffe
			modDest = 34
		}
		for _, at := range []uint32{0xdff064, 0xdff062} {
			if e = h.Write16(at, modSource, cb.RAM); e != nil {
				return e
			}
		}
		for _, at := range []uint32{0xdff060, 0xdff066} {
			if e = h.Write16(at, modDest, cb.RAM); e != nil {
				return e
			}
		}
	}

	if e = (&NativePreparedSprite{Width: width, Height: int(height), Planes: data}).Paint(NativePresentationSprite{X: x, Y: y, HalfWidth: int16(width / 2), Height: int16(height), Routine: routine - cb.CodeBase}, bitmap); e != nil {
		return e
	}
	// Retain the primitive pointers across its clipping/early-return branches.

	actualSource := source
	actualTarget := target
	visible := int16(height)
	if y < 0 {
		removed := uint16(-uint16(y))
		visible = int16(height - removed)
		if visible <= 0 {
			return nil
		}
		actualSource += uint32(removed) * factor
	} else {
		if y >= 200 {
			return nil
		}
		actualTarget += uint32(int(y) * 40)
		if int(y)+int(height) >= 200 {
			visible = int16(200 - int(y))
		}
	}
	if x <= -int16(width) || x >= 320 {
		return nil
	}
	if x >= 0 {
		actualTarget += uint32((int(x) &^ 15) / 8)
	}
	a[0] = NativeRequesterAddress{Address: actualTarget + 24000, Chip: true}
	a[1] = NativeRequesterAddress{Address: actualSource, Absolute: true}
	a[2] = NativeRequesterAddress{Address: actualSource + uint32(height)*factor*4, Absolute: true}
	if len(hardware) != 0 && hardware[0] != nil {
		h := hardware[0]
		columns := uint16(2)
		if width == 32 {
			columns = 3
		}
		h.Words[(0xdff058-0xdff000)/2] = uint16(visible)<<6 | columns
		pointers := map[uint32]uint32{0xdff048: actualTarget + 24000 + uint32(visible)*40, 0xdff054: actualTarget + 24000 + uint32(visible)*40, 0xdff050: actualSource + uint32(visible)*factor, 0xdff04c: actualSource + uint32(height)*factor*4 + uint32(visible)*factor}
		for at, value := range pointers {
			h.Words[(at-0xdff000)/2] = uint16(value >> 16)
			h.Words[(at+2-0xdff000)/2] = uint16(value)
		}
	}
	_ = visible
	return nil
}

// DrawNativeCampaignPortrait executes full $baa8 with caller-owned physical
// face parameters and variant bytes. The backing copy uses actual native
// word order, and each primitive retains its real unsaved address outputs.
func DrawNativeCampaignPortrait(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress) error {
	c := cb.Frame
	target, e := cb.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	bitmap, e := cb.Bitmap(target)
	if e != nil {
		return e
	}
	base, e := cb.RAM.Read16(int(a[2].Address))
	if e != nil {
		return e
	}
	a[2].Address += 2
	a[0] = NativeRequesterAddress{Address: target + uint32(int32(int16(base))), Chip: true}
	a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x3ddf8, Code: true}
	c.D[1] = 63
	for row := 0; row < 64; row++ {
		for plane := 0; plane < 4; plane++ {
			for word := 0; word < 3; word++ {
				v, e := cb.RAM.Read16(int(a[1].Address))
				if e != nil {
					return e
				}
				a[1].Address += 2
				c.RestoreWord(2+word, v)
				at := int(int16(base)) + row*40 + plane*8000 + word*2
				if at < 0 || at > len(bitmap)-2 {
					return fmt.Errorf("native portrait backing outside bitmap")
				}
				binary.BigEndian.PutUint16(bitmap[at:], v)
			}
		}
		a[0].Address += 40
		c.Word(1, uint16(c.D[1])-1)
	}
	a[0] = NativeRequesterAddress{Address: target, Chip: true}
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
