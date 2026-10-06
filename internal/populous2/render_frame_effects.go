package populous2

import (
	"fmt"
)

type NativeActorEffectsCallbacks struct {
	NativeRenderFrameCallbacks
	Cropped           func(NativeCroppedSpriteRequest, []byte) error
	Reinterpreted     func(NativeReinterpretedSpriteRequest, []byte) error
	GridCursorAddress int // Original A4 at the actor call, not reconstructed from a scalar occupant.
}

type NativeActorEffectsPlan struct {
	NativeRenderFramePlan
	Crops []NativeCroppedSpriteRequest
}

func (r *NativeActorRenderRules) crop(descriptor int, cb NativeActorEffectsCallbacks, p *NativeActorEffectsPlan) error {
	c := cb.Frame
	code := r.Frames.code
	if !r.Frames.view.bounds(code, descriptor, 12) {
		return fmt.Errorf("native cropped descriptor outside CODE")
	}
	sourceHeight, err := r.word(descriptor + 6)
	if err != nil {
		return err
	}
	visibleHeight := uint16(c.D[2])
	normal, err := r.Frames.procedure(descriptor + 8)
	if err != nil {
		return err
	}
	half, err := r.word(descriptor + 4)
	if err != nil {
		return err
	}
	routine := uint32(0xf0e8)
	factor := uint16(2)
	if normal == 0xf3a0 {
		routine = 0xf398
		factor = 4
	} else if normal != 0xf0ee {
		return fmt.Errorf("native cropped descriptor procedure unavailable")
	}
	x, y := uint16(c.D[0]), uint16(c.D[1])
	request := NativeCroppedSpriteRequest{Sprite: NativePresentationSprite{Sprite: (descriptor - 0x21626) / 12, X: int16(x), Y: int16(y), HalfWidth: int16(half), Height: int16(visibleHeight), Routine: routine}, SourceHeight: int16(sourceHeight)}
	d0, d1, table := c.D[0], c.D[1], 0
	draw := true
	if int16(y) < 0 {
		removed := -y
		d1 = hudWord(d1, removed)
		if int16(visibleHeight) <= int16(removed) {
			draw = false
		} else {
			d1 = hudWord(d1, removed*factor)
		}
	} else if int16(y) >= 200 {
		draw = false
	} else {
		d1 = hudWord(d1, y+visibleHeight-200)
	}
	if draw {
		if int16(x) < 0 {
			limit := int16(-16)
			if normal == 0xf3a0 {
				limit = -32
			}
			if int16(x) <= limit {
				draw = false
			} else {
				table = 0xf152
			}
		} else {
			d0 = hudWord(d0, x&15)
			column := (x & 0xfff0) >> 3
			if normal == 0xf0ee {
				if column >= 40 {
					draw = false
				} else if column == 38 {
					table = 0xf216
				} else {
					table = 0xf2da
				}
			} else {
				if column > 38 {
					draw = false
				} else if column > 34 {
					table = 0xf216
				} else {
					table = 0xf2da
				}
			}
		}
	}
	r.Frames.Images.blitterRegisters(normal, x, y, &c.D)
	c.Word(7, sourceHeight*factor)
	if draw {
		a := table + int((x&15)*8)
		if !r.Frames.view.bounds(code, a, 8) {
			return fmt.Errorf("native cropped control outside CODE")
		}
		d0, err = r.Frames.long(a)
		if err != nil {
			return err
		}
		d1, err = r.Frames.long(a + 4)
		if err != nil {
			return err
		}
	}
	c.D[0], c.D[1] = d0, d1
	p.Crops = append(p.Crops, request)
	if cb.Cropped == nil {
		p.HardwarePending = true
		return nil
	}
	return cb.Cropped(request, cb.Bitmap)
}

