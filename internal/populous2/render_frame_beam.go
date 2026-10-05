package populous2

import (
	"encoding/binary"
	"fmt"
)

// beam is the original$ebfc mixed-record beam renderer. A4 is the actual
// parent's grid cursor; source/captive aliases read retained raw memory.
func (r *NativeActorRenderRules) beam(at int, cb NativeActorEffectsCallbacks, p *NativeActorEffectsPlan) error {
	if cb.GridCursorAddress == 0 {
		return fmt.Errorf("native beam actual A4 grid context missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	other := cleanupRecordAddress(NativeRecordReference(m.word(at + 28)))
	c.D[6] = uint32(m.byte(other + 6))
	c.Word(6, uint16(c.D[6])-m.word(0x5f44))
	c.D[7] = uint32(m.byte(other + 8))
	c.Word(7, uint16(c.D[7])-m.word(0x5f46))
	c.Word(6, uint16(c.D[6])<<3)
	c.Word(7, uint16(c.D[7])<<3)
	c.Word(4, uint16(c.D[7])+uint16(c.D[6]))
	c.Word(6, (uint16(c.D[6])-uint16(c.D[7]))*2)
	c.Word(7, uint16(c.D[4]))
	c.Word(6, uint16(c.D[6])+192)
	c.Word(7, uint16(c.D[7])+72)
	c.D[2] = uint32(m.byte(cb.GridCursorAddress-4)&7) * 8
	c.Word(7, uint16(c.D[7])-uint16(c.D[2]))
	c.Word(7, uint16(c.D[7])-90)
	if int16(c.D[7]) < 0 {
		c.D[7] = 0
	}
	c.Swap(6)
	c.D[7] += c.D[6]
	endpoint := c.D[7]
	c.Word(3, uint16(c.D[7]))
	c.Swap(7)
	c.Word(2, uint16(c.D[7])-uint16(c.D[0]))
	c.Word(3, uint16(c.D[3])-uint16(c.D[1]))
	c.Word(2, uint16(int16(c.D[2])>>2))
	c.Word(3, uint16(int16(c.D[3])>>2))
	c.Swap(2)
	c.Word(2, uint16(c.D[3]))
	c.Word(6, uint16(c.D[0]))
	c.Swap(6)
	c.Word(6, uint16(c.D[1]))
	c.D[1] = 2
	c.Word(0, m.word(at+30))
	c.D[4] = 1
	if m.byte(0xf43)&1 != 0 {
		c.Word(4, -uint16(c.D[4]))
	}
	for {
		c.D[7] = c.D[6]
		c.Word(7, uint16(c.D[7])+uint16(c.D[2]))
		c.Swap(2)
		c.Swap(7)
		c.Word(7, uint16(c.D[7])+uint16(c.D[2]))
		c.Swap(2)
		c.Swap(7)
		c.Word(3, uint16(c.D[0]))
		c.Word(0, uint16(c.D[0])>>1)
		c.Word(3, (uint16(c.D[3])&7)+2)
		c.Word(4, -uint16(c.D[4]))
		if int16(c.D[4]) < 0 {
			c.Word(3, -uint16(c.D[3]))
		}
		c.Swap(7)
		c.Word(7, uint16(c.D[7])+uint16(c.D[3]))
		c.Swap(7)
		c.Word(3, 5)
		// Original MOVEM saves D7 but its mismatched restore writes that
		// saved long into D6, which advances the next segment's origin.
		saved := c.D
		line, err := r.Frames.Line(cb.NativeRenderFrameCallbacks)
		if err != nil {
			return err
		}
		p.Pixels = append(p.Pixels, line.Pixels...)
		for _, reg := range []int{0, 1, 2, 3, 4} {
			c.D[reg] = saved[reg]
		}
		c.D[6] = saved[7]
		c.Word(1, uint16(c.D[1])-1)
		if uint16(c.D[1]) == 0xffff {
			break
		}
	}
	c.D[7] = endpoint
	c.Word(3, 5)
	line, err := r.Frames.Line(cb.NativeRenderFrameCallbacks)
	if err != nil {
		return err
	}
	p.Pixels = append(p.Pixels, line.Pixels...)
	index := 0x18800 - 0x185a8
	if index+2 > len(cb.Image.AudioBank) {
		return fmt.Errorf("native beam cue bank missing")
	}
	v := binary.BigEndian.Uint16(cb.Image.AudioBank[index:])
	binary.BigEndian.PutUint16(cb.Image.AudioBank[index:], v+1)
	return m.err
}
