package populous2

import (
	"fmt"
)

// campaignChildTileHalf executes $bfac's two descriptor words, preserving
// D0's upper-word long source indexing and the distinct zero-word branches.
func campaignChildTileHalf(cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress, hardware *NativeCampaignBlitterState) error {
	c := cb.Frame
	draw := func(offset uint16) error {
		source := a[2].Address + c.D[0]
		a[0] = NativeRequesterAddress{Address: source, Absolute: true}
		a[1] = NativeRequesterAddress{Address: source + 64, Absolute: true}
		if hardware == nil {
			return fmt.Errorf("native campaign inherited blitter backing missing")
		}
		originalTarget := a[6].Address
		for plane := 0; plane < 4; plane++ {
			a[6].Address = originalTarget + uint32(plane)*8000
			for _, at := range []uint32{0xdff048, 0xdff054} {
				if e := hardware.Write32(at, a[6].Address, cb.RAM); e != nil {
					return e
				}
			}
			if plane == 0 {
				if e := hardware.Write32(0xdff04c, a[0].Address, cb.RAM); e != nil {
					return e
				}
			}
			if e := hardware.Write32(0xdff050, a[1].Address, cb.RAM); e != nil {
				return e
			}
			if e := hardware.Write16(0xdff058, 0x0201, cb.RAM); e != nil {
				return e
			}
		}
		a[6].Address = originalTarget

		return nil
	}
	first, e := cb.RAM.Read16(int(a[3].Address))
	if e != nil {
		return e
	}
	a[3].Address += 2
	c.Word(0, first)
	if first != 0 {
		if e = draw(first); e != nil {
			return e
		}
		a[6].Address += 2
	} else {
		a[6].Address += 2
	}
	second, e := cb.RAM.Read16(int(a[3].Address))
	if e != nil {
		return e
	}
	a[3].Address += 2
	c.Word(0, second)
	if second != 0 {
		if e = draw(second); e != nil {
			return e
		}
		a[6].Address += 318
	} else {
		a[6].Address += 318
	}
	return nil
}
