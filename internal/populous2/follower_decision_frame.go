package populous2

import (
	"encoding/binary"
	"errors"
	"fmt"

	"go-populous2/internal/amiga"
)

// ErrFollowerStationarySearch is the original ILLEGAL after $13126 returns
// two zero velocities. The preceding register and velocity writes remain.
var ErrFollowerStationarySearch = errors.New("native search selected a stationary leg")

type FollowerDecisionFrameRules struct {
	code   []byte
	Motion FollowerMotionRules
}

type FollowerDecisionFrameCallbacks struct {
	Memory FollowerCleanupMemory
	Frame  *NativeFrameRegisterContext
	// $130e4 is an explicit child with the original computed A1 god address.
	// Return its actual nonzero/Z result, retaining every register mutation.
	AttritionFrame func(NativeRecordReference, int, *NativeFrameRegisterContext) (bool, error)
}

type FollowerDecisionFrameStep struct {
	Boundary uint32 // $130e4, $112b8, $12044, $11bb4, $123b4 or $1156c.
	Road     bool
}

func DecodeFollowerDecisionFrameRules(exe *amiga.Executable) (FollowerDecisionFrameRules, error) {
	var r FollowerDecisionFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return r, fmt.Errorf("native search CODE tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.Motion, err = DecodeFollowerMotionRules(exe)
	return r, err
}

func (r *FollowerDecisionFrameRules) word(a int) (uint16, error) {
	if r == nil || a < 0 || a&1 != 0 || a+2 > len(r.code) {
		return 0, fmt.Errorf("native search unaligned/outside CODE word %#x", a)
	}
	return binary.BigEndian.Uint16(r.code[a:]), nil
}

// searchChain executes the original linked-list walk. It retains D0's raw
// head/next words, including negative references and inactive record aliases.
// A corrupt cyclic save returns an error rather than silently normalizing it.
func searchFrameChain(m FollowerCleanupMemory, c *NativeFrameRegisterContext, cell int, preferred bool, own uint8) (match, blocked bool, err error) {
	head, err := m.Read16(cell + 2)
	if err != nil {
		return false, false, err
	}
	c.Word(0, head)
	seen := map[uint16]bool{}
	for uint16(c.D[0]) != 0 {
		ref := uint16(c.D[0])
		if seen[ref] {
			return false, false, fmt.Errorf("native search cyclic record chain %#x", ref)
		}
		seen[ref] = true
		at := cleanupRecordAddress(NativeRecordReference(ref))
		kind, err := m.Read8(at)
		if err != nil {
			return false, false, err
		}
		if kind == 0x18 {
			return false, true, nil
		}
		if preferred {
			owner, err := m.Read8(at + 12)
			if err != nil {
				return false, false, err
			}
			if owner == uint8(c.D[2]) && (kind == 2 || (uint8(c.D[2]) != own && kind == 4)) {
				return true, false, nil
			}
		}
		next, err := m.Read16(at + 2)
		if err != nil {
			return false, false, err
		}
		c.Word(0, next)
	}
	return false, false, nil
}

func (r *FollowerDecisionFrameRules) leg(at int, cb FollowerDecisionFrameCallbacks) error {
	m, c := cb.Memory, cb.Frame
	x, err := m.Read16(at + 6)
	if err != nil {
		return err
	}
	y, err := m.Read16(at + 8)
	if err != nil {
		return err
	}
	speed, err := m.Read8(at + 18)
	if err != nil {
		return err
	}
	a := FollowerMotionActor{X: int16(x), Y: int16(y), Speed: speed}
	err = r.Motion.BeginLegWithFrame(&a, int16(c.D[0]), int16(c.D[1]), c)
	// Native DIVU0 occurs after both velocity stores, before the timer store.
	if writeErr := m.Write16(at+14, uint16(a.VX)); writeErr != nil {
		return writeErr
	}
	if writeErr := m.Write16(at+16, uint16(a.VY)); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return err
	}
	if err := m.Write16(at+20, uint16(a.Timer)); err != nil {
		return err
	}
	if a.VX == 0 && a.VY == 0 {
		return ErrFollowerStationarySearch
	}
	return nil
}

