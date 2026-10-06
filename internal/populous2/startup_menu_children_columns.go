package populous2

import "fmt"

// DrawNativeDeityExperienceFrame is completeBB3A/BB8E. Each nibble selects
// source rows from the actual FACES XP art; no generic proportional bar replaces
// its dark/bright row switch or the surviving source address registers.
func DrawNativeDeityExperienceFrame(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return fmt.Errorf("native deity XP backing missing")
	}
	c := cb.Frame
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	ram := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.RAM}}
	a[1] = NativeRequesterAddress{Address: m.long(0x1e) + 0x504, Chip: true}
	a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x3c568, Code: true}
	c.Word(7, m.word(0xeb42))
	c.D[7] = uint32(uint16(c.D[7])) * 314
	a[2] = NativeRequesterAddress{Address: cb.Frame.AddressBase + 0xe76a + uint32(int32(int16(c.D[7]))) + 0x52}
	c.D[7] = 1
	glyph := func() {
		a[4] = a[0]
		a[0].Address += 256
		a[3] = NativeRequesterAddress{Address: a[4].Address + 3072, Code: true}
		c.Word(4, uint16(c.D[4])-33)
		c.D[3] = 31
		bright := false
		for row := 0; row < 32; row++ {
			if !bright {
				c.Word(4, uint16(c.D[4])+1)
				bright = uint16(c.D[4]) == 0
			}
			source := a[3].Address
			if bright {
				source = a[4].Address
			}
			for plane := 0; plane < 4; plane++ {
				ram.putWord(int(a[1].Address)+plane*8000, ram.word(int(source)+plane*2))
			}
			if bright {
				a[4].Address += 8
			} else {
				a[3].Address += 8
				a[4].Address += 8
			}
			a[1].Address += 40
			c.Word(3, uint16(c.D[3])-1)
		}
		a[1].Address -= 1280
	}
	for bank := 0; bank < 2; bank++ {
		c.D[6] = 2
		for xp := 0; xp < 3; xp++ {
			c.D[4], c.D[5] = 0, 0
			c.Byte(4, ram.byte(int(a[2].Address)))
			a[2].Address++
			c.Word(4, uint16(c.D[4])<<4)
			c.Byte(5, uint8(c.D[4]))
			c.Word(4, uint16(c.D[4])^uint16(c.D[5]))
			c.Word(4, uint16(c.D[4])>>7)
			c.Word(5, uint16(c.D[5])>>3)
			glyph()
			a[1].Address += 2
			c.Word(4, uint16(c.D[5]))
			glyph()
			a[1].Address += 4
			c.Word(6, uint16(c.D[6])-1)
		}
		a[1].Address += 1582
		c.Word(7, uint16(c.D[7])-1)
	}
	if m.err != nil {
		return m.err
	}
	return ram.err
}
