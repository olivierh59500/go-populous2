package populous2

import (
	"fmt"
	"math/bits"
)

// NativeDebugFrameCallbacks supplies the actual inline JSR format and retained
// CODE storage used by $2ae2. General formatter tokens can read saved address
// registers or memory; those inputs are required only when a token uses them.
type NativeDebugFrameCallbacks struct {
	Code             FollowerCleanupMemory
	CodeBase         uint32
	Frame            *NativeFrameRegisterContext
	FormatAddress    uint32
	AddressRegisters *[7]uint32
	ReadStackLong    func(relativeToSavedRegisters int) (uint32, error)
	ReadAbsolute     func(uint32) (uint8, error)
	Bitmap           func(uint32) (NativeBitmapWindow, error)
}

type NativeDebugFramePlan struct {
	ReturnAddress uint32
	Text          []byte
	Glyphs        int
	Drawn         bool
}

// RenderNativeDebugFrame executes $2ae2/$2c30, including the inline format's
// aligned return address and the original four-pixel, six-row software font.
// All eight data registers survive the routine's MOVEM.L boundary unchanged.
func RenderNativeDebugFrame(cb NativeDebugFrameCallbacks) (NativeDebugFramePlan, error) {
	var plan NativeDebugFramePlan
	if cb.Frame == nil || !winMemoryValid(cb.Code) {
		return plan, fmt.Errorf("native debug frame CODE/register backing missing")
	}
	read := func(a uint32) (uint8, error) {
		if a >= cb.CodeBase {
			if v, err := cb.Code.Read8(int(a - cb.CodeBase)); err == nil {
				return v, nil
			}
		}
		if cb.ReadAbsolute == nil {
			return 0, fmt.Errorf("native debug absolute byte %#x unavailable", a)
		}
		return cb.ReadAbsolute(a)
	}
	stack := func(at int) (uint32, error) {
		if cb.ReadStackLong != nil {
			return cb.ReadStackLong(at)
		}
		if at >= 0 && at < 32 && at%4 == 0 {
			return cb.Frame.D[at/4], nil
		}
		if at >= 32 && at < 60 && at%4 == 0 && cb.AddressRegisters != nil {
			return cb.AddressRegisters[(at-32)/4], nil
		}
		if at == 60 {
			return uint32(uint16(cb.Frame.D[0]))<<16 | cb.FormatAddress>>16, nil
		}
		return 0, fmt.Errorf("native debug saved-stack long at %d unavailable", at)
	}
	formatAt, scratch := cb.FormatAddress, 0x2e3e
	next := func() (uint8, error) { v, err := read(formatAt); formatAt++; return v, err }
	emit := func(v uint8) error {
		if err := cb.Code.Write8(scratch, v); err != nil {
			return err
		}
		scratch++
		return nil
	}
	bytesOut := func(data []byte) error {
		for _, v := range data {
			if err := emit(v); err != nil {
				return err
			}
		}
		return nil
	}
	hex := func(value uint32, count int) error {
		for shift := (count - 1) * 4; shift >= 0; shift -= 4 {
			if err := emit("0123456789ABCDEF"[(value>>shift)&15]); err != nil {
				return err
			}
		}
		return nil
	}
	for {
		token, err := next()
		if err != nil {
			return plan, err
		}
		if token < 128 {
			if err := emit(token); err != nil {
				return plan, err
			}
			if token == 0 {
				break
			}
			continue
		}
		stackAt := int(token&15) * 4
		if token >= 0x88 && token < 0x90 || token >= 0x98 && token < 0xb0 {
			// The first native MOVE.L is overwritten before it is consumed.
			// Read the surviving stack alias, including its negative offsets.
			stackAt -= 32
		}
		value, err := stack(stackAt)
		if err != nil {
			return plan, err
		}
		switch {
		case token < 0x88:
			value = uint32(int32(int16(value))) // $1001a EXT.L
			if err := bytesOut(nativeDebugDecimal(value)); err != nil {
				return plan, err
			}
		case token < 0x90:
			if err := bytesOut(nativeDebugDecimal(value & 0xffff)); err != nil {
				return plan, err
			}
		case token < 0x98:
			if err := bytesOut(nativeDebugDecimal(value)); err != nil {
				return plan, err
			}
		case token < 0xb0:
			if err := bytesOut([]byte{3, byte(value), byte(value >> 16)}); err != nil {
				return plan, err
			}
		case token < 0xc0:
			for {
				v, err := read(value)
				value++
				if err != nil {
					return plan, err
				}
				if v == 0 {
					break
				}
				if err := emit(v); err != nil {
					return plan, err
				}
			}
		case token < 0xd0:
			length, err := next()
			if err != nil {
				return plan, err
			}
			count := int(length)
			if count == 0 {
				count = 65536
			} // DBF after SUBQ.W
			for range count {
				v, err := read(value)
				value++
				if err != nil {
					return plan, err
				}
				v &= 127
				if v < 32 {
					v = '.'
				}
				if err := emit(v); err != nil {
					return plan, err
				}
			}
		case token < 0xe0:
			if err := hex(value&0xffff, 4); err != nil {
				return plan, err
			}
		case token < 0xf0:
			if err := hex(value, 8); err != nil {
				return plan, err
			}
		default:
			length, err := next()
			if err != nil {
				return plan, err
			}
			count := int(length)
			if count == 0 {
				count = 65536
			}
			for range count {
				v, err := read(value)
				value++
				if err != nil {
					return plan, err
				}
				if err := hex(uint32(v), 2); err != nil {
					return plan, err
				}
				if err := emit(' '); err != nil {
					return plan, err
				}
			}
		}
	}
	plan.ReturnAddress = (formatAt + 1) &^ 1
	for at := 0x2e3e; at < scratch; at++ {
		v, err := cb.Code.Read8(at)
		if err != nil {
			return plan, err
		}
		plan.Text = append(plan.Text, v)
	}
	target, err := cb.Code.Read32(0x2e3a)
	if err != nil || target == 0 {
		return plan, err
	}
	if cb.Bitmap == nil {
		return plan, fmt.Errorf("native debug target bitmap %#x unavailable", target)
	}
	window, err := cb.Bitmap(target)
	if err != nil {
		return plan, err
	}
	plan.Drawn = true
	if err := nativeDebugText(cb.Code, window, &plan); err != nil {
		return plan, err
	}
	// The native scratch buffer directly precedes the font controls. A long
	// caller string can overlap them, so report the retained post-render bytes.
	plan.Text = plan.Text[:0]
	for at := 0x2e3e; at < scratch; at++ {
		v, err := cb.Code.Read8(at)
		if err != nil {
			return plan, err
		}
		plan.Text = append(plan.Text, v)
		if v == 0 {
			break
		}
	}
	return plan, nil
}

