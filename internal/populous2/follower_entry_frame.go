package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFollowerEntryFrameState struct{ WallObserved uint16 } // CODE $128fa.
type NativeFollowerEntryFrameRules struct {
	code   []byte
	Motion FollowerMotionRules
}
type NativeFollowerEntryFrameCallbacks struct {
	Memory     FollowerCleanupMemory
	Frame      *NativeFrameRegisterContext
	State      *NativeFollowerEntryFrameState
	Merge      func(NativeRecordReference, NativeRecordReference, *NativeFrameRegisterContext) error
	Battle     func(NativeRecordReference, NativeRecordReference, *NativeFrameRegisterContext) error
	ReformTown func(NativeRecordReference) error
}

func DecodeNativeFollowerEntryFrameRules(exe *amiga.Executable) (NativeFollowerEntryFrameRules, error) {
	var r NativeFollowerEntryFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return r, fmt.Errorf("native entry frame tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.Motion, err = DecodeFollowerMotionRules(exe)
	return r, err
}

func (r *NativeFollowerEntryFrameRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native entry frame word outside CODE")
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}

// Enter is $1275a with the actual post-movement frame. It retains raw chain
// order, signed owner/population admission and A1-in-D4 address convention.
// External merge, battle and complete town-reform bodies are explicit.
func (r *NativeFollowerEntryFrameRules) Enter(ref NativeRecordReference, cb NativeFollowerEntryFrameCallbacks) (uint32, error) {
	if r == nil || cb.Frame == nil || cb.State == nil || !winMemoryValid(cb.Memory) {
		return 0, fmt.Errorf("native entry frame backing missing")
	}
	m, c := nativeWhirlwindMemory{m: cb.Memory}, cb.Frame
	long := func(address int) uint32 {
		if m.err != nil {
			return 0
		}
		value, err := cb.Memory.Read32(address)
		m.err = err
		return value
	}
	at := cleanupRecordAddress(ref)
	c.Word(0, m.word(at+8))
	c.Byte(0, m.byte(at+6)*4)
	grid := 0xf44 + int(int16(uint16(c.D[0])))
	if m.byte(at+13)&8 != 0 {
		return 0x123b4, m.err
	}
	cb.State.WallObserved = 0
	c.Word(0, m.word(grid+2))
	target := NativeRecordReference(0)
	if uint16(c.D[0]) != 0 {
		c.Byte(2, m.byte(at+12))
		c.D[4], c.D[5] = 0, 0
		for count := 0; uint16(c.D[0]) != 0; count++ {
			if count >= 1053 {
				return 0, fmt.Errorf("native entry chain is unbounded")
			}
			current := NativeRecordReference(uint16(c.D[0]))
			other := cleanupRecordAddress(current)
			if current != ref && int8(m.byte(other+12)) > 0 && int32(long(other+26)) > 0 {
				kind := m.byte(other)
				if kind == 0x1a {
					cb.State.WallObserved = 1
				}
				if kind == 4 && uint8(c.D[2]) == m.byte(other+12) && m.byte(at+13)&2 == 0 {
					if cb.Merge == nil {
						return 0, fmt.Errorf("native entry merge frame callback missing")
					}
					if err := cb.Merge(ref, current, c); err != nil {
						return 0, err
					}
					return 0x12462, nil
				}
				priority := uint16(0)
				if kind == 4 && uint8(c.D[2]) != m.byte(other+12) {
					priority = 4
				} else if kind == 2 {
					priority = 2
					if uint8(c.D[2]) == m.byte(other+12) {
						priority = 6
					}
				}
				if priority > uint16(c.D[5]) {
					c.Word(5, priority)
					c.D[4] = c.AddressBase + uint32(other)
					target = current
				}
			}
			c.Word(0, m.word(other+2))
			if m.err != nil {
				return 0, m.err
			}
		}
		if uint16(c.D[5]) != 0 {
			priority := uint16(c.D[5])
			offset, err := r.word(0x12824 + int(int16(priority)))
			if err != nil {
				return 0, err
			}
			c.Word(5, offset)
			if priority == 4 {
				if cb.Battle == nil {
					return 0, fmt.Errorf("native entry battle frame callback missing")
				}
				if err := cb.Battle(ref, target, c); err != nil {
					return 0, err
				}
				return 0x123b4, nil
			}
			other := cleanupRecordAddress(target)
			if priority == 6 && (m.byte(at+13)|m.byte(other+13))&2 != 0 {
				return 0x123b4, m.err
			}
			m.putByte(at+23, 12)
			c.Word(0, m.word(other+6))
			c.Word(1, m.word(other+8))
			actor, err := cb.Memory.Read16(at + 6)
			if err != nil {
				return 0, err
			}
			a := FollowerMotionActor{X: int16(actor), Y: int16(m.word(at + 8)), Speed: m.byte(at + 18), VX: int16(m.word(at + 14)), VY: int16(m.word(at + 16)), Timer: int16(m.word(at + 20))}
			err = r.Motion.BeginLegWithFrame(&a, int16(c.D[0]), int16(c.D[1]), c)
			m.putWord(at+14, uint16(a.VX))
			m.putWord(at+16, uint16(a.VY))
			m.putWord(at+20, uint16(a.Timer))
			if err != nil {
				return 0, err
			}
			if m.byte(other+22) == 4 {
				c.Word(2, uint16(c.D[2])+1)
				m.putWord(other+20, uint16(c.D[2]))
				m.putByte(other+22, 10)
				m.putWord(other+10, 0xccc)
				if m.byte(other+13)&2 != 0 {
					animation, err := r.word(0x20a0c + int(int16(m.word(other+40))))
					if err != nil {
						return 0, err
					}
					m.putWord(other+10, animation)
				}
			}
			return 0x123b4, m.err
		}
	}
	c.Byte(2, m.byte(at+12))
	c.ExtendWord(2)
	c.D[2] = uint32(uint16(c.D[2])) * 314
	god := 0xe76a + int(int16(uint16(c.D[2])))
	if m.word(god+12) == 16 || m.byte(at+13)&2 != 0 || cb.State.WallObserved != 0 {
		return 0x123b4, m.err
	}
	c.Byte(0, m.byte(grid+1))
	c.Word(0, uint16(c.D[0])&255)
	c.Word(0, uint16(c.D[0])*2)
	properties, err := r.word(0x33312 + int(int16(uint16(c.D[0]))))
	if err != nil {
		return 0, err
	}
	if properties&1 == 0 {
		return 0x123b4, m.err
	}
	if cb.ReformTown == nil {
		return 0, fmt.Errorf("native entry reform continuation missing")
	}
	if err := cb.ReformTown(ref); err != nil {
		return 0, err
	}
	m.putWord(god+0x44, m.word(god+0x44)+1)
	return 0x11738, m.err
}
