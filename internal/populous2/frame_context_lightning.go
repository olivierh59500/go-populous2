package populous2

import "fmt"

type NativeFrameLightningCallbacks struct {
	Memory FollowerCleanupMemory
	Random func() uint16
	Unlink func(NativeRecordReference) error
	// Dismiss is complete $15f80, including raw deity-pointer and bolt aliases.
	Dismiss func(*NativeFrameRegisterContext) error
	// Scorch is $1735a, whose wrapper preserves D0 and D4.
	Scorch func(NativeRecordReference) error
}

// TickFrameLightning translates $150c2/$150f8/$151bc using the original raw
// common record and signed hero-table indices. A wall ends the bolt scan before
// scorching; retained protected followers still acquire the effect reference.
func (r *NativeCommandRules) TickFrameLightning(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameLightningCallbacks) (NativeFrameFXStep, error) {
	step := NativeFrameFXStep{Draw: true, Color: 5}
	if r == nil || c == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Unlink == nil || cb.Dismiss == nil || cb.Scorch == nil {
		return step, fmt.Errorf("native Lightning frame callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	switch state := m.byte(at + 22); state {
	case 0x16:
		life := m.word(at + 24)
		m.putWord(at+24, life-1)
		if int16(life) <= 1 {
			c.Byte(2, m.byte(at+12))
			if m.err != nil {
				return step, m.err
			}
			return step, cb.Dismiss(c)
		}
		next := m.word(at+10) + 4
		c.Word(0, next)
		marker, e := r.word(0x23d1a + int(int16(next)))
		if e != nil {
			return step, e
		}
		if int16(marker) < 0 {
			next = 0x6f8
			c.Word(0, next)
		}
		m.putWord(at+10, next)
	case 0x1a:
		next := m.word(at+10) + 4
		c.Word(0, next)
		marker, e := r.word(0x23d1a + int(int16(next)))
		if e != nil {
			return step, e
		}
		if int16(marker) < 0 {
			m.putByte(at+12, 0)
			step.Draw = false
			if m.err != nil {
				return step, m.err
			}
			return step, cb.Unlink(ref)
		}
		m.putWord(at+10, next)
	case 0x18:
		bits := cb.Random()
		c.D[0] = uint32(bits)
		m.putWord(at+30, bits)
		m.putByte(at+7, uint8(bits))
		c.Word(0, uint16(c.D[0])>>2)
		m.putByte(at+7, uint8(c.D[0]))
		packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
		c.Word(1, packed)
		c.Byte(1, uint8(packed)*4)
		c.D[2] = 0
		head := m.word(0xf46 + int(int16(uint16(c.D[1]))))
		c.Word(0, head)
		seen := map[uint16]bool{}
		for head != 0 && m.err == nil {
			if seen[head] {
				return step, fmt.Errorf("cyclic native Lightning victim chain")
			}
			seen[head] = true
			target := cleanupRecordAddress(NativeRecordReference(head))
			kind := m.byte(target)
			c.Byte(0, kind)
			if kind == 0x1a {
				return step, m.err
			}
			if kind == 2 {
				state := m.byte(target + 22)
				if state != 0x3a && state != 0x1c {
					animation := uint16(0x738)
					if m.byte(target+13)&2 != 0 {
						word, e := r.word(0x20a84 + int(int16(m.word(target+40))))
						if e != nil {
							return step, e
						}
						animation = word
						c.Word(0, word)
					}
					if animation != 0 {
						m.putWord(target+10, animation)
						m.putByte(target+22, 0x1c)
					}
				}
				c.D[0] = uint32(int32(at - 0x76c0))
				m.putWord(target+32, uint16(c.D[0]))
			} else if kind == 4 {
				state := m.byte(target + 22)
				if state != 0x30 && state != 0x1e {
					m.putByte(target+22, 0x1e)
					m.putWord(target+10, 0x744)
				}
				c.D[0] = uint32(int32(at - 0x76c0))
				m.putWord(target+32, uint16(c.D[0]))
			}
			head = m.word(target + 2)
			c.Word(0, head)
		}
		if m.err != nil {
			return step, m.err
		}
		if e := cb.Scorch(ref); e != nil {
			return step, e
		}
	default:
		return step, fmt.Errorf("native Lightning frame state%02x missing", state)
	}
	return step, m.err
}
