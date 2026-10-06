package populous2

import "fmt"

// nativeFileBrowserRedraw is the actual standalone $d838 entry. Its inclusive
// signed bounds may cross the grid into real following BSS; queued writes and
// the retained FIFO cursor keep their native aliases.
func nativeFileBrowserRedraw(cb NativeGameplayEditorInputCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || cb.Bitmap == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) || !winMemoryValid(cb.RAM) {
		return fmt.Errorf("native file redraw backing missing")
	}
	c, m := cb.Frame, cb.Memory
	target, e := m.Read32(0x22)
	if e != nil {
		return e
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
				return e
			}
			c.Byte(2, tile)
			a[2].Address += 4
			a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x33744, Code: true}
			color, e := cb.Code.Read8(0x33744 + int(int16(c.D[2])))
			if e != nil {
				return e
			}
			c.Byte(2, color)
			cursor, e := m.Read16(0xeb6e)
			if e != nil {
				return e
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
					return e
				}
				written, e := m.Read16(0xeb6e)
				if e != nil {
					return e
				}
				if e = m.Write16(0xeb6e, written+2); e != nil {
					return e
				}
			}
			c.D[0] = 64
			c.Word(0, uint16(c.D[0])+uint16(c.D[4])-uint16(c.D[5])+4)
			c.Word(1, (uint16(c.D[4])+uint16(c.D[5]))>>1)
			c.Word(1, uint16(c.D[1])+4)
			point, e := PlanNativeMapPoint(c)
			if e != nil {
				return e
			}
			bitmap, e := cb.Bitmap(a[0].Address)
			if e != nil {
				return e
			}
			if e = point.Paint(bitmap); e != nil {
				return e
			}
			a[1] = NativeRequesterAddress{Address: uint32(int64(a[0].Address) + int64(point.Offset)), Chip: true}
			c.Word(4, uint16(c.D[4])+1)
			if int16(c.D[4]) > int16(c.D[7]) {
				break
			}
			if cols == 65535 {
				return fmt.Errorf("native editor redrawcolumnloop didnotterminate")
			}
		}
		a[2].Address = uint32(int64(a[2].Address) + int64(int32(a[3].Address)))
		c.Swap(7)
		c.Word(5, uint16(c.D[5])+1)
		if int16(c.D[5]) > int16(c.D[7]) {
			return nil
		}
		if rows == 65535 {
			return fmt.Errorf("native editor redrawrowloop didnotterminate")
		}
		c.Swap(7)
	}
	return fmt.Errorf("native editor redraw didnotreturn")
}
