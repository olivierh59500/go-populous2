package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFollowerMagnetFrameRules struct{ code []byte }

type NativeFollowerMagnetFrameCallbacks struct {
	Memory    FollowerCleanupMemory
	Frame     *NativeFrameRegisterContext
	Attrition func(NativeRecordReference, int, *NativeFrameRegisterContext) (bool, error)
	Plan      func(NativeRecordReference, *NativeFrameRegisterContext) error
	Merge     func(NativeRecordReference, NativeRecordReference, *NativeFrameRegisterContext) error
}

func DecodeNativeFollowerMagnetFrameRules(exe *amiga.Executable) (NativeFollowerMagnetFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return NativeFollowerMagnetFrameRules{}, fmt.Errorf("native magnet follower CODE missing")
	}
	return NativeFollowerMagnetFrameRules{code: exe.Hunks[0].Data}, nil
}

func (r *NativeFollowerMagnetFrameRules) god(ref NativeRecordReference, cb NativeFollowerMagnetFrameCallbacks) (int, error) {
	owner, err := cb.Memory.Read8(cleanupRecordAddress(ref) + 12)
	if err != nil {
		return 0, err
	}
	cb.Frame.Byte(2, owner)
	cb.Frame.ExtendWord(2)
	cb.Frame.D[2] = uint32(uint16(cb.Frame.D[2])) * 314
	return 0xe76a + int(int16(uint16(cb.Frame.D[2]))), nil
}

// Home is $140f0. The marker/leader reference, fractional coordinates and
// D0-only merge continuation remain distinct from the ordinary search leg.
func (r *NativeFollowerMagnetFrameRules) Home(ref NativeRecordReference, cb NativeFollowerMagnetFrameCallbacks) error {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || cb.Plan == nil {
		return fmt.Errorf("native magnet homing frame backing missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	god, err := r.god(ref, cb)
	if err != nil {
		return err
	}
	c.Word(2, m.word(god+10))
	if m.byte(at+13)&1 == 0 {
		c.Word(2, m.word(god+8))
		if uint16(c.D[2]) == 0 {
			c.Word(2, m.word(god+10))
		}
	}
	target := NativeRecordReference(uint16(c.D[2]))
	ta := cleanupRecordAddress(target)
	c.Byte(0, m.byte(ta+6))
	c.Byte(1, m.byte(ta+8))
	if m.err != nil {
		return m.err
	}
	if err := cb.Plan(ref, c); err != nil {
		return err
	}
	m.putWord(at+20, uint16(c.D[1]))
	if uint16(c.D[1]) != 0 {
		m.putByte(at+23, 0x12)
		m.putByte(at+22, 4)
		return m.err
	}
	m.putByte(at+7, m.byte(ta+7))
	m.putByte(at+9, m.byte(ta+9))
	if m.word(god+8) == 0 {
		c.D[0] = uint32(ref)
		m.putWord(god+8, uint16(c.D[0]))
		m.putByte(at+13, m.byte(at+13)|1)
	}
	if m.byte(at+13)&1 == 0 {
		if cb.Merge == nil {
			return fmt.Errorf("native magnet merge frame missing")
		}
		saved := c.D
		c.Word(0, m.word(god+8))
		if m.err != nil {
			return m.err
		}
		err := cb.Merge(ref, NativeRecordReference(uint16(c.D[0])), c)
		copy(c.D[1:], saved[1:])
		return err
	}
	m.putWord(at+10, 0x168)
	m.putByte(at+22, 0x3a)
	m.putWord(at+20, binary.BigEndian.Uint16(r.code[0x20aec:]))
	return m.err
}

// Tick executes state $12/$3a, retaining the source attrition child and exact
// direct-search/common-prepass/count/next boundaries.
func (r *NativeFollowerMagnetFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerMagnetFrameCallbacks) (uint32, error) {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return 0, fmt.Errorf("native magnet follower frame backing missing")
	}
	state, err := cb.Memory.Read8(cleanupRecordAddress(ref) + 22)
	if err != nil {
		return 0, err
	}
	if state != 0x12 && state != 0x3a {
		return 0, fmt.Errorf("native magnet follower state%02x unsupported", state)
	}
	return r.tick(ref, cb, state)
}

// Start enters $11bb4 directly after ordinary search. The caller's state may
// still be $02; the source does not normalize it before attrition/homing.
func (r *NativeFollowerMagnetFrameRules) Start(ref NativeRecordReference, cb NativeFollowerMagnetFrameCallbacks) (uint32, error) {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return 0, fmt.Errorf("native magnet search continuation backing missing")
	}
	return r.tick(ref, cb, 0x12)
}

func (r *NativeFollowerMagnetFrameRules) tick(ref NativeRecordReference, cb NativeFollowerMagnetFrameCallbacks, state uint8) (uint32, error) {
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	if state == 0x3a {
		c.Word(0, m.word(at+10)+4)
		a := 0x23d1a + int(int16(uint16(c.D[0])))
		if a < 0 || a&1 != 0 || a+2 > len(r.code) {
			return 0, fmt.Errorf("native magnet animation CODE word outside backing")
		}
		marker := binary.BigEndian.Uint16(r.code[a:])
		if int16(marker) < 0 {
			c.Word(0, uint16(c.D[0])+marker)
		}
		m.putWord(at+10, uint16(c.D[0]))
	}
	god, err := r.god(ref, cb)
	if err != nil {
		return 0, err
	}
	if m.word(god+12) != 16 {
		if state == 0x3a {
			m.putWord(at+10, 0)
		}
		return 0x1131c, m.err
	}
	attrition := state == 0x12
	if state == 0x3a {
		before := m.word(at + 20)
		m.putWord(at+20, before-1)
		attrition = int16(before) <= 1 // Original SUBI.W/BGT signed operands.
		if attrition {
			m.putWord(at+20, binary.BigEndian.Uint16(r.code[0x20aec:]))
		}
	}
	if m.err != nil {
		return 0, m.err
	}
	if attrition {
		if cb.Attrition == nil {
			return 0x130e4, nil
		}
		dead, err := cb.Attrition(ref, god, c)
		if err != nil {
			return 0, err
		}
		if dead {
			return 0x112b8, nil
		}
	}
	if state == 0x12 {
		if err := r.Home(ref, cb); err != nil {
			return 0, err
		}
		if m.byte(at+12) != 0 {
			return 0x123b4, m.err
		}
		return 0x12462, m.err
	}
	marker := cleanupRecordAddress(NativeRecordReference(m.word(god + 10)))
	c.Byte(0, m.byte(at+6))
	if uint8(c.D[0]) == m.byte(marker+6) {
		c.Byte(0, m.byte(at+8))
		if uint8(c.D[0]) == m.byte(marker+8) {
			return 0x123b4, m.err
		}
	}
	m.putByte(at+22, 0x12)
	m.putWord(at+10, 0)
	return 0x112b8, m.err
}
