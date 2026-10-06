package populous2

import "fmt"

type NativeGameplayEditorInputCallbacks struct {
	NativeStartupResetFrameCallbacks
	Bitmap func(uint32) ([]byte, error)
}

type NativeGameplayEditorInputStep struct {
	Complete              bool
	Raised, ChangedPixels int
}

// RunNativeGameplayEditorInput executes the actual free-editor leaves.
// D97E compares paired rawparcelheaders, calls genuineD81E/CDCA and preserves
// its D2/A0/A1 saveframe around each raise. It never reconstructs typedterrain.
func RunNativeGameplayEditorInput(routine int, cb NativeGameplayEditorInputCallbacks, a *[7]NativeRequesterAddress) (NativeGameplayEditorInputStep, error) {
	var out NativeGameplayEditorInputStep
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native editor input backingmissing")
	}
	c, m := cb.Frame, cb.Memory
	switch routine {
	case 0xd962:
		a[0] = NativeRequesterAddress{Address: c.AddressBase + 0xf44}
		for i := 0; i < 4096; i++ {
			if e := cb.RAM.Write8(int(a[0].Address), 0); e != nil {
				return out, e
			}
			if e := cb.RAM.Write8(int(a[0].Address)+1, 0); e != nil {
				return out, e
			}
			a[0].Address += 4
		}
	case 0xd97e:
		a[0] = NativeRequesterAddress{Address: c.AddressBase + 0xf44}
		a[1] = NativeRequesterAddress{Address: c.AddressBase + 0x4f40}
		c.Word(2, 0x7ff)
		for i := 0; i < 2048; i++ {
			left, e := cb.RAM.Read8(int(a[0].Address))
			if e != nil {
				return out, e
			}
			right, e := cb.RAM.Read8(int(a[1].Address))
			if e != nil {
				return out, e
			}
			c.Byte(0, left)
			c.Byte(1, right)
			c.Word(0, uint16(c.D[0])&7)
			c.Word(1, uint16(c.D[1])&7)
			if uint16(c.D[1]) != uint16(c.D[0]) {
				pointer := a[0].Address
				if int16(c.D[1]) < int16(c.D[0]) {
					c.Word(1, -uint16(c.D[1]))
					pointer = a[1].Address
				}
				c.D[0] = pointer - (c.AddressBase + 0xf44)
				c.Word(0, uint16(c.D[0])>>2)
				c.Word(1, uint16(c.D[0]))
				c.Word(1, uint16(c.D[1])>>6)
				c.Word(0, uint16(c.D[0])&63)
				saved2, savedA0, savedA1 := c.D[2], a[0], a[1]
				step, e := RunNativeGameplayEditorRaise(cb, a)
				if e != nil {
					return out, e
				}
				out.Raised += step.Raised
				out.ChangedPixels += step.ChangedPixels
				c.D[2], a[0], a[1] = saved2, savedA0, savedA1
			}
			a[0].Address += 4
			a[1].Address -= 4
			c.Word(2, uint16(c.D[2])-1)
		}
	case 0xd80c:
		return RunNativeGameplayEditorPlannedRaise(cb, a)
	case 0xd81e:
		return RunNativeGameplayEditorRaise(cb, a)
	case 0x29d2:
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
	default:
		return out, fmt.Errorf("native editor input routine%x unsupported", routine)
	}
	out.Complete = true
	return out, nil
}

