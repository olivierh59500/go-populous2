package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeActorRenderState struct {
	TownHitHeight uint16 // Mutable CODE$e8ce.
}

type NativeActorRenderChildren struct {
	TownInfo func(int, *NativeFrameRegisterContext) error // Genuine$314a caller boundary.
	// TownInfoAdvance is the same original child with a retained modal wait.
	// It is consumed only by AdvanceActor; synchronous APIs retain TownInfo.
	TownInfoAdvance func(int, *NativeFrameRegisterContext) (bool, error)
	// RefreshTargets resolves the physical screen after a completed modal
	// reload/swap, without changing caller registers or repeating projection.
	RefreshTargets func(*NativeActorEffectsCallbacks) error
}

type NativeActorRenderRules struct{ Frames NativeRenderFrameRules }

func DecodeNativeActorRenderRules(exe *amiga.Executable) (NativeActorRenderRules, error) {
	frames, err := DecodeNativeRenderFrameRules(exe)
	return NativeActorRenderRules{Frames: frames}, err
}

func (r *NativeActorRenderRules) word(a int) (uint16, error) { return r.Frames.word(a) }

func (r *NativeActorRenderRules) image(cb NativeRenderFrameCallbacks, p *NativeRenderFramePlan) error {
	sprites, err := r.Frames.Images.DrawImage(cb.Image, &cb.Frame.D)
	if err != nil {
		return err
	}
	return renderFrameSprites(cb, p, sprites)
}

// Angle is the actual$f71c register lookup, including signed-word aliases and
// its asymmetric register-count shifts. D2/D3 are saved as complete longs.
func (r *NativeActorRenderRules) Angle(c *NativeFrameRegisterContext) error {
	if r == nil || c == nil {
		return fmt.Errorf("native actor angle frame missing")
	}
	saved2, saved3 := c.D[2], c.D[3]
	negativeX, negativeY := int16(c.D[0]) < 0, int16(c.D[1]) < 0
	if negativeX {
		c.Word(0, -uint16(c.D[0]))
	}
	if negativeY {
		c.Word(1, -uint16(c.D[1]))
	}
	readByte := func(a int) (uint8, error) {
		if a < 0 || a >= len(r.Frames.code) {
			return 0, fmt.Errorf("native angle byte outside CODE")
		}
		return r.Frames.code[a], nil
	}
	if int16(c.D[1]) <= int16(c.D[0]) {
		c.Word(2, uint16(c.D[0])>>5)
		c.D[3] = 0
		v, err := readByte(0xf81a + int(int16(c.D[2])))
		if err != nil {
			return err
		}
		c.Byte(3, v)
		c.Word(0, uint16(c.D[0])>>uint(c.D[3]&63))
		c.Word(1, uint16(c.D[1])>>uint(c.D[2]&63))
	} else {
		c.Word(3, uint16(c.D[1])>>5)
		c.D[2] = 0
		v, err := readByte(0xf81a + int(int16(c.D[3])))
		if err != nil {
			return err
		}
		c.Byte(2, v)
		c.Word(0, uint16(c.D[0])>>uint(c.D[3]&63))
		c.Word(1, uint16(c.D[1])>>uint(c.D[3]&63))
	}
	c.Word(2, (uint16(c.D[0])<<5)+uint16(c.D[1]))
	v, err := readByte(0xfc1a + int(int16(c.D[2])))
	if err != nil {
		return err
	}
	c.Byte(0, v)
	if negativeX && negativeY {
		c.Word(0, uint16(c.D[0])+128)
	} else if negativeX {
		c.Word(0, -uint16(c.D[0]))
		c.Word(0, uint16(c.D[0])&255)
	} else if negativeY {
		c.Word(0, -uint16(c.D[0]))
		c.Word(0, uint16(c.D[0])+128)
	}
	c.D[2], c.D[3] = saved2, saved3
	c.Word(0, uint16(c.D[0])&255)
	return nil
}

// overlay is the full-register saved$eee2/$ef22 descriptor child.
func (r *NativeActorRenderRules) overlay(at int, selected bool, cb NativeRenderFrameCallbacks, p *NativeRenderFramePlan) error {
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	saved := c.D
	descriptor := 0x21c26
	if selected {
		if int16(m.word(0xeb18)) < 0 {
			return m.err
		}
		c.Word(1, cb.Image.LastY-12)
	} else {
		c.Word(1, cb.Image.LastY-8)
		c.Word(0, uint16(c.D[0])-4)
		descriptor = 0x21a82
		if m.byte(at+12) != 1 {
			descriptor = 0x21a8e
		}
	}
	if err := r.Frames.descriptor(descriptor, cb, p); err != nil {
		return err
	}
	c.D = saved
	return m.err
}

