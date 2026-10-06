package populous2

import "fmt"

// RunNativeGameplayEditorPlannedRaise isD80C, including the mutableD80A
// farm mask, original17x17 scratch planner and its actual wall/parcel aliases.
func RunNativeGameplayEditorPlannedRaise(cb NativeGameplayEditorInputCallbacks, a *[7]NativeRequesterAddress) (NativeGameplayEditorInputStep, error) {
	var out NativeGameplayEditorInputStep
	c := cb.Frame
	if c == nil || a == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native editor plannerbackingmissing")
	}
	if e := cb.Code.Write16(0xd80a, uint16(c.D[3])); e != nil {
		return out, e
	}
	a[3] = NativeRequesterAddress{Address: c.AddressBase + 0x76c0}
	c.Word(0, uint16(c.D[0])-8)
	c.Word(1, uint16(c.D[1])-8)
	a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0xd6bc, Code: true}
	c.D[7] = 16
	for y := 0; y < 17; y++ {
		c.D[6] = 16
		for x := 0; x < 17; x++ {
			if _, e := RunNativeStartupWorldFrameHeight(cb.NativeStartupResetFrameCallbacks, a); e != nil {
				return out, e
			}
			if e := cb.RAM.Write8(int(a[0].Address), uint8(c.D[2])); e != nil {
				return out, e
			}
			a[0].Address++
			c.Word(0, uint16(c.D[0])+1)
			c.Word(6, uint16(c.D[6])-1)
		}
		c.Word(0, uint16(c.D[0])-17)
		c.Word(1, uint16(c.D[1])+1)
		c.Word(7, uint16(c.D[7])-1)
	}
	c.Word(0, uint16(c.D[0])+8)
	c.Word(1, uint16(c.D[1])-9)
	a[2] = NativeRequesterAddress{Address: cb.CodeBase + 0xd74c, Code: true}
	saved0, saved1 := uint16(c.D[0]), uint16(c.D[1])
	work := 0
	var plan func() (bool, error)
	plan = func() (ok bool, e error) {
		work++
		if work > 65536 {
			return false, fmt.Errorf("native editorplanrecursion didnotterminate")
		}
		savedD := [3]uint32{c.D[0], c.D[1], c.D[2]}
		savedA2 := a[2]
		defer func() { c.D[0], c.D[1], c.D[2] = savedD[0], savedD[1], savedD[2]; a[2] = savedA2 }()
		value, e := cb.RAM.Read8(int(a[2].Address))
		if e != nil {
			return false, e
		}
		c.Byte(2, value)
		if int8(value) < 0 {
			c.D[4] = 0
			return true, nil
		}
		c.Byte(2, value+1)
		if e = cb.RAM.Write8(int(a[2].Address), uint8(c.D[2])); e != nil {
			return false, e
		}
		if ok, e = gameplayEditorPlanChecks(cb, a); e != nil || !ok {
			return ok, e
		}
		for _, delta := range [][3]int{{-1, -1, 0}, {-17, 0, -1}, {1, 1, 0}, {1, 1, 0}, {17, 0, 1}, {17, 0, 1}, {-1, -1, 0}, {-1, -1, 0}} {
			a[2].Address = uint32(int64(a[2].Address) + int64(delta[0]))
			c.Word(0, uint16(c.D[0])+uint16(delta[1]))
			c.Word(1, uint16(c.D[1])+uint16(delta[2]))
			v, e := cb.RAM.Read8(int(a[2].Address))
			if e != nil {
				return false, e
			}
			c.Byte(3, v)
			if int8(v) >= 0 {
				c.Byte(3, -(uint8(c.D[3]) - uint8(c.D[2])))
				if int8(c.D[3]) > 1 {
					if ok, e = plan(); e != nil || !ok {
						return ok, e
					}
				}
			}
		}
		c.D[4] = 0
		return true, nil
	}
	allowed, e := plan()
	c.D[0], c.D[1] = uint32(int32(int16(saved0))), uint32(int32(int16(saved1)))
	if e != nil {
		return out, e
	}
	if !allowed {
		out.Complete = true
		return out, nil
	}
	return RunNativeGameplayEditorRaise(cb, a)
}

func gameplayEditorPlanChecks(cb NativeGameplayEditorInputCallbacks, a *[7]NativeRequesterAddress) (bool, error) {
	c := cb.Frame
	x, y := uint16(c.D[0]), uint16(c.D[1])
	defer func() { c.D[0], c.D[1] = uint32(int32(int16(x))), uint32(int32(int16(y))) }()
	c.Word(7, uint16(c.D[1])<<6)
	c.Word(7, (uint16(c.D[7])+uint16(c.D[0]))*4)
	a[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe40) + int64(int16(c.D[7])))}
	c.Word(1, uint16(c.D[1])-1)
	c.Word(0, uint16(c.D[0])-1)
	for corner := 0; corner < 4; corner++ {
		if corner == 1 {
			a[1].Address += 4
			c.Word(0, uint16(c.D[0])+1)
		}
		if corner == 2 {
			a[1].Address += 252
			c.Word(1, uint16(c.D[1])+1)
			c.Word(0, uint16(c.D[0])-1)
		}
		if corner == 3 {
			a[1].Address += 4
			c.Word(0, uint16(c.D[0])+1)
		}
		if int16(c.D[0]) < 0 || int16(c.D[1]) < 0 || int16(c.D[0]) >= 64 || int16(c.D[1]) >= 64 {
			continue
		}
		mask, e := cb.Code.Read16(0xd80a)
		if e != nil {
			return false, e
		}
		c.Word(5, mask)
		if mask != 0 {
			tile, e := cb.RAM.Read8(int(a[1].Address) + 1)
			if e != nil {
				return false, e
			}
			if uint8(c.D[5]) == tile {
				c.D[4] = 0xffffffff
				return false, nil
			}
		}
		head, e := cb.RAM.Read16(int(a[1].Address) + 2)
		if e != nil {
			return false, e
		}
		c.Word(5, head)
		seen := map[uint16]bool{}
		for head != 0 {
			if seen[head] {
				return false, fmt.Errorf("native editorplannerwallchaincycle")
			}
			seen[head] = true
			a[4] = NativeRequesterAddress{Address: uint32(int64(a[3].Address) + int64(int16(c.D[5])))}
			kind, e := cb.RAM.Read8(int(a[4].Address))
			if e != nil {
				return false, e
			}
			if kind == 0x1a {
				c.D[4] = 0xffffffff
				return false, nil
			}
			head, e = cb.RAM.Read16(int(a[4].Address) + 2)
			if e != nil {
				return false, e
			}
			c.Word(5, head)
		}
	}
	c.D[4] = 0
	return true, nil
}

// RunNativeStartupWorldFrameHeight is the exactD2B4 leaf, retainingA0 because
// the source helper saves it together withD0/D1/D3. ReturnedD2 is rawsigned.
func RunNativeStartupWorldFrameHeight(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) (int, error) {
	if cb.Frame == nil || a == nil {
		return 0, fmt.Errorf("native editorheightcontextmissing")
	}
	s := startupWorldFrame{cb: cb, c: cb.Frame, a: a, m: nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}, code: nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Code}}}
	saved := cb.Frame.D
	oldA0 := a[0]
	height := s.height()
	cb.Frame.D[0], cb.Frame.D[1], cb.Frame.D[3] = saved[0], saved[1], saved[3]
	a[0] = oldA0
	if s.m.err != nil {
		return height, s.m.err
	}
	return height, s.code.err
}
