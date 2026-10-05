package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFollowerWaitContactFrameRules struct{ code []byte }

type NativeFollowerWaitContactFrameCallbacks struct {
	Memory  FollowerCleanupMemory
	Frame   *NativeFrameRegisterContext
	Merge   func(NativeRecordReference, NativeRecordReference, *NativeFrameRegisterContext) error
	Contact func(NativeRecordReference, NativeRecordReference, *NativeFrameRegisterContext) (FollowerContactStep, error)
}

type NativeFollowerWaitContactFrameStep struct {
	Target          NativeRecordReference
	Merged, Contact bool
	ContactResult   FollowerContactStep
	Continuation    uint32
}

func DecodeNativeFollowerWaitContactFrameRules(exe *amiga.Executable) (NativeFollowerWaitContactFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x2541a {
		return NativeFollowerWaitContactFrameRules{}, fmt.Errorf("native waiting/contact CODE missing")
	}
	return NativeFollowerWaitContactFrameRules{code: exe.Hunks[0].Data}, nil
}

// TickWaiting is $119c8. The animation can address adjacent signed CODE words;
// timer expiry returns directly to search without changing the saved state or
// rerunning the common prepass. Only D0 is assigned by this body.
func (r *NativeFollowerWaitContactFrameRules) TickWaiting(ref NativeRecordReference, cb NativeFollowerWaitContactFrameCallbacks) (NativeFollowerWaitContactFrameStep, error) {
	s := NativeFollowerWaitContactFrameStep{Continuation: 0x123b4}
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return s, fmt.Errorf("native waiting frame backing missing")
	}
	c, m := cb.Frame, nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	c.Word(0, m.word(at+10)+4)
	codeAt := 0x23d1a + int(int16(c.D[0]))
	if codeAt&1 != 0 {
		return s, fmt.Errorf("native waiting CODE word at %x is odd", codeAt)
	}
	if codeAt < 0 || codeAt+2 > len(r.code) {
		return s, fmt.Errorf("native waiting CODE word outside retained window")
	}
	marker := binary.BigEndian.Uint16(r.code[codeAt:])
	if int16(marker) < 0 {
		c.Word(0, uint16(c.D[0])+marker)
	}
	m.putWord(at+10, uint16(c.D[0]))
	timer := m.word(at + 20)
	m.putWord(at+20, timer-1)
	// SUBI.W/BGT tests the original signed value against one, including
	// overflow when -32768 wraps to +32767. Testing the result alone differs.
	if int16(timer) <= 1 {
		m.putWord(at+10, 0)
		s.Continuation = 0x1131c
	}
	return s, m.err
}

// CompleteContact is $119f6. Its do-while scan probes reserved record $76c0
// when the head word is zero. Kind is signed; owner and population admission
// are deliberately absent. Last matching friend wins over the last enemy.
func (r *NativeFollowerWaitContactFrameRules) CompleteContact(ref NativeRecordReference, cb NativeFollowerWaitContactFrameCallbacks) (NativeFollowerWaitContactFrameStep, error) {
	s := NativeFollowerWaitContactFrameStep{Continuation: 0x1131c}
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return s, fmt.Errorf("native completed-contact frame backing missing")
	}
	c, m := cb.Frame, nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	c.Byte(2, m.byte(at+12))
	c.Word(0, m.word(at+8))
	c.Byte(0, m.byte(at+6)*4)
	grid := 0xf44 + int(int16(c.D[0]))
	c.Word(0, m.word(grid+2))
	var friend, enemy NativeRecordReference
	hasFriend, hasEnemy := false, false
	seen := map[uint16]bool{}
	for {
		current := uint16(c.D[0])
		if seen[current] {
			return s, fmt.Errorf("cyclic native completed-contact chain")
		}
		seen[current] = true
		other := cleanupRecordAddress(NativeRecordReference(current))
		if int8(m.byte(other)) <= 4 && other != at {
			if uint8(c.D[2]) == m.byte(other+12) {
				friend, hasFriend = NativeRecordReference(current), true
			} else {
				enemy, hasEnemy = NativeRecordReference(current), true
			}
		}
		c.Word(0, m.word(other+2))
		if m.err != nil {
			return s, m.err
		}
		if uint16(c.D[0]) == 0 {
			break
		}
	}
	if hasFriend {
		if cb.Merge == nil {
			return s, fmt.Errorf("native completed-contact merge callback missing")
		}
		s.Target, s.Merged, s.Continuation = friend, true, 0x12462
		if err := cb.Merge(ref, friend, c); err != nil {
			return s, err
		}
		other := cleanupRecordAddress(friend)
		if m.byte(other+22) == 10 {
			m.putWord(other+10, 0)
			m.putByte(other+22, 2)
		}
		return s, m.err
	}
	if hasEnemy {
		if cb.Contact == nil {
			return s, fmt.Errorf("native completed-contact battle callback missing")
		}
		s.Target, s.Contact, s.Continuation = enemy, true, 0x123b4
		var err error
		s.ContactResult, err = cb.Contact(ref, enemy, c)
		return s, err
	}
	if m.byte(at+13)&2 != 0 {
		s.Continuation = 0x1204e
	}
	return s, m.err
}
