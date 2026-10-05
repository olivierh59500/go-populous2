package populous2

import "fmt"

type NativeFrameHurricaneCallbacks struct {
	Memory  FollowerCleanupMemory
	Move    func(NativeRecordReference, *NativeFrameRegisterContext) error
	Cleanup func(NativeRecordReference, *NativeFrameRegisterContext) error
	Unlink  func(NativeRecordReference) error
}

func frameHurricaneRemove(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameHurricaneCallbacks) error {
	at := cleanupRecordAddress(ref)
	m := nativeWhirlwindMemory{m: cb.Memory}
	if at >= 0xe740 {
		upper, e := m.m.Read32(0xe76a)
		if e != nil {
			return e
		}
		if int32(c.AddressBase+uint32(at)) < int32(upper) {
			return nil
		}
	}
	if at >= 0x76c0 && at < 0xc800 {
		saved0, saved1 := c.D[0], c.D[1]
		c.D[0] = 0
		e := cb.Cleanup(ref, c)
		c.D[0], c.D[1] = saved0, saved1
		return e
	}
	m.putByte(at+12, 0)
	if m.err != nil {
		return m.err
	}
	return cb.Unlink(ref)
}

// TickFrameHurricane translates $15916-$15ace. It reads whole parcels in the
// two linear sweeps, preserves raw headers in D0 and follows next links after
// moving. Boundary cleanup ends that cell immediately, as the source does.
func (r *NativeCommandRules) TickFrameHurricane(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameHurricaneCallbacks) (NativeFrameFXStep, error) {
	step := NativeFrameFXStep{Draw: true, Color: 5}
	if r == nil || c == nil || !winMemoryValid(cb.Memory) || cb.Move == nil || cb.Cleanup == nil || cb.Unlink == nil {
		return step, fmt.Errorf("native Hurricane frame callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	life := m.word(at + 24)
	m.putWord(at+24, life-1)
	if int16(life) <= 1 {
		m.putByte(at+12, 0)
		step.Draw = false
		return step, m.err
	}
	c.D[5] = uint32(m.byte(at + 18))
	packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	gridOffset := packed&0xff00 | uint16(uint8(packed)*4)
	c.Word(0, gridOffset)
	c.Word(0, uint16(c.D[0])>>2)
	first := int(int16(uint16(c.D[0])))
	direction := m.word(at + 26)
	c.Word(0, direction)
	offset, e := r.word(0x15960 + int(int16(direction)))
	if e != nil {
		return step, e
	}
	c.Word(0, offset)
	target := 0x15960 + int(int16(offset))
	if target != 0x15968 && target != 0x159a8 && target != 0x159fe && target != 0x15a3a {
		return step, fmt.Errorf("native Hurricane direction%04x dispatch%05x missing", direction, target)
	}
	cell := func(index int, longRead bool) error {
		m.putByte(0x4f44+index, 0)
		grid := 0xf44 + index*4
		if longRead {
			v, e := m.m.Read32(grid)
			if e != nil {
				return e
			}
			c.D[0] = v
		} else {
			c.Word(0, m.word(grid+2))
		}
		head := uint16(c.D[0])
		visits := 0
		for head != 0 && m.err == nil {
			visits++
			if visits > 65536 {
				return fmt.Errorf("native Hurricane linked traversal does not terminate")
			}
			ref := NativeRecordReference(head)
			actor := cleanupRecordAddress(ref)
			x, y := m.word(actor+6), m.word(actor+8)
			c.RestoreWord(6, x)
			c.RestoreWord(7, y)
			inside := true
			switch target {
			case 0x15968:
				y -= uint16(c.D[5])
				c.Word(7, y)
				inside = int16(y) >= 0
			case 0x159a8:
				x += uint16(c.D[5])
				c.Word(6, x)
				inside = int16(x) < 0x4000
			case 0x159fe:
				y += uint16(c.D[5])
				c.Word(7, y)
				inside = int16(y) < 0x4000
			case 0x15a3a:
				x -= uint16(c.D[5])
				c.Word(6, x)
				inside = int16(x) >= 0
			}
			if !inside {
				return frameHurricaneRemove(ref, c, cb)
			}
			if m.err != nil {
				return m.err
			}
			if e := cb.Move(ref, c); e != nil {
				return e
			}
			head = m.word(actor + 2)
			c.Word(0, head)
		}
		return m.err
	}
	switch target {
	case 0x15968:
		for index := first; index >= 0; index-- {
			if e := cell(index, true); e != nil {
				return step, e
			}
		}
	case 0x159fe:
		for index := first; index < 4096; index++ {
			if e := cell(index, true); e != nil {
				return step, e
			}
		}
	case 0x159a8:
		c.D[4] = 63
		c.Byte(4, uint8(c.D[4])-m.byte(at+6))
		columns := uint16(c.D[4])
		for col := int(columns); col >= 0; col-- {
			c.Word(4, uint16(col))
			c.D[1] = 63
			column := first/64*64 + first%64 + int(columns) - col
			for row := 0; row < 64; row++ {
				c.Word(1, uint16(63-row))
				if e := cell(column+row*64, false); e != nil {
					return step, e
				}
			}
			c.Word(1, 0xffff)
		}
		c.Word(4, 0xffff)
	case 0x15a3a:
		c.D[4] = uint32(m.byte(at + 6))
		columns := uint16(c.D[4])
		for col := int(columns); col >= 0; col-- {
			c.Word(4, uint16(col))
			c.D[1] = 63
			column := first - int(columns) + col
			for row := 0; row < 64; row++ {
				c.Word(1, uint16(63-row))
				if e := cell(column+row*64, false); e != nil {
					return step, e
				}
			}
			c.Word(1, 0xffff)
		}
		c.Word(4, 0xffff)
	}
	return step, m.err
}