func (r *NativeActorRenderRules) markers(at int, cb NativeRenderFrameCallbacks, p *NativeRenderFramePlan) error {
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	if m.long(0xf36) == cb.Frame.AddressBase+uint32(at) {
		if err := r.overlay(at, true, cb, p); err != nil {
			return err
		}
	}
	for _, bit := range []uint8{1, 2} {
		if m.byte(at+13)&bit != 0 {
			if err := r.overlay(at, false, cb, p); err != nil {
				return err
			}
		}
	}
	return m.err
}

// Follower executes the actual kind2..18 state draw branch. Town$e72e is a
// separate method below; unknown state dispatches remain explicit errors.
func (r *NativeActorRenderRules) Follower(at int, cb NativeRenderFrameCallbacks, state *NativeActorRenderState, children NativeActorRenderChildren) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}, Drawn: true}
	if r == nil || cb.Frame == nil || cb.Image == nil || state == nil || !winMemoryValid(cb.Memory) {
		return p, fmt.Errorf("native follower renderer backing missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	c.D[2] = 0
	c.Byte(2, m.byte(at))
	branch, err := r.word(0xe46a + int(int16(c.D[2])))
	if err != nil {
		return p, err
	}
	c.Word(2, branch)
	if uint16(c.D[2])+0xe46a != 0xe4a8 {
		return p, fmt.Errorf("native actor kind requires original FX/scenery child")
	}
	c.D[2] = 0
	c.Byte(2, m.byte(at+22))
	branch, err = r.word(0xe4ba + int(int16(c.D[2])))
	if err != nil {
		return p, err
	}
	c.Word(2, branch)
	target := 0xe4ba + int(int16(c.D[2]))
	return r.followerBranch(at, target, cb, state, children)
}

func (r *NativeActorRenderRules) followerBranch(at, target int, cb NativeRenderFrameCallbacks, state *NativeActorRenderState, children NativeActorRenderChildren) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}, Drawn: true}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	var err error
	if target == 0xe72e {
		return r.Town(at, cb, state, children)
	}
	if target == 0xe506 {
		if m.byte(at) == 4 {
			return r.Town(at, cb, state, children)
		}
		return p, m.err
	}
	if target == 0xe8d0 {
		saved0, saved1 := c.D[0], c.D[1]
		q, err := r.Town(at, cb, state, children)
		p.Pixels = append(p.Pixels, q.Pixels...)
		p.Sprites = append(p.Sprites, q.Sprites...)
		p.HardwarePending = q.HardwarePending
		if err != nil {
			return p, err
		}
		c.D[0], c.D[1] = saved0, saved1
		c.Word(1, uint16(c.D[1])-8)
		target = 0xe8e2
	}
	if target == 0xe8e2 {
		c.Word(1, uint16(c.D[1])+8)
		c.Word(2, m.word(at+10))
		err := r.image(cb, &p)
		return p, err
	}
	if target == 0xe5ee {
		c.Word(2, m.word(at+10))
		if err := r.image(cb, &p); err != nil {
			return p, err
		}
		err = r.markers(at, cb, &p)
		return p, err
	}
	if target == 0xe624 {
		if int16(m.word(0xeb18)) >= 0 {
			c.Byte(2, m.byte(at+12))
			if int8(c.D[2]) > 0 {
				c.ExtendWord(2)
				c.D[2] = uint32(uint16(c.D[2])) * 314
				god := 0xe76a + int(int16(c.D[2]))
				m.putByte(god+0x4b, m.byte(god+0x4b)|2)
			}
		}
		if m.byte(at+13)&16 != 0 {
			c.Word(2, m.word(at+48))
			if err := r.image(cb, &p); err != nil {
				return p, err
			}
		}
		x, y := uint16(c.D[0]), uint16(c.D[1])
		c.Word(0, m.word(at+14))
		c.Word(1, -m.word(at+16))
		if err := r.Angle(c); err != nil {
			return p, err
		}
		c.Word(0, uint16(c.D[0])>>5)
		c.Word(0, uint16(c.D[0])*2)
		base, err := r.word(0x20d34 + int(int16(c.D[0])))
		if err != nil {
			return p, err
		}
		c.Word(2, base)
		bank, selector := 0x20a00, int(int16(m.word(at+40)))
		if m.byte(at+13)&2 == 0 {
			bank = 0x209e0
			if m.byte(at+12) != 1 {
				bank = 0x209f0
			}
			selector = int(int16(m.word(at + 50)))
		}
		v, err := r.word(bank + selector)
		if err != nil {
			return p, err
		}
		c.Word(2, uint16(c.D[2])+v+m.word(at+10))
		c.RestoreWord(0, x)
		c.RestoreWord(1, y)
		if err := r.image(cb, &p); err != nil {
			return p, err
		}
		if m.word(0xeb18) == 12 && m.word(0xdd4) != 0 && m.word(0x140) != 0 {
			c.Word(3, m.word(0x134)-uint16(c.D[0]))
			if int16(c.D[3]) < 0 {
				c.Word(3, -uint16(c.D[3]))
			}
			if int16(c.D[3]) <= 8 {
				c.Word(3, m.word(0x136)-uint16(c.D[1]))
				if int16(m.word(0x136)) <= int16(c.D[1]) {
					c.Word(3, -uint16(c.D[3]))
					if int16(c.D[3]) <= 18 {
						m.putLong(0xf36, c.AddressBase+uint32(at))
						m.putWord(0x140, 0)
					}
				}
			}
		}
		err = r.markers(at, cb, &p)
		return p, err
	}
	banks := map[int]int{0xe514: 0x209a0, 0xe51e: 0x208c0, 0xe528: 0x20880, 0xe532: 0x209c0, 0xe556: 0x208a0, 0xe560: 0x208e0, 0xe56a: 0x20900, 0xe574: 0x20920, 0xe57e: 0x20940, 0xe588: 0x20980, 0xe592: 0x20960}
	bank, ok := banks[target]
	if !ok {
		return p, fmt.Errorf("native follower draw state child%#x unproved", target)
	}
	if target == 0xe532 {
		c.Byte(2, m.byte(at+12))
		if int8(c.D[2]) > 0 {
			c.ExtendWord(2)
			c.D[2] = uint32(uint16(c.D[2])) * 314
			god := 0xe76a + int(int16(c.D[2]))
			m.putByte(god+0x4b, m.byte(god+0x4b)|2)
		}
	}
	if m.byte(at+13)&2 != 0 {
		c.Word(2, m.word(at+10))
	} else {
		if m.byte(at+12) != 1 {
			bank += 16
		}
		v, err := r.word(bank + int(int16(m.word(at+50))))
		if err != nil {
			return p, err
		}
		c.Word(2, v+m.word(at+10))
	}
	if err := r.image(cb, &p); err != nil {
		return p, err
	}
	err = r.markers(at, cb, &p)
	return p, err
}

