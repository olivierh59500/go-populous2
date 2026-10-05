package populous2

import "fmt"

type NativeFrameFXStep struct {
	// Draw is the original continuation to$15ad4. False reaches$15b6a
	// directly; a controller that changes state still owns this branch.
	Draw  bool
	Color uint16
}
type NativeFrameFXCallbacks struct {
	Memory FollowerCleanupMemory
	Tick   func(NativeRecordReference, *NativeFrameRegisterContext) (NativeFrameFXStep, error)
}

// TickFrameFX is original $1482e through its shared $15ad4/$15b6a suffix.
// It scans raw owner/state bytes; activity, kind and allocation metadata do
// not replace the source dispatch. Inner bodies must supply their real
// register writes and continuation, never an assumed empty callback.
func (r *NativeCommandRules) TickFrameFX(c *NativeFrameRegisterContext, cb NativeFrameFXCallbacks) error {
	if c == nil || !winMemoryValid(cb.Memory) || cb.Tick == nil {
		return fmt.Errorf("native FX frame callbacks/context missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	for at := 0xc800; at < 0xe740; at += 32 {
		if m.byte(at+12) == 0 {
			if m.err != nil {
				return m.err
			}
			continue
		}
		state := m.byte(at + 22)
		c.D[0] = 0
		c.Byte(0, state)
		offset, e := r.word(0x14852 + int(int16(uint16(c.D[0]))))
		if e != nil {
			return e
		}
		c.Word(0, offset)
		step, e := cb.Tick(NativeRecordReference(uint16(at-0x76c0)), c)
		if e != nil {
			return e
		}
		if !step.Draw {
			continue
		}
		if e := r.frameFXDraw(at, step.Color, c, cb.Memory); e != nil {
			return e
		}
	}
	return m.err
}

func (r *NativeCommandRules) frameFXDraw(at int, color uint16, c *NativeFrameRegisterContext, memory FollowerCleanupMemory) error {
	m := nativeWhirlwindMemory{m: memory}
	c.Byte(2, m.byte(at+12))
	c.ExtendWord(2)
	scenario := m.word(0xeb2c)
	if m.word(0xeb42) != 1 {
		scenario = m.word(0xeb2e)
	}
	c.Word(0, scenario)
	if scenario&0x100 != 0 {
		return m.err
	}
	c.Word(2, color)
	c.Word(4, m.word(0xf42)&7)
	if uint16(c.D[4]) == 0 {
		cursor := m.word(0xeb6e)
		queue := 0xeb70 + int(int16(cursor))
		if queue < 0x11280 {
			c.D[3] = uint32(m.byte(at + 6))
			c.Word(3, uint16(c.D[3])<<6)
			c.Byte(3, uint8(c.D[3])+m.byte(at+8))
			m.putWord(queue, uint16(c.D[3]))
			m.putWord(0xeb6e, m.word(0xeb6e)+2)
		}
	}
	c.D[1] = uint32(m.byte(at + 8))
	c.D[0] = 64
	c.Word(0, uint16(c.D[0])-uint16(c.D[1]))
	c.D[3] = uint32(m.byte(at + 6))
	c.Word(0, uint16(c.D[0])+uint16(c.D[3]))
	c.Word(1, (uint16(c.D[1])+uint16(c.D[3]))>>1)
	if m.word(0xf0c) == 8 {
		c.Word(0, uint16(c.D[0])+4)
		c.Word(1, uint16(c.D[1])+4)
		pixelX := uint16(c.D[0])
		c.Word(1, uint16(c.D[1])<<3)
		c.Word(1, uint16(c.D[1])*4)
		c.Word(1, pixelX)
		c.Word(0, (pixelX&7)^7)
		c.Word(1, uint16(c.D[1])>>3)
		c.Word(2, (uint16(c.D[2])&15)<<4)
	}
	return m.err
}