// $10024 tests the N flag after each wrapping SUB.L, rather than DIVU or
// a signed/unsigned Go decimal conversion. In particular $ffffffff emits
// no digits. The quotient shortcut below preserves that repeated-subtract
// result without an unbounded host loop.
func nativeDebugDecimal(value uint32) []byte {
	if value == 0 {
		return []byte{'0'}
	}
	out := []byte{}
	started := false
	for _, divisor := range []uint32{1000000000, 100000000, 10000000, 1000000, 100000, 10000, 1000, 100, 10, 1} {
		candidate := value - divisor
		count := uint32(0)
		if int32(candidate) >= 0 {
			count = candidate/divisor + 1
			value -= count * divisor
		}
		if count != 0 || started {
			started = true
			out = append(out, byte('0'+count))
		}
	}
	return out
}

func nativeDebugText(code FollowerCleanupMemory, window NativeBitmapWindow, plan *NativeDebugFramePlan) error {
	at := 0x2e3e
	next := func() (uint8, error) { v, err := code.Read8(at); at++; return v, err }
	word := func(a int) (uint16, error) { return code.Read16(a) }
	for {
		ch, err := next()
		if err != nil || ch == 0 {
			return err
		}
		if int8(ch) >= 32 {
			row, err := word(0x2e36)
			if err != nil {
				return err
			}
			col, err := word(0x2e34)
			if err != nil {
				return err
			}
			mode, err := word(0x2e2a)
			if err != nil {
				return err
			}
			color, err := code.Read8(0x2f06)
			if err != nil {
				return err
			}
			fontAt := 0x2f0a + int(ch-32)*6
			dest := window.BitmapOffset + int(int16(uint32(row)*240)) + int(int16(col>>1))
			shift := int((1 - (col & 1)) * 4)
			for y := range 6 {
				glyph, err := code.Read8(fontAt + y)
				if err != nil {
					return err
				}
				mask := bits.RotateLeft8(glyph, shift)
				planes := 4
				if mode != 0 {
					planes = 1
				}
				for plane := range planes {
					pixel := dest + y*40 + plane*8000
					if pixel < 0 || pixel >= len(window.Bytes) {
						return fmt.Errorf("native debug font write outside actual bitmap window: %d", pixel)
					}
					if mode != 0 {
						window.Bytes[pixel] = window.Bytes[pixel]&bits.RotateLeft8(0xf0, shift) | mask
					} else {
						window.Bytes[pixel] &^= mask
						if color&(1<<plane) != 0 {
							window.Bytes[pixel] |= mask
						}
					}
				}
			}
			plan.Glyphs++
			col++
			right, err := word(0x2e30)
			if err != nil {
				return err
			}
			if int16(col) > int16(right) {
				col, err = word(0x2e2c)
				if err != nil {
					return err
				}
				row++
				bottom, err := word(0x2e32)
				if err != nil {
					return err
				}
				if int16(row) > int16(bottom) {
					row--
				} // $2e28 is an RTS, not scrolling.
				if err := code.Write16(0x2e36, row); err != nil {
					return err
				}
			}
			if err := code.Write16(0x2e34, col); err != nil {
				return err
			}
			continue
		}
		switch ch {
		case 1, 2:
			v := uint8(0)
			if ch == 2 {
				v = 255
			}
			if err := code.Write8(0x2f08, v); err != nil {
				return err
			}
		case 3, 4:
			x, err := next()
			if err != nil {
				return err
			}
			col := uint16(int16(int8(x))) - 1
			if err := code.Write16(0x2e34, col); err != nil {
				return err
			}
			if err := code.Write16(0x2e38, col); err != nil {
				return err
			}
			if ch == 3 {
				y, err := next()
				if err != nil {
					return err
				}
				if err := code.Write16(0x2e36, uint16(int16(int8(y-1)))); err != nil {
					return err
				}
			}
		case 5, 6:
			v, err := next()
			if err != nil {
				return err
			}
			if err := code.Write8(0x2f06+int(ch-5), v); err != nil {
				return err
			}
		case 8:
			col, err := word(0x2e34)
			if err != nil {
				return err
			}
			col--
			left, err := word(0x2e2c)
			if err != nil {
				return err
			}
			if int16(col) <= int16(left) {
				col = left
			}
			if err := code.Write16(0x2e34, col); err != nil {
				return err
			}
		case 10, 11, 13:
			row, err := word(0x2e36)
			if err != nil {
				return err
			}
			row++
			if ch == 10 {
				bottom, err := word(0x2e32)
				if err != nil {
					return err
				}
				if int16(row) > int16(bottom) {
					row = bottom
				}
			}
			if err := code.Write16(0x2e36, row); err != nil {
				return err
			}
			if ch == 11 {
				col, err := word(0x2e38)
				if err != nil {
					return err
				}
				if err := code.Write16(0x2e34, col); err != nil {
					return err
				}
			}
		case 14, 15:
			mode := uint16(0)
			if ch == 14 {
				mode = 1
			}
			if err := code.Write16(0x2e2a, mode); err != nil {
				return err
			}
		}
	}
}
