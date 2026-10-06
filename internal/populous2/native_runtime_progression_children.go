package populous2

import "fmt"

// RunNativeProgressionChild executes the synchronous source children used by
// $b244. Every operand and returned address belongs to the caller's live RAM.
func RunNativeProgressionChild(routine int, h *NativeRuntimeHost, frame *NativeFrameRegisterContext, a *[7]NativeRequesterAddress) error {
	if h == nil || h.Memory == nil || frame == nil || a == nil || frame.AddressBase != h.Memory.BSSBase {
		return fmt.Errorf("native progression child context missing")
	}
	cb := NativeStartupResetFrameCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase, Frame: frame}
	switch routine {
	case 0xcd22:
		_, e := RunNativeStartupWorldFrame(routine, cb, a)
		return e
	case 0xd8cc:
		return nativeProgressionPreview(cb, a)
	case 0xe0fe, 0xe11a:
		return nativeProgressionPixel(routine, cb, a)
	case 0xb6b6:
		a[0].Address += 0x782
		frame.Word(1, 95)
		for {
			for column := 0; column < 3; column++ {
				for plane := 0; plane < 4; plane++ {
					value, e := cb.RAM.Read8(int(a[1].Address))
					if e != nil {
						return e
					}
					a[1].Address++
					if e = cb.RAM.Write8(int(a[0].Address)+plane*8000+column, value); e != nil {
						return e
					}
				}
			}
			a[0].Address += 40
			frame.Word(1, uint16(frame.D[1])-1)
			if uint16(frame.D[1]) == 0xffff {
				break
			}
		}
		return nil
	case 0xbaee:
		rules, e := DecodeNativeRenderFrameRules(h.Bundle.Executable)
		if e != nil {
			return e
		}
		return DrawNativeProgressionPortrait(&rules, NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cb.Code, Memory: cb.Memory, CodeBase: cb.CodeBase, Frame: frame, Presentation: h.Session.Presentation, Bitmap: h.Bitmap}, RAM: cb.RAM}, a)
	default:
		return fmt.Errorf("native progression child $%x unavailable", routine)
	}
}

func nativeProgressionPixel(routine int, cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	c := cb.Frame
	point := func() error {
		plan, e := PlanNativeMapPoint(c)
		if e != nil {
			return e
		}
		a[1] = NativeRequesterAddress{Address: uint32(int64(a[0].Address) + int64(plan.Offset)), Chip: true}
		for plane := 0; plane < 4; plane++ {
			at := int(a[1].Address) + plane*8000
			v, e := cb.RAM.Read8(at)
			if e != nil {
				return e
			}
			v &= ^byte(1 << plan.Bit)
			if plan.Color&(1<<uint(plane)) != 0 {
				v |= byte(1 << plan.Bit)
			}
			if e = cb.RAM.Write8(at, v); e != nil {
				return e
			}
		}
		return nil
	}
	switch routine {
	case 0xe196:
		return point()
	case 0xe17a:
		if int16(c.D[0]) < 0 || int16(c.D[0]) >= 320 || int16(c.D[1]) < 0 || int16(c.D[1]) >= 200 {
			return nil
		}
		return point()
	case 0xe0fe:
		c.Word(0, uint16(c.D[0])*2)
		saved0, saved1, saved2, savedA0 := c.D[0], c.D[1], c.D[2], a[0]
		if e := point(); e != nil {
			return e
		}
		c.D[0], c.D[1], c.D[2], a[0] = saved0, saved1, saved2, savedA0
		c.Word(0, uint16(c.D[0])+1)
		return point()
	case 0xe11a:
		saved0, saved1, savedA0 := c.D[0], c.D[1], a[0]
		c.Word(0, uint16(c.D[0])*2)
		c.D[1] = uint32(uint16(c.D[1])) * 40
		a[0].Address += uint32(int32(int16(c.D[1])))
		c.Word(1, uint16(c.D[0]))
		c.Word(0, uint16(c.D[0])>>3)
		a[0].Address += uint32(int32(int16(c.D[0])))
		c.Word(1, 7-(uint16(c.D[1])&7))
		for plane := 3; plane >= 0; plane-- {
			v, e := cb.RAM.Read8(int(a[0].Address) + plane*8000)
			if e != nil {
				return e
			}
			c.Byte(0, v)
			c.Word(0, uint16(c.D[0])>>uint(uint16(c.D[1])&63))
			c.Word(0, uint16(c.D[0])&1)
			if plane == 3 {
				c.Word(2, uint16(c.D[0]))
			} else {
				c.Word(2, uint16(c.D[2])|uint16(c.D[0]))
			}
			if plane != 0 {
				c.Word(2, uint16(c.D[2])<<1)
			}
		}
		c.D[0], c.D[1], a[0] = saved0, saved1, savedA0
		return nil
	default:
		return fmt.Errorf("native progression pixel $%x unsupported", routine)
	}
}

func nativeProgressionPreview(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return fmt.Errorf("native startup minimap backing missing")
	}
	c := cb.Frame
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	code := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Code}}
	dx, dy, procedure := code.word(0x33612), code.word(0x33614), code.long(0x33616)
	if procedure != cb.CodeBase+0xe196 && procedure != cb.CodeBase+0xe17a && procedure != cb.CodeBase+0xe0fe {
		return fmt.Errorf("native startup minimap pixel procedure%x has no actual body", procedure)
	}
	a[2] = NativeRequesterAddress{Address: c.AddressBase + 0xf45}
	c.D[7] = 0
	for y := 0; y < 64; y++ {
		c.D[6] = 0
		for x := 0; x < 64; x++ {
			c.D[2] = uint32(m.byte(int(a[2].Address - c.AddressBase)))
			a[2].Address += 4
			a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x33744, Code: true}
			c.Byte(2, code.byte(0x33744+int(int16(c.D[2]))))
			c.D[0] = 64
			c.Word(0, uint16(c.D[0])+uint16(c.D[6])-uint16(c.D[7]))
			c.Word(1, (uint16(c.D[6])+uint16(c.D[7]))>>1)
			a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x33612, Code: true}
			c.Word(0, uint16(c.D[0])+dx)
			c.Word(1, uint16(c.D[1])+dy)
			a[1] = NativeRequesterAddress{Address: procedure, Code: true}
			if procedure != cb.CodeBase+0xe17a || int16(c.D[0]) >= 0 && int16(c.D[0]) < 320 && int16(c.D[1]) >= 0 && int16(c.D[1]) < 200 {
				if err := nativeProgressionPixel(int(procedure-cb.CodeBase), cb, a); err != nil {
					return err
				}

			}
			c.Word(6, uint16(c.D[6])+1)
		}
		c.Word(7, uint16(c.D[7])+1)
	}
	if m.err != nil {
		return m.err
	}
	return code.err
}