// Search is $1131c..$1156c. It performs neither a second common prepass nor
// the movement fallthrough. Hero $12044 and magnet $11bb4 are real parent
// boundaries; no target or register result is invented for those children.
func (r *FollowerDecisionFrameRules) Search(ref NativeRecordReference, cb FollowerDecisionFrameCallbacks) (FollowerDecisionFrameStep, error) {
	var step FollowerDecisionFrameStep
	m, c := cb.Memory, cb.Frame
	if r == nil || c == nil || !winMemoryValid(m) {
		return step, fmt.Errorf("native search raw memory/frame missing")
	}
	at := cleanupRecordAddress(ref)
	own, err := m.Read8(at + 12)
	if err != nil {
		return step, err
	}
	c.Byte(2, own)
	c.ExtendWord(2)
	c.D[2] = uint32(uint16(c.D[2])) * 314
	god := 0xe76a + int(int16(c.D[2]))
	if cb.AttritionFrame == nil {
		step.Boundary = 0x130e4
		return step, nil
	}
	died, err := cb.AttritionFrame(ref, god, c)
	if err != nil {
		return step, err
	}
	if died {
		step.Boundary = 0x112b8
		return step, nil
	}
	flags, err := m.Read8(at + 13)
	if err != nil {
		return step, err
	}
	if flags&2 != 0 {
		step.Boundary = 0x12044
		return step, nil
	}
	mode, err := m.Read16(god + 12)
	if err != nil {
		return step, err
	}
	if mode == 16 {
		step.Boundary = 0x11bb4
		return step, nil
	}
	c.D[0] = 0
	index, err := m.Read8(at + 24)
	if err != nil {
		return step, err
	}
	c.Byte(0, index)
	count, err := r.word(0x20bc0 + int(int16(c.D[0])))
	if err != nil {
		return step, err
	}
	c.Word(1, count)
	y, err := m.Read16(at + 8)
	if err != nil {
		return step, err
	}
	x, err := m.Read8(at + 6)
	if err != nil {
		return step, err
	}
	c.Word(7, y)
	c.Byte(7, x)
	c.D[2] = 0
	if mode == 18 {
		c.Byte(2, own)
	}
	if mode == 20 {
		c.Word(2, 2)
		if own != 1 {
			c.Word(2, 1)
		}
	}
	c.D[4] = 0
	for pointer := 0x20be6; ; pointer += 2 {
		offset, err := r.word(pointer)
		if err != nil {
			return step, err
		}
		c.Word(3, offset+uint16(c.D[7]))
		c.Word(0, uint16(c.D[3])&0xc0c0)
		selected := false
		if uint16(c.D[0]) == 0 {
			c.Byte(3, uint8(c.D[3])*4)
			cell := 0xf44 + int(int16(c.D[3]))
			match, blocked, err := searchFrameChain(m, c, cell, true, own)
			if err != nil {
				return step, err
			}
			if match {
				c.Word(4, uint16(c.D[3]))
				selected = true
			} else if !blocked {
				c.D[0] = 0
				tile, err := m.Read8(cell + 1)
				if err != nil {
					return step, err
				}
				c.Byte(0, tile)
				c.Word(0, uint16(c.D[0])*2)
				properties, err := r.word(0x33312 + int(int16(c.D[0])))
				if err != nil {
					return step, err
				}
				if properties&1 != 0 {
					c.Word(4, uint16(c.D[3]))
					selected = uint16(c.D[2]) == 0
				}
			}
		}
		if selected {
			break
		}
		c.Word(1, uint16(c.D[1])-1)
		if uint16(c.D[1]) == 0xffff {
			break
		}
	}
	if uint16(c.D[4]) != 0 {
		c.D[0] = uint32(uint8(c.D[4]))
		c.Word(0, uint16(c.D[0])<<6)
		c.Word(1, uint16(c.D[4])&0xff00)
		if err := r.leg(at, cb); err != nil {
			return step, err
		}
	} else {
		seed, err := m.Read32(0xeb28)
		if err != nil {
			return step, err
		}
		if seed == 0 {
			seed = 0xbc614e
		}
		seed *= 0xbb40e62d
		if err := m.Write32(0xeb28, seed); err != nil {
			return step, err
		}
		c.D[0] = seed >> 8 & 0x7fff
		c.Word(6, uint16(c.D[0]))
		c.Word(0, uint16(c.D[0])&14)
		pointer := 0x20c46 + int(int16(c.D[0]))
		c.Word(3, 255)
		chosen, road := uint16(0), uint16(0)
		for reg, offset := range []int{14, 16} {
			velocity, err := m.Read16(at + offset)
			if err != nil {
				return step, err
			}
			c.D[4+reg] = 1
			if int16(velocity) <= 0 {
				value := uint8(0)
				if int16(velocity) < 0 {
					value = 255
				}
				c.Byte(4+reg, value)
			}
			c.Byte(4+reg, -uint8(c.D[4+reg]))
		}
		c.Word(1, 7)
		for {
			offset, err := r.word(pointer)
			if err != nil {
				return step, err
			}
			pointer += 2
			c.Word(2, offset+uint16(c.D[7]))
			c.Word(0, uint16(c.D[2])&0xc0c0)
			if uint16(c.D[0]) == 0 {
				c.Byte(2, uint8(c.D[2])*4)
				cell := 0xf44 + int(int16(c.D[2]))
				tile, err := m.Read8(cell + 1)
				if err != nil {
					return step, err
				}
				c.Byte(0, tile)
				c.Word(0, uint16(c.D[0])*2)
				properties, err := r.word(0x33312 + int(int16(c.D[0])))
				if err != nil {
					return step, err
				}
				c.Word(0, properties)
				if properties&8 == 0 {
					direction, err := r.word(pointer + 30)
					if err != nil {
						return step, err
					}
					if properties&64 != 0 && direction != 0 && (uint8(direction) != uint8(c.D[4]) || uint8(direction>>8) != uint8(c.D[5])) {
						road = uint16(c.D[2])
						break
					}
					_, blocked, err := searchFrameChain(m, c, cell, false, own)
					if err != nil {
						return step, err
					}
					if !blocked {
						header, err := m.Read8(cell)
						if err != nil {
							return step, err
						}
						c.Byte(0, header)
						c.Word(0, uint16(c.D[0])&248)
						pressure := uint8(c.D[0])
						if pressure < uint8(c.D[3]) || (pressure == uint8(c.D[3]) && c.D[6]&(1<<(uint16(c.D[1])&31)) != 0) {
							c.Byte(3, pressure)
							chosen = uint16(c.D[2])
						}
					}
				}
			}
			c.Word(1, uint16(c.D[1])-1)
			if uint16(c.D[1]) == 0xffff {
				break
			}
		}
		// A road jumps directly to $11504, even when its native A5 is zero.
		if road == 0 && uint16(c.D[1]) == 0xffff && chosen == 0 {
			step.Boundary = 0x123b4
			return step, nil
		}
		target := chosen
		if uint16(c.D[1]) != 0xffff {
			target = road
		}
		c.Word(0, target)
		c.Word(1, uint16(c.D[0]))
		c.Word(0, uint16(c.D[0])&255)
		c.Word(0, uint16(c.D[0])<<6)
		c.Word(1, uint16(c.D[1])&0xff00)
		bonus, err := r.word(0x2075a)
		if err != nil {
			return step, err
		}
		if road != 0 {
			c.D[5] = 0
			speed, err := m.Read8(at + 18)
			if err != nil {
				return step, err
			}
			c.Byte(5, speed)
			c.Word(5, uint16(c.D[5])+bonus)
			if int16(c.D[5]) > 255 {
				c.Word(5, 255)
			}
			if err := m.Write8(at+18, uint8(c.D[5])); err != nil {
				return step, err
			}
		}
		if err := r.leg(at, cb); err != nil {
			return step, err
		}
		if road != 0 {
			c.Word(5, bonus)
			speed, err := m.Read8(at + 18)
			if err != nil {
				return step, err
			}
			if err := m.Write8(at+18, speed-uint8(c.D[5])); err != nil {
				return step, err
			}
			step.Road = true
		}
	}
	if err := m.Write8(at+22, 4); err != nil {
		return step, err
	}
	if err := m.Write8(at+23, 2); err != nil {
		return step, err
	}
	step.Boundary = 0x1156c
	return step, nil
}
