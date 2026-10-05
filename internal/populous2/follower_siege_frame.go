package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFollowerSiegeFrameRules struct{ code []byte }

type NativeFollowerSiegeFrameCallbacks struct {
	Memory     FollowerCleanupMemory
	Frame      *NativeFrameRegisterContext
	Cleanup    func(NativeRecordReference, *NativeFrameRegisterContext) error
	ReformTown func(NativeRecordReference) error
}

func DecodeNativeFollowerSiegeFrameRules(exe *amiga.Executable) (NativeFollowerSiegeFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return NativeFollowerSiegeFrameRules{}, fmt.Errorf("native siege follower CODE missing")
	}
	return NativeFollowerSiegeFrameRules{code: exe.Hunks[0].Data}, nil
}

func (r *NativeFollowerSiegeFrameRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native siege follower CODE word%x unavailable", at)
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}

// Tick executes $11f06/$11f30/$11fde. Death is observed after the stored
// population subtraction, while hero animation and cleanup mode writes retain
// the upper D0 word. The linked target's signed owner decides the release.
func (r *NativeFollowerSiegeFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerSiegeFrameCallbacks) (uint32, error) {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return 0, fmt.Errorf("native siege follower frame backing missing")
	}
	m, c := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}, cb.Frame
	at := cleanupRecordAddress(ref)
	state := m.byte(at + 22)
	if state != 0x1c && state != 0x1e && state != 0x22 {
		return 0, fmt.Errorf("native siege follower state%02x unsupported", state)
	}
	c.Word(0, m.word(at+10)+4)
	marker, err := r.word(0x23d1a + int(int16(uint16(c.D[0]))))
	if err != nil {
		return 0, err
	}
	if int16(marker) < 0 {
		if state == 0x22 {
			m.putByte(at+22, 2)
			m.putWord(at+10, 0)
			return 0x1131c, m.err
		}
		c.Word(0, uint16(c.D[0])+marker)
	}
	m.putWord(at+10, uint16(c.D[0]))
	if state == 0x22 {
		return 0x12462, m.err
	}
	c.D[0] = m.long(at + 26)
	if int32(c.D[0]) > 0 {
		shift, err := r.word(0x20d5a)
		if err != nil {
			return 0, err
		}
		c.Word(1, shift)
		c.D[0] <<= uint(c.D[1] & 63)
		c.D[0] = uint32(int32(c.D[0])>>7) + 4
		m.putLong(at+26, m.long(at+26)-c.D[0])
	}
	target := cleanupRecordAddress(NativeRecordReference(m.word(at + 32)))
	if int8(m.byte(target+12)) > 0 {
		return 0x12462, m.err
	}
	if state == 0x1e {
		if int32(m.long(at+26)) <= 0 {
			c.Word(0, 0)
			if cb.Cleanup == nil {
				return 0, fmt.Errorf("native town siege cleanup frame missing")
			}
			return 0x12462, cb.Cleanup(ref, c)
		}
		if cb.ReformTown == nil {
			return 0, fmt.Errorf("native town siege reform callback missing")
		}
		return 0x12462, cb.ReformTown(ref)
	}
	dead := int32(m.long(at+26)) <= 0
	animation := uint16(0x750)
	heroTable := 0x20a30
	if dead {
		animation, heroTable = 0x738, 0x20a24
		m.putByte(at+22, 0x20)
	} else {
		m.putByte(at+22, 0x22)
	}
	if m.byte(at+13)&2 != 0 {
		animation, err = r.word(heroTable + int(int16(m.word(at+40))))
		if err != nil {
			return 0, err
		}
		c.Word(0, animation)
		if animation == 0 {
			return 0x12462, m.err
		}
	}
	m.putWord(at+10, animation)
	if m.err != nil {
		return 0, m.err
	}
	if dead {
		c.Word(0, 1)
		if cb.Cleanup == nil {
			return 0, fmt.Errorf("native follower siege cleanup frame missing")
		}
		return 0x12462, cb.Cleanup(ref, c)
	}
	return 0x12462, nil
}
