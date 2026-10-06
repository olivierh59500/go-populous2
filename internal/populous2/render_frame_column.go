package populous2

import (
	"encoding/binary"
	"fmt"
)

func (r *NativeActorRenderRules) column(at int, cb NativeActorEffectsCallbacks, p *NativeActorEffectsPlan) error {
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	x, y := uint16(c.D[0]), uint16(c.D[1])
	c.Word(1, uint16(c.D[1])-75)
	if int16(c.D[1]) < 0 {
		c.Word(1, 0)
	}
	c.Word(2, m.word(at+20))
	if uint16(c.D[2]) != 0 {
		c.Word(2, (uint16(c.D[2])&1)*4+0xd30)
		frame := 0x23d1a + int(int16(c.D[2]))
		cue, e := r.word(frame + 2)
		if e != nil {
			return e
		}
		c.Word(2, cue)
		if cue != 0 {
			if cue&1 != 0 || int(cue)+2 > len(cb.Image.AudioBank) {
				return fmt.Errorf("native column cue alias outside bank")
			}
			v := binary.BigEndian.Uint16(cb.Image.AudioBank[int(cue):])
			binary.BigEndian.PutUint16(cb.Image.AudioBank[int(cue):], v+1)
		}
		v, e := r.word(frame)
		if e != nil {
			return e
		}
		c.Word(2, v)
		c.Word(6, uint16(c.D[0]))
		c.Word(7, uint16(c.D[1]))
		for seen := 0; seen < 256; seen++ {
			c.Word(2, uint16(c.D[2])*2)
			c.D[2] &= 0xffff
			layer := 0x26956 + int(c.D[2])
			if !r.Frames.view.bounds(r.Frames.code, layer, 6) {
				return fmt.Errorf("native column layer outside CODE")
			}
			layerX, e := r.Frames.byte(layer)
			if e != nil {
				return e
			}
			c.Byte(0, layerX)
			c.ExtendWord(0)
			layerY, e := r.Frames.byte(layer + 1)
			if e != nil {
				return e
			}
			c.Byte(1, layerY)
			c.ExtendWord(1)
			offset, e := r.word(layer + 2)
			if e != nil {
				return e
			}
			descriptor := 0x21626 + int(int16(offset))
			half, e := r.word(descriptor + 4)
			if e != nil {
				return e
			}
			height, e := r.word(descriptor + 6)
			if e != nil {
				return e
			}
			c.Word(0, uint16(c.D[0])-half)
			c.Word(2, height)
			c.Word(1, uint16(c.D[1])-height)
			c.Word(0, uint16(c.D[0])+uint16(c.D[6]))
			c.Word(1, uint16(c.D[1])+uint16(c.D[7]))
			c.Word(3, uint16(c.D[1])+uint16(c.D[2])-y)
			draw := true
			if int16(c.D[3]) > 0 {
				if int16(c.D[2]) < int16(c.D[3]) {
					draw = false
				} else {
					c.Word(2, uint16(c.D[3]))
				}
			}
			if draw {
				visible := uint16(c.D[2])
				saved6, saved7 := c.D[6], c.D[7]
				procedure, e := r.Frames.procedure(descriptor + 8)
				if e != nil {
					return e
				}
				s := NativePresentationSprite{Sprite: (descriptor - 0x21626) / 12, X: int16(c.D[0]), Y: int16(c.D[1]), HalfWidth: int16(half), Height: int16(visible), Routine: procedure}
				if e := r.Frames.primitiveRegisters(s.Routine, c); e != nil {
					return e
				}
				p.Sprites = append(p.Sprites, s)
				if cb.Reinterpreted == nil {
					p.HardwarePending = true
				} else if e := cb.Reinterpreted(NativeReinterpretedSpriteRequest{Sprite: s}, cb.Bitmap); e != nil {
					return e
				}
				c.D[6], c.D[7] = saved6, saved7
			}
			next, e := r.word(layer + 4)
			if e != nil {
				return e
			}
			c.Word(2, next)
			if next == 0 {
				break
			}
			if seen == 255 {
				return fmt.Errorf("native column layer chain unbounded")
			}
		}
	}
	c.Word(1, y)
	c.Word(0, x)
	c.Word(2, m.word(at+26))
	if uint16(c.D[2]) != 0 {
		c.Word(1, uint16(c.D[1])+8)
		if e := r.image(cb.NativeRenderFrameCallbacks, &p.NativeRenderFramePlan); e != nil {
			return e
		}
	}
	c.Word(1, y)
	c.Word(0, x)
	c.Word(1, uint16(c.D[1])-75)
	if int16(c.D[1]) < 0 {
		c.Word(1, 0)
	}
	c.Word(2, m.word(at+10))
	if e := r.image(cb.NativeRenderFrameCallbacks, &p.NativeRenderFramePlan); e != nil {
		return e
	}
	return m.err
}
