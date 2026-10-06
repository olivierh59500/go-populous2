package populous2

import "fmt"

// RunNativeGameplayHUDScan translates2854/2940 and their actual29D2/11180
// leaves. The scan uses raw32-byte FX and52-byte follower records without
// reducing source aliases to typed actors or selecting a guessed nearest one.
func RunNativeGameplayHUDScan(routine int, cb NativeGameplayHUDInputCallbacks, a *[7]NativeRequesterAddress) (NativeGameplayHUDInputStep, error) {
	var out NativeGameplayHUDInputStep
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native HUD scan backing missing")
	}
	c, m := cb.Frame, cb.Memory
	gate, e := m.Read16(0x146)
	if e != nil {
		return out, e
	}
	if gate == 0 {
		c.D[0] = 0
		return NativeGameplayHUDInputStep{Complete: true, FlagsKnown: true, Zero: true}, nil
	}
	profile, e := m.Read16(0xeb42)
	if e != nil {
		return out, e
	}
	c.Word(1, profile)
	start, end, stride := 0x5140, 0x7080, uint16(32)
	cursorAt := 0xeb1a
	if routine == 0x2940 {
		start, end, stride, cursorAt = 0, 0x5140, 52, 0xeb1c
	} else if routine != 0x2854 {
		return out, fmt.Errorf("native HUD scan routine%x unsupported", routine)
	}
	if routine == 0x2854 {
		a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x21066, Code: true}
		kind, e := cb.Code.Read16(0x21066 + int(int16(c.D[0])))
		if e != nil {
			return out, e
		}
		c.Word(2, kind)
		if int16(c.D[2]) < 0 {
			if e = m.Write16(0x142, 0); e != nil {
				return out, e
			}
			c.D[0] = 1
			return NativeGameplayHUDInputStep{Complete: true, FlagsKnown: true}, nil
		}
	}
	c.D[3] = 0
	cursor, e := m.Read16(cursorAt)
	if e != nil {
		return out, e
	}
	c.Word(0, cursor)
	if routine == 0x2854 && cursor == 0 {
		c.Word(0, 0x5140)
		cursor = 0x5140
		if e = m.Write16(cursorAt, cursor); e != nil {
			return out, e
		}
	}
	right, e := m.Read16(0x142)
	if e != nil {
		return out, e
	}
	if routine == 0x2854 && right != 0 {
		c.D[4] = 0
		c.Word(4, uint16(c.D[0]))
		c.Word(4, uint16(c.D[4])-0x5140)
		if e = frameDivide(c, 4, 32); e != nil {
			return out, e
		}
		c.Swap(4)
		if uint16(c.D[4]) != 0 {
			c.Word(0, 0x5140)
			cursor = 0x5140
			if e = m.Write16(cursorAt, cursor); e != nil {
				return out, e
			}
		}
	}
	advance := false
	if routine == 0x2854 && right == 0 || routine == 0x2940 && right != 0 {
		advance = true
	}
	terminated := false
	for visits := 0; visits < 65536; visits++ {
		if advance {
			c.Word(0, uint16(c.D[0])+stride)
			saved, e := m.Read16(cursorAt)
			if e != nil {
				return out, e
			}
			if uint16(c.D[0]) == saved {
				terminated = true
				break
			}
			if int16(c.D[0]) >= int16(end) {
				c.Word(0, uint16(start))
				if uint16(c.D[0]) == saved {
					terminated = true
					break
				}
			}
		}
		advance = true
		a[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0x76c0) + int64(int16(c.D[0])))}
		owner, e := cb.RAM.Read8(int(a[1].Address) + 12)
		if e != nil {
			return out, e
		}
		if uint8(c.D[1]) != owner {
			continue
		}
		if routine == 0x2854 {
			kind, e := cb.RAM.Read8(int(a[1].Address))
			if e != nil {
				return out, e
			}
			if uint8(c.D[2]) != kind {
				continue
			}
		} else {
			flags, e := cb.RAM.Read8(int(a[1].Address) + 13)
			if e != nil {
				return out, e
			}
			if flags&2 == 0 {
				continue
			}
		}
		if e = m.Write16(cursorAt, uint16(c.D[0])); e != nil {
			return out, e
		}
		if routine == 0x2940 {
			old, e := m.Read32(0xf36)
			if e != nil {
				return out, e
			}
			c.D[3] = old
			if old != 0 {
				counter, e := m.Read16(0xf30)
				if e != nil {
					return out, e
				}
				if counter != 0 {
					if e = m.Write32(0xf32, old); e != nil {
						return out, e
					}
				}
				if e = m.Write16(0xf30, 100); e != nil {
					return out, e
				}
			}
			if e = m.Write32(0xf36, a[1].Address); e != nil {
				return out, e
			}
		}
		c.D[3] = 0
		x, e := cb.RAM.Read8(int(a[1].Address) + 6)
		if e != nil {
			return out, e
		}
		c.Byte(3, x)
		c.Word(3, uint16(c.D[3])-4)
		if e = m.Write16(0x5f44, uint16(c.D[3])); e != nil {
			return out, e
		}
		if routine == 0x2940 {
			c.D[3] = 0
		}
		y, e := cb.RAM.Read8(int(a[1].Address) + 8)
		if e != nil {
			return out, e
		}
		c.Byte(3, y)
		c.Word(3, uint16(c.D[3])-4)
		if e = m.Write16(0x5f46, uint16(c.D[3])); e != nil {
			return out, e
		}
		if e = ClampNativeFrameCamera(m, c); e != nil {
			return out, e
		}
		terminated = true
		break
	}
	if !terminated {
		return out, fmt.Errorf("native HUD scan didnotreturntoitsrawcursor")
	}
	if e = m.Write16(0x142, 0); e != nil {
		return out, e
	}
	c.D[0] = 1
	out.Complete, out.FlagsKnown = true, true
	return out, nil
}