// Actor extends $e45c with the actual image/scenery effect families. Complex
// effect branches report their real source target until their own bodies land.
func (r *NativeActorRenderRules) Actor(at int, cb NativeActorEffectsCallbacks, state *NativeActorRenderState, children NativeActorRenderChildren) (NativeActorEffectsPlan, error) {
	p := NativeActorEffectsPlan{NativeRenderFramePlan: NativeRenderFramePlan{Drawn: true, Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}, Crops: []NativeCroppedSpriteRequest{}}
	if r == nil || cb.Frame == nil || cb.Image == nil || state == nil || !winMemoryValid(cb.Memory) {
		return p, fmt.Errorf("native actor effects backing missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	c.D[2] = 0
	c.Byte(2, m.byte(at))
	branch, err := r.word(0xe46a + int(int16(c.D[2])))
	if err != nil {
		return p, err
	}
	c.Word(2, branch)
	target := 0xe46a + int(int16(c.D[2]))
	return r.actorBranch(at, target, cb, state, children)
}

func (r *NativeActorRenderRules) actorBranch(at, target int, cb NativeActorEffectsCallbacks, state *NativeActorRenderState, children NativeActorRenderChildren) (NativeActorEffectsPlan, error) {
	p := NativeActorEffectsPlan{NativeRenderFramePlan: NativeRenderFramePlan{Drawn: true, Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}, Crops: []NativeCroppedSpriteRequest{}}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	var err error
	if target == 0xe4a8 {
		q, err := r.Follower(at, cb.NativeRenderFrameCallbacks, state, children)
		p.NativeRenderFramePlan = q
		return p, err
	}
	image := func() error { return r.image(cb.NativeRenderFrameCallbacks, &p.NativeRenderFramePlan) }
	switch target {
	case 0xebfc:
		err = r.beam(at, cb, &p)
	case 0xecde:
		err = r.column(at, cb, &p)
	case 0xeda2:
		if m.word(at+40) == 2 {
			c.Word(2, m.word(at+8))
			c.Byte(2, m.byte(at+6)*4)
			c.Byte(2, m.byte(0xf44+int(int16(c.D[2]))+1))
			c.Word(2, uint16(c.D[2])&255)
			c.Word(2, uint16(c.D[2])*2)
			property, e := r.word(0x33312 + int(int16(c.D[2])))
			if e != nil {
				return p, e
			}
			if property&8 != 0 {
				c.Word(2, m.word(at+10))
				v, e := r.word(0x23d1a + int(int16(c.D[2])))
				if e != nil {
					return p, e
				}
				c.Word(2, v*2)
				c.D[2] &= 0xffff
				layer := 0x26956 + int(c.D[2])
				next, e := r.word(layer + 4)
				if e != nil {
					return p, e
				}
				c.Word(2, next)
				if next != 0 {
					c.Word(2, next*2)
					c.D[2] &= 0xffff
					v, e := r.word(0x26956 + int(c.D[2]) + 2)
					if e != nil {
						return p, e
					}
					descriptor := 0x21626 + int(int16(v))
					half, e := r.word(descriptor + 4)
					if e != nil {
						return p, e
					}
					height, e := r.word(descriptor + 6)
					if e != nil {
						return p, e
					}
					c.Word(0, uint16(c.D[0])-half)
					c.Word(2, height)
					c.Word(1, uint16(c.D[1])-height)
					err = r.Frames.descriptor(descriptor, cb.NativeRenderFrameCallbacks, &p.NativeRenderFramePlan)
				}
				break
			}
		}
		c.Word(2, m.word(at+10))
		err = image()
	case 0xeac0:
		c.Word(1, uint16(c.D[1])+8)
		c.Word(2, m.word(at+10))
		err = image()
	case 0xead2:
		c.Word(1, uint16(c.D[1])+12)
		c.Word(2, m.word(at+10))
		err = image()
	case 0xead6:
		c.Word(2, m.word(at+10))
		err = image()
	case 0xebe6:
		c.Word(1, uint16(c.D[1])-80)
		if int16(c.D[1]) < 0 {
			c.Word(1, 0)
		}
		c.Word(2, m.word(at+10))
		err = image()
	case 0xeafa:
		c.Word(2, m.word(at+10))
		c.Word(3, uint16(c.D[2])&3)
		low, high := uint16(0x1e8), uint16(0x204)
		if m.byte(at+12) == 1 {
			low, high = 0x1d4, 0x1e8
		}
		if uint16(c.D[3]) != 0 || int16(c.D[2]) < int16(low) || int16(c.D[2]) >= int16(high) {
			c.Word(2, low)
		}
		c.Word(2, uint16(c.D[2])+4)
		v, e := r.word(0x23d1a + int(int16(c.D[2])))
		if e != nil {
			return p, e
		}
		if int16(v) <= 0 {
			c.Word(2, uint16(c.D[2])+v)
		}
		m.putWord(at+10, uint16(c.D[2]))
		err = image()
	case 0xeb58, 0xeae4, 0xeb6a:
		age := m.byte(at + 1)
		if target == 0xeae4 && age == 0 {
			c.Word(2, m.word(at+10))
			err = image()
			break
		}
		c.Word(1, uint16(c.D[1])+8)
		c.Word(2, m.word(at+10))
		if target == 0xeb58 {
			err = image()
			break
		}
		v, e := r.word(0x23d1a + int(int16(c.D[2])))
		if e != nil {
			return p, e
		}
		c.Word(2, v*2)
		c.D[2] &= 0xffff
		layer := 0x26956 + int(c.D[2])
		if !r.Frames.view.bounds(r.Frames.code, layer, 6) {
			return p, fmt.Errorf("native scenery layer outside CODE")
		}
		layerX, e := r.Frames.byte(layer)
		if e != nil {
			return p, e
		}
		c.Byte(2, layerX)
		c.ExtendWord(2)
		c.Word(0, uint16(c.D[0])+uint16(c.D[2]))
		layerY, e := r.Frames.byte(layer + 1)
		if e != nil {
			return p, e
		}
		c.Byte(2, layerY)
		c.ExtendWord(2)
		c.Word(1, uint16(c.D[1])+uint16(c.D[2]))
		offset, e := r.word(layer + 2)
		if e != nil {
			return p, e
		}
		descriptor := 0x21626 + int(int16(offset))
		half, e := r.word(descriptor + 4)
		if e != nil {
			return p, e
		}
		height, e := r.word(descriptor + 6)
		if e != nil {
			return p, e
		}
		c.Word(0, uint16(c.D[0])-half)
		c.Byte(2, age)
		c.ExtendWord(2)
		if int16(c.D[2]) > 0 {
			c.Word(2, -uint16(c.D[2]))
		}
		before := int16(c.D[2])
		c.Word(2, uint16(c.D[2])+height)
		if int32(before)+int32(int16(height)) <= 0 {
			break
		}
		c.Word(3, height)
		c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
		err = r.crop(descriptor, cb, &p)
	default:
		return p, fmt.Errorf("native actor effect child%#x requires its actual body", target)
	}
	if err != nil {
		return p, err
	}
	return p, m.err
}
