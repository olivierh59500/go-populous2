package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// BeginLegWithFrame adds the full $13126 register ABI. Nil keeps the existing
// controller's behavior, including its unchanged-actor zero-speed error.
// A real context retains the native partial velocity writes before DIVU0.
func (r *FollowerMotionRules) BeginLegWithFrame(actor *FollowerMotionActor, x, y int16, c *NativeFrameRegisterContext) error {
	if c == nil {
		return r.BeginLeg(actor, x, y)
	}
	if actor == nil {
		return fmt.Errorf("native leg actor missing")
	}
	c.Word(0, uint16(x))
	c.Word(1, uint16(y))
	c.D[3] = uint32(actor.Speed)
	c.Word(2, uint16(c.D[0])>>8)
	same := uint8(c.D[2]) == uint8(uint16(actor.X)>>8)
	if same {
		c.Word(2, uint16(c.D[1])>>8)
		same = uint8(c.D[2]) == uint8(uint16(actor.Y)>>8)
	}
	if same {
		c.Word(2, uint16(c.D[3]))
		c.Word(0, uint16(c.D[0])&0xff)
		c.Word(1, uint16(c.D[1])&0xff)
		c.D[4], c.D[5] = uint32(uint8(actor.X)), uint32(uint8(actor.Y))
		c.Word(0, uint16(c.D[0])-uint16(c.D[4]))
		if int16(c.D[0]) <= 0 {
			if uint16(c.D[0]) != 0 {
				c.Byte(0, uint8(-uint8(c.D[0])))
				c.Word(2, 0)
			}
			c.Word(2, uint16(c.D[2])-uint16(c.D[3]))
		}
		actor.VX = int16(c.D[2])
		c.Word(2, uint16(c.D[3]))
		c.Word(1, uint16(c.D[1])-uint16(c.D[5]))
		if int16(c.D[1]) <= 0 {
			if uint16(c.D[1]) != 0 {
				c.Byte(1, uint8(-uint8(c.D[1])))
				c.Word(2, 0)
			}
			c.Word(2, uint16(c.D[2])-uint16(c.D[3]))
		}
		actor.VY = int16(c.D[2])
		c.D[0] &= 0xff
		c.D[1] &= 0xff
		reg := 0
		if int16(c.D[1]) > int16(c.D[0]) {
			reg = 1
		}
		if actor.Speed == 0 {
			return ErrFollowerZeroSpeed
		}
		if err := frameDivide(c, reg, uint16(actor.Speed)); err != nil {
			return err
		}
		c.Word(2, uint16(c.D[reg]))
	} else {
		c.Word(0, uint16(c.D[0])>>8)
		c.Word(1, uint16(c.D[1])>>8)
		c.Word(2, uint16(c.D[3]))
		left, right := int8(uint8(c.D[0])), int8(uint16(actor.X)>>8)
		if left <= right {
			if left != right {
				c.Word(2, 0)
			}
			c.Word(2, uint16(c.D[2])-uint16(c.D[3]))
		}
		actor.VX = int16(c.D[2])
		c.Word(2, uint16(c.D[3]))
		left, right = int8(uint8(c.D[1])), int8(uint16(actor.Y)>>8)
		if left <= right {
			if left != right {
				c.Word(2, 0)
			}
			c.Word(2, uint16(c.D[2])-uint16(c.D[3]))
		}
		actor.VY = int16(c.D[2])
		c.D[2] = 256
		if actor.Speed == 0 {
			return ErrFollowerZeroSpeed
		}
		if err := frameDivide(c, 2, uint16(actor.Speed)); err != nil {
			return err
		}
	}
	actor.Timer = int16(c.D[2])
	return nil
}

type FollowerMotionFrameRules struct {
	code     []byte
	Crossing FollowerCrossingRules
}

type FollowerMotionFrameCallbacks struct {
	Memory FollowerCleanupMemory
	Frame  *NativeFrameRegisterContext
	// The real child bodies own their full register continuation. They must
	// not reset registers on entry or replace graph/contact with scalar state.
	Move      func(NativeRecordReference, uint16, uint16, *NativeFrameRegisterContext) error // $12518.
	Enter     func(NativeRecordReference, *NativeFrameRegisterContext) error                 // $1275a.
	WallBreak func(NativeRecordReference, *NativeFrameRegisterContext) error                 // $11ce8.
}

type FollowerMotionFrameStep struct {
	Moved, Crossed, Blocked, WallBroken, LandRequested bool
	Redispatch                                         bool   // Original branch to $112b8, before another prepass.
	Boundary                                           uint32 // $123b4, $112b8, $1275a or $11ce8 source continuation.
}

func DecodeFollowerMotionFrameRules(exe *amiga.Executable) (FollowerMotionFrameRules, error) {
	var r FollowerMotionFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native motion frame tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.Crossing, err = DecodeFollowerCrossingRules(exe)
	return r, err
}

