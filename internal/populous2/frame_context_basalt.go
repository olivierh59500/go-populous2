package populous2

import "fmt"

type NativeFrameBasaltCallbacks struct {
	Memory FollowerCleanupMemory
	Unlink func(NativeRecordReference) error
	// Create executes $171ea with the supplied full caller context. Its wrapper
	// preserves D1-D7 and returns the actual creation result in D0.
	Create func(*NativeFrameRegisterContext) error
}

// TickFrameBasalt translates $158a6 and the distinct $15898/$15908 exits.
// Direction is a signed word offset into retained CODE, including adjacent
// table aliases. Neither kind nor a normalized direction selects admission.
func (r *NativeCommandRules) TickFrameBasalt(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameBasaltCallbacks) (NativeFrameFXStep, error) {
	step := NativeFrameFXStep{Draw: true, Color: 5}
	if r == nil || c == nil || !winMemoryValid(cb.Memory) || cb.Unlink == nil || cb.Create == nil {
		return step, fmt.Errorf("native Basalt frame callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	state := m.byte(at + 22)
	remove := func(draw bool) (NativeFrameFXStep, error) {
		m.putByte(at+12, 0)
		step.Draw = draw
		if m.err != nil {
			return step, m.err
		}
		return step, cb.Unlink(ref)
	}
	if state == 0x36 || state == 0x3a {
		return remove(true)
	}
	if state != 0x38 {
		return step, fmt.Errorf("native Basalt frame state%02x missing", state)
	}
	life := m.word(at + 24)
	m.putWord(at+24, life-1)
	if int16(life) <= 1 {
		m.putByte(at+22, 0x3a)
		return remove(false)
	}
	next := m.word(at+10) + 4
	c.Word(0, next)
	marker, e := r.word(0x23d1a + int(int16(next)))
	if e != nil {
		return step, e
	}
	if int16(marker) < 0 {
		next += marker
		c.Word(0, next)
	}
	m.putWord(at+10, next)
	timer := m.word(at + 20)
	m.putWord(at+20, timer-1)
	if int16(timer) > 1 {
		return step, m.err
	}
	direction := m.word(at + 26)
	c.Word(3, direction)
	c.Word(1, m.word(at+8))
	c.Byte(1, m.byte(at+6))
	offset, e := r.word(0x17298 + int(int16(direction)))
	if e != nil {
		return step, e
	}
	c.Word(1, uint16(c.D[1])+offset)
	c.Byte(0, uint8(c.D[1]))
	c.Word(1, uint16(c.D[1])>>8)
	c.Byte(2, m.byte(at+12))
	c.Word(4, m.word(at+24))
	if m.err != nil {
		return step, m.err
	}
	if e := cb.Create(c); e != nil {
		return step, e
	}
	m.putByte(at+22, 0x3a)
	return remove(false)
}
