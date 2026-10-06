package populous2

import "fmt"

// RunNativeDeityPasswordFrame executes original1047C/10564 with mutable
// payload/letters and the real1055C transpose scratch in CODE. Invalid decode
// leaves the native prefix writes intact; callerB740 owns its rollback.
func RunNativeDeityPasswordFrame(routine int, cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.RAM) {
		return fmt.Errorf("native deity codec backing missing")
	}
	c := cb.Frame
	ram := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.RAM}}
	code := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Code}}
	transpose := func() {
		var input [8]byte
		for i := range input {
			input[i] = ram.byte(int(a[0].Address) + i)
		}
		output := transposeDeityBits(input)
		for i, v := range output {
			code.putByte(0x1055c+i, v)
			ram.putByte(int(a[0].Address)+i, v)
		}
	}
	letters := func() {
		for _, pair := range [][2]int{{1, 4}, {2, 8}, {3, 12}, {6, 9}, {7, 13}, {11, 14}} {
			at := int(a[0].Address)
			c.Byte(0, ram.byte(at+pair[0]))
			ram.putByte(at+pair[0], ram.byte(at+pair[1]))
			ram.putByte(at+pair[1], uint8(c.D[0]))
		}
	}
	divide := func(reg int, by uint16) {
		v := c.D[reg]
		if v/uint32(by) <= 0xffff {
			c.D[reg] = v%uint32(by)<<16 | v/uint32(by)
		}
	}
	xor := func() {
		c.D[0] = 0xec89bb22
		ram.putLong(int(a[0].Address), ram.long(int(a[0].Address))^c.D[0])
		c.D[0] = ^c.D[0]
		ram.putLong(int(a[0].Address)+4, ram.long(int(a[0].Address)+4)^c.D[0])
	}
	switch routine {
	case 0x1047c:
		transpose()
		xor()
		c.D[1] = 3
		for word := 0; word < 4; word++ {
			c.Word(0, ram.word(int(a[0].Address)))
			a[0].Address += 2
			c.D[0] = uint32(uint16(c.D[0])) * 3
			for digit, by := range []uint16{17576, 676, 26, 0} {
				if by != 0 {
					divide(0, by)
				}
				c.Byte(0, uint8(c.D[0])+65)
				ram.putByte(int(a[1].Address), uint8(c.D[0]))
				a[1].Address++
				if digit < 3 {
					c.Swap(0)
					if digit < 2 {
						c.D[0] = uint32(int32(int16(c.D[0])))
					}
				}
			}
			c.Word(1, uint16(c.D[1])-1)
		}
		a[0] = NativeRequesterAddress{Address: a[1].Address - 16, Absolute: true}
		letters()
	case 0x10564:
		letters()
		c.D[2] = 3
		for word := 0; word < 4; word++ {
			c.D[1] = 0
			c.Byte(1, ram.byte(int(a[0].Address)))
			a[0].Address++
			c.Byte(1, uint8(c.D[1])-65)
			c.Word(0, uint16(c.D[1]))
			for digit := 0; digit < 3; digit++ {
				c.D[0] = uint32(uint16(c.D[0])) * 26
				c.Byte(1, ram.byte(int(a[0].Address)))
				a[0].Address++
				c.Byte(1, uint8(c.D[1])-65)
				if digit == 2 {
					c.D[0] += c.D[1]
				} else {
					c.Word(0, uint16(c.D[0])+uint16(c.D[1]))
				}
			}
			divide(0, 3)
			c.Swap(0)
			if uint16(c.D[0]) != 0 {
				c.D[0] = 0xffffffff
				return ram.err
			}
			c.Swap(0)
			ram.putWord(int(a[1].Address), uint16(c.D[0]))
			a[1].Address += 2
			c.Word(2, uint16(c.D[2])-1)
		}
		a[1].Address -= 8
		a[0] = a[1]
		xor()
		transpose()
		c.D[0] = 0
	default:
		return fmt.Errorf("native deity codec routine%x unsupported", routine)
	}
	if ram.err != nil {
		return ram.err
	}
	return code.err
}
