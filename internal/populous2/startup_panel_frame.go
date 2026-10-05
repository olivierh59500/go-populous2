package populous2

import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"go-populous2/internal/amiga"
)

type NativeStartupPanelFrameRules struct{ Render NativeRenderFrameRules }

func DecodeNativeStartupPanelFrameRules(exe *amiga.Executable) (NativeStartupPanelFrameRules, error) {
	r, err := DecodeNativeRenderFrameRules(exe)
	return NativeStartupPanelFrameRules{Render: r}, err
}

type NativeStartupPanelFrameCallbacks struct {
	NativeStartupResetFrameCallbacks
	// Logical is the actual linker-aware CODE view. Physical descriptor
	// source addresses are read through Code/RAM, never through this view.
	Logical   FollowerCleanupMemory
	Image     *NativeImageRenderState
	Bitmap    func(uint32) ([]byte, error)
	Ownership func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error
}

func panelFrameOwnership(cb NativeStartupPanelFrameCallbacks, a *[7]NativeRequesterAddress, released bool) error {
	if cb.Ownership == nil {
		return fmt.Errorf("native panel ownership operation missing")
	}
	d, addresses := cb.Frame.D, *a
	err := cb.Ownership(released, cb.Frame, a)
	cb.Frame.D, *a = d, addresses // Actual E28/E4C outer MOVEM, including A6.
	return err
}

func (r *NativeStartupPanelFrameRules) live(cb NativeStartupPanelFrameCallbacks) (NativeRenderFrameRules, error) {
	if r == nil || !winMemoryValid(cb.Logical) {
		return NativeRenderFrameRules{}, fmt.Errorf("native panel logical CODE missing")
	}
	code := make([]byte, len(r.Render.code))
	for i := range code {
		value, err := cb.Logical.Read8(i)
		if err != nil {
			return NativeRenderFrameRules{}, err
		}
		code[i] = value
	}
	result := r.Render
	result.code, result.Commands.Code, result.Images.code = code, code, code
	return result, nil
}

// blitAddresses follows the actual F0EE/F3A0 address instructions. Clipping
// can return after a row adjustment but before any plane pointer exists.
func panelFrameBlitAddresses(s NativePresentationSprite, a *[7]NativeRequesterAddress) error {
	width, factor := 16, uint16(2)
	if s.Routine == 0xf3a0 {
		width, factor = 32, 4
	} else if s.Routine != 0xf0ee {
		return fmt.Errorf("native panel sprite procedure%x unsupported", s.Routine)
	}
	x, y, height := s.X, s.Y, uint16(s.Height)
	stride := height * factor
	if y < 0 {
		removed := uint16(-uint16(y))
		if int16(height) <= int16(removed) {
			return nil
		}
		a[1].Address += uint32(int32(int16(removed * factor)))
	} else {
		if y >= 200 {
			return nil
		}
		a[0].Address += uint32(int32(int16(uint16(y) * 40)))
	}
	if x < 0 {
		if x <= int16(-width) {
			return nil
		}
		a[0].Address -= 2
		if width == 32 && x < -16 {
			a[1].Address += 2
		}
	} else {
		column := uint16(x) & 0xfff0
		column >>= 3
		if width == 16 {
			if column >= 40 {
				return nil
			}
			a[0].Address += uint32(int32(int16(column)))
		} else {
			a[0].Address += uint32(int32(int16(column)))
			if column > 38 {
				return nil
			}
		}
	}
	a[2] = NativeRequesterAddress{Address: a[1].Address + uint32(int32(int16(stride))*4), Absolute: true}
	a[0].Address += 24000
	return nil
}

func panelFramePaint(cb NativeStartupPanelFrameCallbacks, s NativePresentationSprite, descriptor int, target uint32) error {
	source, err := cb.Code.Read32(descriptor)
	if err != nil {
		return err
	}
	width := int(s.HalfWidth) * 2
	length := width / 8 * 5 * int(s.Height)
	if width != 16 && width != 32 || length <= 0 {
		return fmt.Errorf("native prepared panel sprite geometry unavailable")
	}
	planes := make([]byte, length)
	for i := range planes {
		v, err := cb.RAM.Read8(int(source) + i)
		if err != nil {
			return err
		}
		planes[i] = v
	}
	bitmap, err := cb.Bitmap(target)
	if err != nil {
		return err
	}
	return (NativePreparedSprite{Width: width, Height: int(s.Height), Planes: planes}).Paint(s, bitmap)
}