func (r *FollowerMotionFrameRules) codeWord(a int) (uint16, error) {
	if r == nil || a < 0 || a&1 != 0 || a+2 > len(r.code) {
		return 0, fmt.Errorf("native motion word outside CODE")
	}
	return binary.BigEndian.Uint16(r.code[a:]), nil
}

// Movement is $1156c through its actual tail boundaries. Timer-expiry returns
// to the parent-owned $112b8 dispatcher; it does not invent a decision callback
// or run a second prepass. All mutations use the raw retained record/grid/god
// backing. Committed crossing and wall-break callbacks are explicit children.
func (r *FollowerMotionFrameRules) Movement(ref NativeRecordReference, cb FollowerMotionFrameCallbacks) (FollowerMotionFrameStep, error) {
	var s FollowerMotionFrameStep
	m, c := cb.Memory, cb.Frame
	if r == nil || c == nil || !winMemoryValid(m) {
		return s, fmt.Errorf("native motion raw memory/frame missing")
	}
	at := cleanupRecordAddress(ref)
	animation, err := m.Read16(at + 10)
	if err != nil {
		return s, err
	}
	c.Word(0, animation+4)
	marker, err := r.codeWord(0x23d1a + int(int16(c.D[0])))
	if err != nil {
		return s, err
	}
	c.Word(1, marker)
	if int16(c.D[1]) < 0 {
		c.D[0] = 0
	}
	if err := m.Write16(at+10, uint16(c.D[0])); err != nil {
		return s, err
	}
	timer, err := m.Read16(at + 20)
	if err != nil {
		return s, err
	}
	if err := m.Write16(at+20, timer-1); err != nil {
		return s, err
	}
	// SUBI.W/BGE compares the signed operands, preserving its overflow
	// behavior when -32768 decrements to +32767.
	if int16(timer) < 1 {
		state, err := m.Read8(at + 23)
		if err != nil {
			return s, err
		}
		if err := m.Write8(at+22, state); err != nil {
			return s, err
		}
		s.Redispatch, s.Boundary = true, 0x112b8
		return s, nil
	}
	x, err := m.Read16(at + 6)
	if err != nil {
		return s, err
	}
	y, err := m.Read16(at + 8)
	if err != nil {
		return s, err
	}
	c.RestoreWord(6, x)
	c.RestoreWord(7, y)
	vx, err := m.Read16(at + 14)
	if err != nil {
		return s, err
	}
	vy, err := m.Read16(at + 16)
	if err != nil {
		return s, err
	}
	c.Word(6, uint16(c.D[6])+vx)
	c.Word(7, uint16(c.D[7])+vy)
	c.Word(0, uint16(c.D[6])>>8)
	crossed := uint8(c.D[0]) != uint8(x>>8)
	if !crossed {
		c.Word(1, uint16(c.D[7])>>8)
		crossed = uint8(c.D[1]) != uint8(y>>8)
	}
	if !crossed {
		if err := m.Write16(at+6, uint16(c.D[6])); err != nil {
			return s, err
		}
		if err := m.Write16(at+8, uint16(c.D[7])); err != nil {
			return s, err
		}
		s.Moved, s.Boundary = true, 0x123b4
		return s, nil
	}
	owner, err := m.Read8(at + 12)
	if err != nil {
		return s, err
	}
	c.Byte(2, owner)
	c.ExtendWord(2)
	c.D[2] = uint32(uint16(c.D[2])) * 314
	god := 0xe76a + int(int16(c.D[2]))
	c.Word(1, uint16(c.D[7]))
	c.Byte(1, uint8(c.D[0]))
	c.Word(0, uint16(c.D[1])&0xc0c0)
	blocked := uint16(c.D[0]) != 0
	if !blocked {
		c.Byte(1, uint8(c.D[1])*4)
		cell := 0xf44 + int(int16(c.D[1]))
		header, err := m.Read8(cell)
		if err != nil {
			return s, err
		}
		c.Byte(0, header)
		c.Word(0, uint16(c.D[0])&7)
		if uint16(c.D[0]) == 0 {
			tile, err := m.Read8(cell + 1)
			if err != nil {
				return s, err
			}
			c.Byte(0, tile)
			if r.Crossing.Raster[tile]&1 == 0 {
				c.D[2] = uint32(int32(int16(ref)))
				if err := m.Write16(god+0x32, uint16(c.D[2])); err != nil {
					return s, err
				}
				c.Word(2, uint16(c.D[1]))
				c.Byte(2, uint8(c.D[2])>>2)
				if err := m.Write16(god+0x34, uint16(c.D[2])); err != nil {
					return s, err
				}
				s.LandRequested = true
			}
		}
		c.Byte(0, uint8(c.D[0])*2)
		blocked = r.Crossing.Properties[int(uint8(c.D[0]))/2]&8 != 0
		if !blocked {
			head, err := m.Read16(cell + 2)
			if err != nil {
				return s, err
			}
			c.Word(0, head)
			for count := 0; uint16(c.D[0]) != 0; count++ {
				if count >= NativeRecordImageSize {
					return s, fmt.Errorf("native motion crossing chain exceeds retained window")
				}
				other := cleanupRecordAddress(NativeRecordReference(c.D[0]))
				kind, err := m.Read8(other)
				if err != nil {
					return s, err
				}
				if kind == 0x18 {
					blocked = true
					break
				}
				if kind == 0x1a {
					c.Byte(2, owner)
					otherOwner, err := m.Read8(other + 12)
					if err != nil {
						return s, err
					}
					if uint8(c.D[2]) == otherOwner {
						break
					}
					xp, err := m.Read8(god + 0x54)
					if err != nil {
						return s, err
					}
					population, err := m.Read32(at + 26)
					if err != nil {
						return s, err
					}
					c.D[2] = uint32(xp) << 7
					c.D[2] += r.Crossing.WallThresholds[1]
					if int32(c.D[2]) < int32(population) {
						if err := m.Write8(at+22, 0x2a); err != nil {
							return s, err
						}
						animation := uint16(0x7cc)
						if owner != 1 {
							animation = 0x7d4
						}
						if err := m.Write16(at+10, animation); err != nil {
							return s, err
						}
						stage, err := m.Read8(other + 1)
						if err != nil {
							return s, err
						}
						c.Byte(0, stage)
						c.ExtendWord(0)
						if err := m.Write8(other, 0x1c); err != nil {
							return s, err
						}
						image, err := r.codeWord(0x20f1e + int(int16(c.D[0])))
						if err != nil {
							return s, err
						}
						if err := m.Write16(other+10, image); err != nil {
							return s, err
						}
						s.WallBroken, s.Boundary = true, 0x11ce8
						if cb.WallBreak != nil {
							err = cb.WallBreak(ref, c)
						}
						return s, err
					}
					c.D[2] = uint32(xp) << 7
					c.D[2] += r.Crossing.WallThresholds[0]
					blocked = c.D[2] > population
					break
				}
				next, err := m.Read16(other + 2)
				if err != nil {
					return s, err
				}
				c.Word(0, next)
			}
		}
	}
	if blocked {
		c.D[0], c.D[1] = 1, 1
		if int16(vx) <= 0 {
			value := uint8(0)
			if int16(vx) < 0 {
				value = 0xff
			}
			c.Byte(0, value)
			c.ExtendWord(0)
		}
		if int16(vy) <= 0 {
			value := uint8(0)
			if int16(vy) < 0 {
				value = 0xff
			}
			c.Byte(1, value)
			c.ExtendWord(1)
		}
		c.Word(0, uint16(c.D[0])+1)
		c.Word(1, uint16(c.D[1])+1)
		c.Word(1, uint16(c.D[1])<<2)
		c.Word(1, uint16(c.D[1])+uint16(c.D[0]))
		c.Word(1, uint16(c.D[1])*2)
		speed, err := m.Read8(at + 18)
		if err != nil {
			return s, err
		}
		c.Byte(0, speed)
		c.Word(6, uint16(c.D[0]))
		lookup := 0x1171e + int(int16(c.D[1]))
		if lookup < 0 || lookup+2 > len(r.code) {
			return s, fmt.Errorf("native motion bounce outside CODE")
		}
		c.Byte(2, r.code[lookup])
		if int8(c.D[2]) <= 0 {
			if uint8(c.D[2]) != 0 {
				c.Word(6, 0)
			}
			c.Word(6, uint16(c.D[6])-uint16(c.D[0]))
		}
		c.Word(7, uint16(c.D[0]))
		c.Byte(2, r.code[lookup+1])
		if int8(c.D[2]) <= 0 {
			if uint8(c.D[2]) != 0 {
				c.Word(7, 0)
			}
			c.Word(7, uint16(c.D[7])-uint16(c.D[0]))
		}
		if err := m.Write16(at+14, uint16(c.D[6])); err != nil {
			return s, err
		}
		if err := m.Write16(at+16, uint16(c.D[7])); err != nil {
			return s, err
		}
		s.Blocked, s.Boundary = true, 0x123b4
		return s, nil
	}
	s.Moved, s.Crossed, s.Boundary = true, true, 0x1275a
	if cb.Move == nil {
		return s, fmt.Errorf("native motion12518 callback missing")
	}
	if err := cb.Move(ref, uint16(c.D[6]), uint16(c.D[7]), c); err != nil {
		return s, err
	}
	if cb.Enter != nil {
		if err := cb.Enter(ref, c); err != nil {
			return s, err
		}
	}
	return s, nil
}