// RunNativeGameplayEditorRaise executesD81E and its literalD838/E196 redraw.
// Rawqueuewrites include cursor/selfaliases and the actual11280 upperbound.
func RunNativeGameplayEditorRaise(cb NativeGameplayEditorInputCallbacks, a *[7]NativeRequesterAddress) (NativeGameplayEditorInputStep, error) {
	var out NativeGameplayEditorInputStep
	c, m := cb.Frame, cb.Memory
	if cb.Bitmap == nil {
		return out, fmt.Errorf("native editor actualbackgroundbitmapmissing")
	}
	if e := m.Write16(0xdd2, 0); e != nil {
		return out, e
	}
	c.D[6], c.D[7] = 0x01000100, 0
	raised, e := RunNativeStartupWorldFrame(0xcdca, cb.NativeStartupResetFrameCallbacks, a)
	if e != nil {
		return out, e
	}
	out.Raised = raised.Raised
	if uint16(c.D[6]) == 256 {
		out.Complete = true
		return out, nil
	}
	target, e := m.Read32(0x22)
	if e != nil {
		return out, e
	}
	a[0] = NativeRequesterAddress{Address: target, Chip: true}
	c.Word(5, uint16(c.D[6]))
	c.Word(0, uint16(c.D[5]))
	c.Word(0, uint16(c.D[0])<<6)
	c.Swap(6)
	c.Word(0, uint16(c.D[0])+uint16(c.D[6]))
	c.Word(0, uint16(c.D[0])*4)
	a[2] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xf45) + int64(int16(c.D[0])))}
	c.Swap(7)
	c.Word(0, uint16(c.D[7]))
	c.Swap(7)
	c.Word(0, uint16(c.D[0])-uint16(c.D[6]))
	c.D[1] = 63
	c.Word(1, (uint16(c.D[1])-uint16(c.D[0]))*4)
	a[3] = NativeRequesterAddress{Address: uint32(int32(int16(c.D[1]))), Absolute: true}
	c.Swap(7)
	for rows := 0; rows < 65536; rows++ {
		c.Word(4, uint16(c.D[6]))
		for cols := 0; cols < 65536; cols++ {
			c.D[2] = 0
			tile, e := cb.RAM.Read8(int(a[2].Address))
			if e != nil {
				return out, e
			}
			c.Byte(2, tile)
			a[2].Address += 4
			a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x33744, Code: true}
			color, e := cb.Code.Read8(0x33744 + int(int16(c.D[2])))
			if e != nil {
				return out, e
			}
			c.Byte(2, color)
			cursor, e := m.Read16(0xeb6e)
			if e != nil {
				return out, e
			}
			a[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xeb70) + int64(int16(cursor)))}
			if int32(a[1].Address) < int32(c.AddressBase+0x11280) {
				c.Word(3, uint16(c.D[4]))
				c.Word(3, uint16(c.D[3])<<6)
				c.Word(3, uint16(c.D[3])+uint16(c.D[5]))
				v := uint16(c.D[2])
				c.Word(2, v>>4|v<<12)
				c.Word(3, uint16(c.D[3])+uint16(c.D[2]))
				v = uint16(c.D[2])
				c.Word(2, v<<4|v>>12)
				if e = cb.RAM.Write16(int(a[1].Address), uint16(c.D[3])); e != nil {
					return out, e
				}
				written, e := m.Read16(0xeb6e)
				if e != nil {
					return out, e
				}
				if e = m.Write16(0xeb6e, written+2); e != nil {
					return out, e
				}
			}
			c.D[0] = 64
			c.Word(0, uint16(c.D[0])+uint16(c.D[4])-uint16(c.D[5])+4)
			c.Word(1, (uint16(c.D[4])+uint16(c.D[5]))>>1)
			c.Word(1, uint16(c.D[1])+4)
			point, e := PlanNativeMapPoint(c)
			if e != nil {
				return out, e
			}
			bitmap, e := cb.Bitmap(a[0].Address)
			if e != nil {
				return out, e
			}
			if e = point.Paint(bitmap); e != nil {
				return out, e
			}
			a[1] = NativeRequesterAddress{Address: uint32(int64(a[0].Address) + int64(point.Offset)), Chip: true}
			out.ChangedPixels++
			c.Word(4, uint16(c.D[4])+1)
			if int16(c.D[4]) > int16(c.D[7]) {
				break
			}
			if cols == 65535 {
				return out, fmt.Errorf("native editor redrawcolumnloop didnotterminate")
			}
		}
		a[2].Address = uint32(int64(a[2].Address) + int64(int32(a[3].Address)))
		c.Swap(7)
		c.Word(5, uint16(c.D[5])+1)
		if int16(c.D[5]) > int16(c.D[7]) {
			out.Complete = true
			return out, nil
		}
		if rows == 65535 {
			return out, fmt.Errorf("native editor redrawrowloop didnotterminate")
		}
		c.Swap(7)
	}
	return out, fmt.Errorf("native editor redraw didnotreturn")
}