// Town is the complete raw center-image$e72e branch, including its actual
// population DIVU and mixed owner/clock descriptor adjustments.
func (r *NativeActorRenderRules) Town(at int, cb NativeRenderFrameCallbacks, state *NativeActorRenderState, children NativeActorRenderChildren) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}, Drawn: true}
	if r == nil || cb.Frame == nil || cb.Image == nil || state == nil || !winMemoryValid(cb.Memory) {
		return p, fmt.Errorf("native town renderer backing missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	if m.word(0x3b8) == 0 && m.word(0x3b0) != 0 {
		if children.TownInfo == nil {
			return p, fmt.Errorf("native town info child314a missing")
		}
		if err := children.TownInfo(at, c); err != nil {
			return p, err
		}
	}
	return r.townBody(at, cb, state)
}

// townBody resumes immediately at $e744 after the original $314a child.
func (r *NativeActorRenderRules) townBody(at int, cb NativeRenderFrameCallbacks, state *NativeActorRenderState) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}, Drawn: true}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	if m.byte(at+13)&16 != 0 {
		c.Word(2, m.word(at+48))
		if err := r.image(cb, &p); err != nil {
			return p, err
		}
	}
	c.Word(1, uint16(c.D[1])+8)
	c.D[2] = uint32(m.byte(at + 1))
	c.Word(2, uint16(c.D[2])*2)
	v, err := r.word(0x20b14 + int(int16(c.D[2])))
	if err != nil {
		return p, err
	}
	c.Word(2, v)
	image, err := r.word(0x23d1a + int(int16(c.D[2])))
	if err != nil {
		return p, err
	}
	c.Word(2, image)
	for seen := 0; seen < 256; seen++ {
		c.Word(2, uint16(c.D[2])*2)
		c.D[2] &= 0xffff
		layer := 0x26956 + int(c.D[2])
		if layer < 0 || layer+6 > len(r.Frames.code) {
			return p, fmt.Errorf("native town layer outside CODE")
		}
		x, y := c.D[0], c.D[1]
		c.Byte(2, r.Frames.code[layer])
		c.ExtendWord(2)
		c.Word(0, uint16(c.D[0])+uint16(c.D[2]))
		c.Byte(2, r.Frames.code[layer+1])
		c.ExtendWord(2)
		c.Word(1, uint16(c.D[1])+uint16(c.D[2]))
		v, err := r.word(layer + 2)
		if err != nil {
			return p, err
		}
		c.Word(2, v)
		descriptor := 0x21626 + int(int16(c.D[2]))
		draw := true
		if uint16(c.D[2]) == 0x42c {
			c.D[2] = 0
			if int16(m.word(0xeb18)) >= 0 {
				c.Byte(2, m.byte(at+12))
				if int8(c.D[2]) <= 0 {
					draw = false
				} else {
					c.D[2] = uint32(uint16(c.D[2])) * 314
					god := 0xe76a + int(int16(c.D[2]))
					m.putByte(god+0x4b, m.byte(god+0x4b)|1)
				}
			}
			if draw {
				if err := r.markers(at, cb, &p); err != nil {
					return p, err
				}
				c.D[2] = uint32(m.byte(at+1)) * 2
				c.D[3] = m.long(at + 26)
				divisor, err := r.word(0x20aee + int(int16(c.D[2])))
				if err != nil {
					return p, err
				}
				if err := frameDivide(c, 3, divisor); err != nil {
					return p, err
				}
				if int16(c.D[3]) > 24 {
					c.Word(3, 24)
				}
				c.Word(3, 24-uint16(c.D[3]))
				c.Word(1, uint16(c.D[1])+uint16(c.D[3]))
				c.Word(2, m.word(0xf42)&1)
				c.Byte(3, m.byte(at+12))
				c.ExtendWord(3)
				c.Word(3, uint16(c.D[3])-1)
				c.Word(2, (uint16(c.D[2])+uint16(c.D[3])*2)*2)
				v, err := r.word(0xe844 + int(int16(c.D[2])))
				if err != nil {
					return p, err
				}
				descriptor += int(int16(v))
			}
		} else {
			v, err := r.word(descriptor + 6)
			if err != nil {
				return p, err
			}
			c.Word(2, v)
			cb.Image.LastY = uint16(c.D[1]) - uint16(c.D[2])
			state.TownHitHeight = uint16(c.D[2])
		}
		if draw {
			half, err := r.word(descriptor + 4)
			if err != nil {
				return p, err
			}
			height, err := r.word(descriptor + 6)
			if err != nil {
				return p, err
			}
			c.Word(0, uint16(c.D[0])-half)
			c.Word(2, height)
			c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
			if err := r.Frames.descriptor(descriptor, cb, &p); err != nil {
				return p, err
			}
		}
		c.D[0], c.D[1] = x, y
		c.Word(2, binary.BigEndian.Uint16(r.Frames.code[layer+4:]))
		if uint16(c.D[2]) == 0 {
			break
		}
		if seen == 255 {
			return p, fmt.Errorf("native town layer chain unbounded")
		}
	}
	if m.word(0xeb18) == 12 && m.word(0xdd4) != 0 && m.word(0x140) != 0 {
		c.Word(2, m.word(0x134)-uint16(c.D[0]))
		if int16(c.D[2]) < 0 {
			c.Word(2, -uint16(c.D[2]))
		}
		if int16(c.D[2]) <= 16 {
			before := int16(m.word(0x136))
			c.Word(2, m.word(0x136)-uint16(c.D[1]))
			if before <= int16(c.D[1]) {
				c.Word(2, -uint16(c.D[2]))
				if int16(c.D[2]) <= int16(state.TownHitHeight) {
					m.putLong(0xf36, c.AddressBase+uint32(at))
					m.putWord(0x140, 0)
				}
			}
		}
	}
	return p, m.err
}
