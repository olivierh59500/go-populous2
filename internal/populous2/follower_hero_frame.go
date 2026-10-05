package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeFollowerHeroFrameRules retains the original selector, terrain probe,
// homing planner and state $24/$26 decisions with all eight data registers.
type NativeFollowerHeroFrameRules struct{ code []byte }

type NativeFollowerHeroFrameCallbacks struct {
	Memory  FollowerCleanupMemory
	Frame   *NativeFrameRegisterContext
	Raise   func(*NativeFrameRegisterContext) error // Genuine unpriced $d81e.
	Contact func(NativeRecordReference, NativeRecordReference, *NativeFrameRegisterContext) (FollowerContactStep, error)
	Cleanup func(NativeRecordReference, *NativeFrameRegisterContext) error // Complete $124a2.
}

type NativeFollowerHeroFrameStep struct {
	Target, RedispatchSource         NativeRecordReference
	ContactResult                    FollowerContactStep
	Selected, Contact, Waiting, Dead bool
	Continuation                     uint32 // $123b4 or $112b8, before its common prepass.
}

func DecodeNativeFollowerHeroFrameRules(exe *amiga.Executable) (NativeFollowerHeroFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return NativeFollowerHeroFrameRules{}, fmt.Errorf("native hero frame CODE missing")
	}
	return NativeFollowerHeroFrameRules{code: exe.Hunks[0].Data}, nil
}

func (r *NativeFollowerHeroFrameRules) word(at int) (uint16, error) {
	if at&1 != 0 {
		return 0, fmt.Errorf("native hero CODE word at %x is odd", at)
	}
	if r == nil || at < 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native hero CODE word at %x is outside the retained window", at)
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}

func heroFrameValid(cb NativeFollowerHeroFrameCallbacks) bool {
	return cb.Frame != nil && winMemoryValid(cb.Memory)
}