// RestorePanel follows complete 1DA0 and its real 1F5E tail. Prepared ICONS
// and arrow planes come from physical RAM; the scalar panel API is unchanged.
func (r *NativeStartupPanelFrameRules) RestorePanel(cb NativeStartupPanelFrameCallbacks, a *[7]NativeRequesterAddress) (NativeProfilePanelFramePlan, error) {
	var p NativeProfilePanelFramePlan
	if cb.Frame == nil || a == nil || cb.Image == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return p, fmt.Errorf("native startup panel backing missing")
	}
	live, err := r.live(cb)
	if err != nil {
		return p, err
	}
	c := cb.Frame
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	c.Word(0, m.word(0xeb42))
	c.D[0] = uint32(uint16(c.D[0])) * 314
	a[5] = NativeRequesterAddress{Address: c.AddressBase + 0xe76a + uint32(int32(int16(c.D[0])))}
	c.Word(0, m.word(0xf3a))
	c.D[0] = uint32(uint16(c.D[0])) * 3
	a[5].Address += 0x70 + uint32(int32(int16(c.D[0])))
	c.Word(0, uint16(c.D[0])*2)
	a[3] = NativeRequesterAddress{Address: cb.CodeBase + 0x21102 + uint32(int32(int16(c.D[0]))), Code: true}
	a[4] = NativeRequesterAddress{Address: cb.CodeBase + 0x2114a, Code: true}
	c.D[3] = 4
	for i := 0; i < 5; i++ {
		at := int(a[4].Address - cb.CodeBase)
		c.Word(0, binary.BigEndian.Uint16(live.code[at:]))
		c.Word(1, binary.BigEndian.Uint16(live.code[at+2:]))
		a[4].Address += 4
		a[2] = NativeRequesterAddress{Address: cb.CodeBase + 0x214b2, Code: true}
		at = int(a[3].Address - cb.CodeBase)
		c.Word(2, binary.BigEndian.Uint16(live.code[at:]))
		a[3].Address += 2
		flag, err := cb.RAM.Read8(int(a[5].Address))
		if err != nil {
			return p, err
		}
		a[5].Address++
		if int8(flag) > 0 {
			a[2].Address += uint32(int32(int16(c.D[2])))
		}
		descriptor := int(a[2].Address - cb.CodeBase)
		target := m.long(0x22)
		a[0] = NativeRequesterAddress{Address: target, Chip: true}
		source, err := cb.Code.Read32(descriptor)
		if err != nil {
			return p, err
		}
		a[1] = NativeRequesterAddress{Address: source, Absolute: true}
		c.Word(2, binary.BigEndian.Uint16(live.code[descriptor+6:]))
		procedure := binary.BigEndian.Uint32(live.code[descriptor+8:])
		a[2] = NativeRequesterAddress{Address: cb.CodeBase + procedure, Code: true}
		sprite := NativePresentationSprite{X: int16(c.D[0]), Y: int16(c.D[1]), HalfWidth: 16, Height: int16(c.D[2]), Routine: procedure}
		p.Descriptors = append(p.Descriptors, descriptor)
		p.Sprites = append(p.Sprites, sprite)
		saved3, savedA := c.D[3], [3]NativeRequesterAddress{a[3], a[4], a[5]}
		if err = panelFrameOwnership(cb, a, true); err != nil {
			return p, err
		}
		if err = live.primitiveRegisters(procedure, c); err != nil {
			return p, err
		}
		if err = panelFrameBlitAddresses(sprite, a); err != nil {
			return p, err
		}
		if err = panelFramePaint(cb, sprite, descriptor, target); err != nil {
			return p, err
		}
		if err = panelFrameOwnership(cb, a, false); err != nil {
			return p, err
		}
		c.D[3], a[3], a[4], a[5] = saved3, savedA[0], savedA[1], savedA[2]
		c.Word(3, uint16(c.D[3])-1)
	}
	p.HUD, err = startupPanelHUD(&live, cb, a)
	if err == nil {
		err = m.err
	}
	return p, err
}

func (r *NativeStartupPanelFrameRules) HUD(cb NativeStartupPanelFrameCallbacks, a *[7]NativeRequesterAddress) (NativeRenderFramePlan, error) {
	live, err := r.live(cb)
	if err != nil {
		return NativeRenderFramePlan{}, err
	}
	return startupPanelHUD(&live, cb, a)
}

