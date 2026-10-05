package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type NativeFollowerRetainedFrameRules struct{ code []byte }

func DecodeNativeFollowerRetainedFrameRules(exe *amiga.Executable) (NativeFollowerRetainedFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return NativeFollowerRetainedFrameRules{}, fmt.Errorf("native retained follower frame CODE missing")
	}
	return NativeFollowerRetainedFrameRules{code: exe.Hunks[0].Data}, nil
}
func (r *NativeFollowerRetainedFrameRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native retained follower CODE word unavailable")
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}

type NativeFollowerRetainedFrameCallbacks struct {
	Memory            FollowerCleanupMemory
	Frame             *NativeFrameRegisterContext
	Attrition         func(NativeRecordReference, int, *NativeFrameRegisterContext) (bool, error)
	Plan, ClearLeader func(NativeRecordReference, *NativeFrameRegisterContext) error
	Unlink            func(NativeRecordReference) error
}

// Tick translates captive34/$120cc and ruin46/$12074. It does not run another
// prepass. Captive backlink repair retains the original A1 target address even
// when it overlaps the newly written source/captor records.
func (r *NativeFollowerRetainedFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerRetainedFrameCallbacks) (uint32, error) {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return 0, fmt.Errorf("native retained follower frame backing missing")
	}
	m, c := nativeWhirlwindMemory{m: cb.Memory}, cb.Frame
	at := cleanupRecordAddress(ref)
	switch state := m.byte(at + 22); state {
	case 0x46, 0x30:
		timer := m.word(at + 20)
		m.putWord(at+20, timer-1)
		if int16(timer) > 1 {
			packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
			c.Word(0, packed&0xff00|uint16(uint8(packed)*4))
			tile := m.byte(0xf45 + int(int16(uint16(c.D[0]))))
			c.Byte(0, tile)
			c.Word(0, uint16(c.D[0])&255)
			rasterAt := 0x33512 + int(uint16(c.D[0]))
			if rasterAt >= len(r.code) {
				return 0, fmt.Errorf("native retained follower raster unavailable")
			}
			c.Byte(0, r.code[rasterAt])
			c.Word(0, uint16(c.D[0])&15)
			if uint8(c.D[0]) == 15 {
				return 0x12462, m.err
			}
		}
		if m.byte(at+13)&1 != 0 {
			if cb.ClearLeader == nil {
				return 0, fmt.Errorf("native ruin leader frame callback missing")
			}
			if e := cb.ClearLeader(ref, c); e != nil {
				return 0, e
			}
		}
		m.putByte(at+12, 0)
		if e := m.m.Write32(at+26, 0); e != nil {
			return 0, e
		}
		if m.err != nil {
			return 0, m.err
		}
		if cb.Unlink == nil {
			return 0, fmt.Errorf("native ruin unlink frame callback missing")
		}
		if e := cb.Unlink(ref); e != nil {
			return 0, e
		}
		return 0x12462, nil
	case 0x34:
		c.Byte(2, m.byte(at+12))
		c.ExtendWord(2)
		c.D[2] = uint32(uint16(c.D[2])) * 314
		god := 0xe76a + int(int16(uint16(c.D[2])))
		if cb.Attrition == nil {
			return 0, fmt.Errorf("native captive attrition frame callback missing")
		}
		dead, e := cb.Attrition(ref, god, c)
		if e != nil {
			return 0, e
		}
		if dead {
			return 0x112b8, nil
		}
		target := cleanupRecordAddress(NativeRecordReference(m.word(at + 42)))
		flags, owner := m.byte(target+13), m.byte(target+12)
		if !(flags&2 != 0 && int8(owner) > 0) && !(int8(owner) > 0 && flags&8 != 0) {
			captor := cleanupRecordAddress(NativeRecordReference(m.word(at + 44)))
			backOwner, backFlags := m.byte(captor+12), m.byte(captor+13)
			if int8(backOwner) <= 0 || backFlags&2 == 0 {
				m.putByte(at+13, m.byte(at+13)&^8)
				m.putByte(at+22, 2)
				return 0x112b8, m.err
			}
			c.D[0] = uint32(int32(at - 0x76c0))
			head := m.word(captor + 42)
			c.Word(1, head)
			if head != 0 {
				previous := cleanupRecordAddress(NativeRecordReference(head))
				if int8(m.byte(previous+12)) > 0 && m.byte(previous+13)&8 != 0 {
					m.putWord(previous+42, uint16(c.D[0]))
				}
			}
			c.D[1] = uint32(int32(captor - 0x76c0))
			m.putWord(at+42, uint16(c.D[1]))
			m.putWord(captor+42, uint16(c.D[0]))
		}
		// $1216e reads from the saved original target, after any overlapping writes.
		c.Byte(0, m.byte(target+6))
		c.Byte(1, m.byte(target+8))
		if m.err != nil {
			return 0, m.err
		}
		if cb.Plan == nil {
			return 0, fmt.Errorf("native captive planner frame callback missing")
		}
		if e := cb.Plan(ref, c); e != nil {
			return 0, e
		}
		m.putWord(at+20, uint16(c.D[1]))
		m.putByte(at+23, 0x34)
		m.putByte(at+22, 4)
		return 0x123b4, m.err
	default:
		return 0, fmt.Errorf("native retained follower state%02x outside family", state)
	}
}