// Select is $14414. Signed-byte distance, preferred ties, fallback slot order
// and stale associations are retained rather than reduced to a nearest query.
func (r *NativeFollowerHeroFrameRules) Select(ref NativeRecordReference, cb NativeFollowerHeroFrameCallbacks) (NativeRecordReference, error) {
	if r == nil || !heroFrameValid(cb) {
		return 0, fmt.Errorf("native hero selection memory/frame missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	c.Byte(2, m.byte(at+12))
	c.Byte(3, m.byte(at+6))
	c.Byte(4, m.byte(at+8))
	c.Word(5, 0x7fff)
	preferred, fallback := 0, 0
	for other := 0x76f4; other < 0xc800; other += 52 {
		c.Byte(0, m.byte(other+12))
		if int8(c.D[0]) <= 0 || uint8(c.D[0]) == uint8(c.D[2]) || int32(m.long(other+26)) <= 0 {
			continue
		}
		c.Byte(0, m.byte(other+6)-uint8(c.D[3]))
		if int8(c.D[0]) < 0 {
			c.Byte(0, -uint8(c.D[0]))
		}
		c.Byte(1, m.byte(other+8)-uint8(c.D[4]))
		if int8(c.D[1]) < 0 {
			c.Byte(1, -uint8(c.D[1]))
		}
		c.ExtendWord(0)
		c.ExtendWord(1)
		c.Word(1, uint16(c.D[1])+uint16(c.D[0]))
		if int16(c.D[5]) < int16(c.D[1]) || other == at {
			continue
		}
		fallback = other
		if m.byte(other+13)&8 == 0 && m.word(other+36) == 0 {
			c.Word(5, uint16(c.D[1]))
			preferred = other
		}
	}
	if preferred == 0 {
		preferred = fallback
	}
	if preferred == 0 {
		c.D[0] = 0
		return 0, m.err
	}
	target := NativeRecordReference(uint16(preferred - 0x76c0))
	c.D[0] = uint32(target)
	m.putWord(at+34, uint16(c.D[0]))
	c.D[0] = uint32(ref)
	m.putWord(preferred+36, uint16(c.D[0]))
	c.D[0] = 1
	m.putByte(at+22, 0x26)
	return target, m.err
}

// Probe is $141a2. D0 contains the packed source tile and D2/D3 its candidate
// vector. Successful returns restore both saved full longs. An odd wall-table
// read reports the native exception prefix before that MOVEM executes.
func (r *NativeFollowerHeroFrameRules) Probe(ref NativeRecordReference, cb NativeFollowerHeroFrameCallbacks) error {
	if r == nil || !heroFrameValid(cb) {
		return fmt.Errorf("native hero probe memory/frame missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	saved2, saved3 := c.D[2], c.D[3]
	finish := func(result uint32) error {
		c.D[0] = result
		c.D[2], c.D[3] = saved2, saved3
		return m.err
	}
	c.Byte(0, uint8(c.D[0])+uint8(c.D[2]))
	c.Byte(1, uint8(c.D[3]))
	c.Word(1, uint16(c.D[1])<<8)
	c.Word(0, uint16(c.D[0])+uint16(c.D[1]))
	c.Word(1, uint16(c.D[0])&0xc0c0)
	if uint16(c.D[1]) != 0 {
		return finish(1)
	}
	c.Byte(0, uint8(c.D[0])*4)
	grid := 0xf44 + int(int16(c.D[0]))
	c.D[1] = 0
	c.Byte(1, m.byte(grid+1))
	c.Word(0, m.word(grid+2))
	c.Word(1, uint16(c.D[1])*2)
	property, err := r.word(0x33312 + int(int16(c.D[1])))
	if err != nil {
		return err
	}
	c.Word(3, property)
	if c.D[3]&8 != 0 {
		return finish(0xffffffff)
	}
	c.Word(3, uint16(c.D[3])&0x790)
	at := cleanupRecordAddress(ref)
	if uint16(c.D[3]) != 0 && m.byte(at+13)&2 != 0 && m.word(at+40) == 0 {
		return finish(0xffffffff)
	}
	for seen := 0; uint16(c.D[0]) != 0; seen++ {
		if seen >= NativeRecordImageSize {
			return fmt.Errorf("native hero probe chain exceeds retained window")
		}
		other := cleanupRecordAddress(NativeRecordReference(c.D[0]))
		if m.byte(other) == 0x18 {
			return finish(0xfffffffe)
		}
		if m.byte(other) == 0x1a {
			c.D[1] = 0
			c.Byte(1, m.byte(at+12))
			if uint8(c.D[1]) != m.byte(other+12) {
				threshold := func(table int) uint32 {
					c.D[1] = 0
					c.Byte(1, m.byte(at+12))
					c.D[1] = uint32(uint16(c.D[1])) * 314
					god := 0xe76a + int(int16(c.D[1]))
					c.Byte(1, m.byte(god+0x54))
					c.D[1] <<= 7
					c.D[1] += binary.BigEndian.Uint32(r.code[table:])
					return c.D[1]
				}
				if int32(threshold(0x20d60)) < int32(m.long(at+26)) {
					if m.byte(at+13)&2 != 0 {
						m.putByte(at+22, 0x2a)
						image := uint16(0x7cc)
						if m.byte(at+12) != 1 {
							image = 0x7d4
						}
						m.putWord(at+10, image)
					}
					c.Byte(0, m.byte(other+1))
					c.ExtendWord(0)
					m.putByte(other, 0x1c)
					image, err := r.word(0x20f1e + int(int16(c.D[0])))
					if err != nil {
						return err
					}
					m.putWord(other+10, image)
				} else if int32(threshold(0x20d5c)) >= int32(m.long(at+26)) {
					return finish(0xfffffffe)
				}
			}
		}
		c.Word(0, m.word(other+2))
		if m.err != nil {
			return m.err
		}
	}
	return finish(0)
}

// Plan is $1452e. It has its own MULS/DIVU motion ABI; it does not invoke the
// fractional BeginLeg routine at $13126. Same-cell return clears only D1.W.
func (r *NativeFollowerHeroFrameRules) Plan(ref NativeRecordReference, cb NativeFollowerHeroFrameCallbacks) error {
	if r == nil || !heroFrameValid(cb) {
		return fmt.Errorf("native hero planner memory/frame missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	setStep := func(reg int, target, current uint8) {
		c.D[reg] = 1
		if int8(target) <= int8(current) {
			value := uint8(0)
			if int8(target) < int8(current) {
				value = 0xff
			}
			c.Byte(reg, value)
			c.ExtendWord(reg)
		}
	}
	setStep(2, uint8(c.D[0]), m.byte(at+6))
	setStep(3, uint8(c.D[1]), m.byte(at+8))
	if uint16(c.D[2]) == 0 && uint16(c.D[3]) == 0 {
		c.Word(1, 0)
		return m.err
	}
	probe := func() error {
		c.Word(0, m.word(at+8))
		c.Byte(0, m.byte(at+6))
		return r.Probe(ref, cb)
	}
	if err := probe(); err != nil {
		return err
	}
	if uint16(c.D[0]) != 0 {
		if uint16(c.D[0]) == 0xffff {
			if m.word(0xf12) != 0 {
				if cb.Raise == nil {
					return fmt.Errorf("native hero direct raise callback missing")
				}
				saved := c.D
				c.D[0], c.D[1] = 0, 0
				c.Byte(0, m.byte(at+6)+uint8(c.D[2]))
				c.Byte(1, m.byte(at+8)+uint8(c.D[3]))
				err := cb.Raise(c)
				c.D = saved
				if err != nil {
					return err
				}
			} else {
				c.Byte(0, m.byte(at+12))
				c.ExtendWord(0)
				c.D[0] = uint32(uint16(c.D[0])) * 314
				god := 0xe76a + int(int16(c.D[0]))
				c.D[0] = uint32(ref)
				m.putWord(god+0x32, uint16(c.D[0]))
				m.putWord(god+0x34, m.word(at+8))
				m.putByte(god+0x34, m.byte(god+0x34)+uint8(c.D[3]))
				m.putByte(god+0x35, m.byte(at+6))
				m.putByte(god+0x35, m.byte(god+0x35)+uint8(c.D[2]))
			}
		}
		c.Word(0, uint16(c.D[2])*3+uint16(c.D[3]))
		c.Word(4, -uint16(c.D[2]))
		c.Word(5, -uint16(c.D[3]))
		lookup := 0x20c86 + int(int16(c.D[0])) + 4
		if lookup < 0 || lookup >= len(r.code) {
			return fmt.Errorf("native hero alternative index outside CODE")
		}
		c.Byte(0, r.code[lookup])
		c.ExtendWord(0)
		vector := 0x20c90 + int(int16(c.D[0]))
		c.Word(7, 7)
		accepted := false
		for i := 0; i < 8; i++ {
			dx, err := r.word(vector + i*4)
			if err != nil {
				return err
			}
			dy, err := r.word(vector + i*4 + 2)
			if err != nil {
				return err
			}
			c.Word(2, dx)
			c.Word(3, dy)
			if uint16(c.D[4]) != dx || uint16(c.D[5]) != dy {
				saved := [4]uint16{uint16(c.D[2]), uint16(c.D[3]), uint16(c.D[4]), uint16(c.D[5])}
				if err := probe(); err != nil {
					return err
				}
				for reg, value := range saved {
					c.RestoreWord(reg+2, value)
				}
				if uint16(c.D[0]) == 0 {
					accepted = true
					break
				}
			}
			c.Word(7, uint16(c.D[7])-1)
		}
		if !accepted {
			c.Word(2, -uint16(c.D[4]))
			c.Word(3, -uint16(c.D[5]))
			if err := probe(); err != nil {
				return err
			}
			if uint16(c.D[0]) != 0 {
				c.Word(2, 0)
				c.Word(3, 0)
			}
		}
	}
	c.D[0] = uint32(m.byte(at + 18))
	c.D[2] = uint32(int32(int16(c.D[2])) * int32(int16(c.D[0])))
	m.putWord(at+14, uint16(c.D[2]))
	c.D[3] = uint32(int32(int16(c.D[3])) * int32(int16(c.D[0])))
	m.putWord(at+16, uint16(c.D[3]))
	c.D[1] = 256
	if c.D[0] == 0 {
		return ErrFollowerZeroSpeed
	}
	if err := frameDivide(c, 1, uint16(c.D[0])); err != nil {
		return err
	}
	m.putByte(at+7, 128)
	m.putByte(at+9, 128)
	return m.err
}

// Tick executes only states $24/$26 after the common prepass. State $26
// branches back to $112b8, which must execute another genuine prepass before
// the next state dispatch. Contact swaps participants internally but restores
// A0/A1 at $12bd2, so redispatch keeps the original source reference.
func (r *NativeFollowerHeroFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerHeroFrameCallbacks) (NativeFollowerHeroFrameStep, error) {
	step := NativeFollowerHeroFrameStep{RedispatchSource: ref, Continuation: 0x123b4}
	if r == nil || !heroFrameValid(cb) {
		return step, fmt.Errorf("native hero decision memory/frame missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	state := m.byte(at + 22)
	if state == 0x24 {
		var err error
		step.Target, err = r.Select(ref, cb)
		step.Selected = step.Target != 0
		return step, err
	}
	if state != 0x26 {
		return step, fmt.Errorf("native hero decision state %x unsupported", state)
	}
	step.Continuation = 0x112b8
	c.Byte(2, m.byte(at+12))
	c.ExtendWord(2)
	c.D[2] = uint32(uint16(c.D[2])) * 314
	god := 0xe76a + int(int16(c.D[2]))
	c.D[0] = m.long(at+26) - m.long(god+0x14)
	m.putLong(at+26, c.D[0])
	// The stored MOVE.L clears V, so BGT observes the signed wrapped result.
	if int32(c.D[0]) <= 0 {
		if cb.Cleanup == nil {
			return step, fmt.Errorf("native hero attrition cleanup callback missing")
		}
		c.D[0] = 1
		if err := cb.Cleanup(ref, c); err != nil {
			return step, err
		}
		m.putWord(at+10, 0x7f4)
		if m.byte(at+13)&2 != 0 {
			m.putWord(at+10, 0x9d4)
			m.putByte(at+22, 0x40)
		} else {
			m.putByte(at+22, 0x2c)
		}
		c.D[0] = 1
		step.Dead = true
		return step, m.err
	}
	c.D[0] = 0
	for attempts := 0; ; attempts++ {
		if attempts > 400 {
			return step, fmt.Errorf("native hero reselection exceeded the retained pool")
		}
		c.Word(0, m.word(at+34))
		target := NativeRecordReference(uint16(c.D[0]))
		other := cleanupRecordAddress(target)
		valid := target != 0 && int32(m.long(other+26)) > 0
		if valid {
			c.Byte(0, m.byte(other+12))
			valid = int8(c.D[0]) > 0 && uint8(c.D[0]) != m.byte(at+12)
		}
		if valid {
			step.Target = target
			c.Byte(0, m.byte(other+6))
			c.Byte(1, m.byte(other+8))
			if err := r.Plan(ref, cb); err != nil {
				return step, err
			}
			m.putWord(at+20, uint16(c.D[1]))
			if uint16(c.D[1]) == 0 {
				if cb.Contact == nil {
					return step, fmt.Errorf("native hero contact frame callback missing")
				}
				contact, err := cb.Contact(ref, target, c)
				step.Contact, step.ContactResult = true, contact
				return step, err
			}
			m.putByte(at+23, 0x26)
			m.putByte(at+22, 4)
			return step, m.err
		}
		selected, err := r.Select(ref, cb)
		if err != nil {
			return step, err
		}
		step.Selected = selected != 0
		if selected != 0 {
			continue
		}
		m.putWord(at+20, 20)
		m.putByte(at+23, 0x24)
		m.putByte(at+22, 10)
		m.putWord(at+10, 0xccc)
		step.Waiting = true
		return step, m.err
	}
}