func startupPanelPixel(cb NativeStartupPanelFrameCallbacks, a *[7]NativeRequesterAddress, p *NativeRenderFramePlan, checked bool) error {
	c := cb.Frame
	x, y := int16(c.D[0]), int16(c.D[1])
	if checked && (x < 0 || x >= 320 || y < 0 || y >= 200) {
		return nil
	}
	point, err := PlanNativeMapPoint(c)
	if err != nil {
		return err
	}
	a[1] = NativeRequesterAddress{Address: uint32(int64(a[0].Address) + int64(point.Offset)), Chip: true}
	bitmap, err := cb.Bitmap(a[0].Address)
	if err != nil {
		return err
	}
	p.Pixels = append(p.Pixels, NativeHUDPixel{X: x, Y: y, Color: uint16(point.Color)})
	return point.Paint(bitmap)
}

func startupPanelImage(r *NativeRenderFrameRules, cb NativeStartupPanelFrameCallbacks, a *[7]NativeRequesterAddress, p *NativeRenderFramePlan) error {
	c := cb.Frame
	frame := uint16(c.D[2])
	image := binary.BigEndian.Uint16(r.code[0x23d1a+int(frame):])
	for i := range cb.Image.AudioBank {
		v, err := cb.Code.Read8(0x185a8 + i)
		if err != nil {
			return err
		}
		cb.Image.AudioBank[i] = v
	}
	last, err := cb.Code.Read16(0xeee0)
	if err != nil {
		return err
	}
	cb.Image.LastY = last
	sprites, err := r.Images.DrawImage(cb.Image, &c.D)
	if err != nil {
		return err
	}
	for i, v := range cb.Image.AudioBank {
		if err = cb.Code.Write8(0x185a8+i, v); err != nil {
			return err
		}
	}
	if err = cb.Code.Write16(0xeee0, cb.Image.LastY); err != nil {
		return err
	}
	for _, s := range sprites {
		layer := 0x26956 + int(uint16(image*2))
		descriptor := 0x21626 + int(binary.BigEndian.Uint16(r.code[layer+2:]))
		source, err := cb.Code.Read32(descriptor)
		if err != nil {
			return err
		}
		a[2] = NativeRequesterAddress{Address: cb.CodeBase + uint32(layer+4), Code: true}
		a[1] = NativeRequesterAddress{Address: source, Absolute: true}
		a[4] = NativeRequesterAddress{Address: cb.CodeBase + s.Routine, Code: true}
		target, err := cb.Memory.Read32(0x1e)
		if err != nil {
			return err
		}
		a[0] = NativeRequesterAddress{Address: target, Chip: true}
		if err = panelFrameBlitAddresses(s, a); err != nil {
			return err
		}
		if err = panelFramePaint(cb, s, descriptor, target); err != nil {
			return err
		}
		p.Sprites = append(p.Sprites, s)
		image = binary.BigEndian.Uint16(r.code[layer+4:])
	}
	return nil
}

func startupPanelHUD(r *NativeRenderFrameRules, cb NativeStartupPanelFrameCallbacks, a *[7]NativeRequesterAddress) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if cb.Frame == nil {
		return p, fmt.Errorf("native rendering frame missing")
	}
	if r == nil || !winMemoryValid(cb.Memory) || cb.Image == nil || a == nil || cb.Bitmap == nil {
		return p, fmt.Errorf("native HUD raw backing/image/bitmap missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	p.Drawn = true
	c.Word(0, m.word(0xeb42))
	c.D[0] = uint32(uint16(c.D[0])) * 314
	god := 0xe76a + int(int16(c.D[0]))
	a[4] = NativeRequesterAddress{Address: c.AddressBase + uint32(god+0x70)}
	a[2] = NativeRequesterAddress{Address: cb.CodeBase + 0x2115e, Code: true}
	c.D[3] = m.long(god) >> 2
	c.D[7] = 0
	price := func() error {
		context := c.CommandContext()
		err := r.Commands.Cost(&context, cb.Memory)
		c.SetCommandContext(context)
		return err
	}
	for pointer, slot := 0x2115e, 0; ; pointer, slot = pointer+6, slot+1 {
		v, err := r.word(pointer)
		if err != nil {
			return p, err
		}
		c.Word(2, v)
		a[2].Address += 2
		if int16(v) < 0 {
			break
		}
		c.Word(0, uint16(c.D[7]))
		c.Word(1, m.word(0xeb42))
		if err := price(); err != nil {
			return p, err
		}
		c.Word(5, uint16(c.D[0]))
		c.Word(0, uint16(c.D[2]))
		y, err := r.word(pointer + 2)
		if err != nil {
			return p, err
		}
		color, err := r.word(pointer + 4)
		if err != nil {
			return p, err
		}
		c.Word(1, y)
		c.Word(2, color)
		a[2].Address += 4
		a[4].Address++
		if int8(m.byte(god+0x70+slot)) > 0 {
			c.D[4] = c.D[3]
			if err := frameDivide(c, 4, uint16(c.D[5])); err != nil {
				return p, err
			}
			if int16(c.D[4]) > 0 {
				if int16(c.D[4]) > 4 {
					c.Word(4, 4)
				}
				c.Word(4, uint16(c.D[4])-1)
				for {
					saved := c.D
					target, e := m.m.Read32(0x1e)
					if e != nil {
						return p, e
					}
					a[0] = NativeRequesterAddress{Address: target, Chip: true}
					if err := startupPanelPixel(cb, a, &p, false); err != nil {
						return p, err
					}
					// The original MOVEM restores D0-D4/D7, while this
					// pixel child assigns no other data registers.
					c.D = saved
					c.Word(1, uint16(c.D[1])-1)
					c.Word(4, uint16(c.D[4])-1)
					if uint16(c.D[4]) == 0xffff {
						break
					}
				}
			}
		}
		c.Word(7, uint16(c.D[7])+1)
	}
	c.Word(4, m.word(0xf3a))
	c.D[4] = uint32(uint16(c.D[4])) * 6
	c.Word(0, m.word(0xeb42))
	c.D[0] = uint32(uint16(c.D[0])) * 314
	god = 0xe76a + int(int16(c.D[0]))
	a[1] = NativeRequesterAddress{Address: c.AddressBase + uint32(god)}
	c.D[6], c.D[5], c.D[3] = 0, 0, 0
	c.Word(4, uint16(c.D[4])>>1)
	for {
		c.Word(1, m.word(0xeb42))
		c.Word(0, uint16(c.D[4]))
		if err := price(); err != nil {
			return p, err
		}
		c.D[2] = c.D[0]
		if int8(m.byte(god+0x70+int(int16(c.D[4])))) > 0 {
			c.D[2] *= 4
			if int32(c.D[2]) >= int32(m.long(god)) {
				break
			}
			c.D[3] = c.D[2]
			c.Word(6, uint16(c.D[5]))
		}
		c.Word(4, uint16(c.D[4])+1)
		c.Word(5, uint16(c.D[5])+1)
		if uint16(c.D[5]) == 6 {
			break
		}
	}
	if c.D[3] == 0 {
		c.D[0], c.D[6] = 0, 0
	} else {
		c.Word(0, uint16(c.D[6]))
		c.Word(0, uint16(c.D[0])<<4)
		c.Word(6, uint16(c.D[6])<<3)
		c.D[2] = m.long(god) >> 2
		c.D[3] >>= 2
		if err := frameDivide(c, 2, uint16(c.D[3])); err != nil {
			return p, err
		}
		if uint16(c.D[2]) > 7 {
			c.Word(2, 7)
		}
		c.Word(1, uint16(c.D[6])+uint16(c.D[2]))
		c.Word(0, uint16(c.D[0])+uint16(c.D[2])*2)
	}
	c.Word(0, uint16(c.D[0])+27)
	c.Word(1, uint16(c.D[1])+166)
	c.Word(2, 0x195c)
	savedA1 := a[1]
	if err := panelFrameOwnership(cb, a, true); err != nil {
		return p, err
	}
	if err := startupPanelImage(r, cb, a, &p); err != nil {
		return p, err
	}
	if err := panelFrameOwnership(cb, a, false); err != nil {
		return p, err
	}
	a[1] = savedA1
	c.D[0] = m.long(god)
	if err := frameDivide(c, 0, 5000); err != nil {
		return p, err
	}
	if int16(c.D[0]) > 0 {
		if int16(c.D[0]) > 35 {
			c.Word(0, 35)
		}
		span := uint16(c.D[0])
		// These are $e056's actual x-major line assignments for its
		// source endpoints (25,164)..(25+2n,164+n), not a fitted D result.
		c.D[6] = 0x001900a4
		c.D[7] = c.D[6]
		c.Word(7, uint16(c.D[7])+span)
		c.Swap(7)
		c.Word(7, uint16(c.D[7])+span*2)
		c.Swap(7)
		c.Word(3, 8)
		c.Word(3, uint16(c.D[3])&15)
		c.D[7] -= c.D[6]
		c.Swap(6)
		c.Swap(7)
		c.D[5] = 0x8000
		c.Word(4, uint16(c.D[6]))
		c.Word(6, 0)
		for {
			c.Word(0, uint16(c.D[4]))
			c.Swap(6)
			c.Word(1, uint16(c.D[6]))
			c.Word(2, uint16(c.D[3]))
			target, e := m.m.Read32(0x1e)
			if e != nil {
				return p, e
			}
			a[0] = NativeRequesterAddress{Address: target, Chip: true}
			if err := startupPanelPixel(cb, a, &p, true); err != nil {
				return p, err
			}
			c.Swap(6)
			c.D[6] += c.D[5]
			c.Word(4, uint16(c.D[4])+1)
			c.Word(7, uint16(c.D[7])-1)
			if uint16(c.D[7]) == 0 {
				break
			}
		}
	}
	target, e := m.m.Read32(0x1e)
	if e != nil {
		return p, e
	}
	bitmap, e := cb.Bitmap(target)
	if e != nil {
		return p, e
	}
	a[4] = NativeRequesterAddress{Address: target + 31, Chip: true}
	a[5] = NativeRequesterAddress{Address: cb.CodeBase + 0x33a88, Code: true}
	a[2] = NativeRequesterAddress{Address: c.AddressBase + 0xe8a4}
	a[3] = NativeRequesterAddress{Address: cb.CodeBase + 0x219e, Code: true}
	c.Word(0, m.word(0x138))
	c.Word(0, uint16(c.D[0])+m.word(0x13a)+m.word(0xf42))
	c.Word(4, uint16(c.D[0]))
	for side := 0; side < 2; side++ {
		c.D[3] = m.long(0xe8a4 + side*314 + 4)
		if c.D[3] == 0 {
			a[2].Address += 314
			a[5].Address += 240
			a[3] = NativeRequesterAddress{Address: cb.CodeBase + 0x22c6, Code: true}
			continue
		}
		if err := frameDivide(c, 3, 2048); err != nil {
			return p, err
		}
		if int16(c.D[3]) > 73 {
			c.Word(3, 73)
		}
		if int16(c.D[3]) <= 0 {
			a[2].Address += 314
			a[5].Address += 240
			a[3] = NativeRequesterAddress{Address: cb.CodeBase + 0x22c6, Code: true}
			continue
		}
		pointer := 0x219e + side*0x128
		for {
			c.Word(4, bits.RotateLeft16(uint16(c.D[4]), 1))
			point, err := r.word(pointer)
			if err != nil {
				return p, err
			}
			row, err := r.word(pointer + 2)
			if err != nil {
				return p, err
			}
			pointer += 4
			a[3].Address = cb.CodeBase + uint32(pointer)
			c.Word(0, point)
			c.Word(1, uint16(c.D[0])>>3)
			c.Word(1, uint16(c.D[1])+row)
			at := 31 + int(int16(c.D[1]))
			a[0] = NativeRequesterAddress{Address: uint32(int64(target) + int64(at)), Chip: true}
			c.D[1] = 2
			c.Word(0, uint16(c.D[0])&7)
			variant, err := r.word(0x218e + int(int16(c.D[0])))
			if err != nil {
				return p, err
			}
			c.Word(0, uint16(c.D[2])+uint16(c.D[4]))
			c.Word(0, uint16(c.D[0])&6)
			phase, err := r.word(0x2196 + int(int16(c.D[0])))
			if err != nil {
				return p, err
			}
			source := 0x33a88 + side*240 + int(int16(variant)) + int(int16(phase))
			a[1] = NativeRequesterAddress{Address: cb.CodeBase + uint32(source), Code: true}
			for {
				if source < 0 || source+5 > len(r.code) || at < 0 || at >= 8000 {
					return p, fmt.Errorf("native population stencil outside backing")
				}
				c.Byte(0, r.code[source])
				source++
				for plane := range 4 {
					address := at + plane*8000
					c.Byte(2, bitmap[address])
					c.Byte(2, uint8(c.D[2])&uint8(c.D[0]))
					c.Byte(2, uint8(c.D[2])|r.code[source])
					source++
					bitmap[address] = uint8(c.D[2])
				}
				at += 40
				a[0].Address += 40
				a[1].Address += 5
				c.Word(1, uint16(c.D[1])-1)
				if uint16(c.D[1]) == 0xffff {
					break
				}
			}
			c.Word(3, uint16(c.D[3])-1)
			if uint16(c.D[3]) == 0xffff {
				break
			}
		}
		a[2].Address += 314
		a[5].Address += 240
		a[3] = NativeRequesterAddress{Address: cb.CodeBase + 0x22c6, Code: true}
	}
	return p, m.err
}
