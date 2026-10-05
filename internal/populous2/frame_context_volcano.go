package populous2

import "fmt"

type NativeFrameVolcanoCallbacks struct {
	Memory           FollowerCleanupMemory
	Random           func() uint16
	Terrain          func(bool, *NativeFrameRegisterContext) error
	FireColumn, Lava func(*NativeFrameRegisterContext) error
}

// TickFrameVolcano translates $16d2c/$16e6e using the source CODE cursor.
// Terrain calls preserve D0-D5, leaving their actual D6/D7 continuation live.
// The final DBF owner-word alias and raw stage offsets are not normalized.
func (r *NativeCommandRules) TickFrameVolcano(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameVolcanoCallbacks) (NativeFrameFXStep, error) {
	step := NativeFrameFXStep{Draw: true, Color: 5}
	if r == nil || c == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Terrain == nil || cb.FireColumn == nil || cb.Lava == nil {
		return step, fmt.Errorf("native Volcano frame callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	state := m.byte(at + 22)
	if state == 0x32 {
		m.putByte(at+12, 0)
		step.Draw = false
		return step, m.err
	}
	if state != 0x30 {
		return step, fmt.Errorf("native Volcano frame state%02x missing", state)
	}
	stage := 0x171c6 + int(int16(m.word(at+20)))
	size, e := r.byte(stage)
	if e != nil {
		return step, e
	}
	c.D[2], c.D[3], c.D[4] = uint32(size), uint32(m.byte(at+6)), uint32(m.byte(at+8))
	c.Word(0, uint16(size)>>1)
	left := int(int16(uint16(c.D[3]))) - int(uint16(c.D[0])) - 1
	top := int(int16(uint16(c.D[4]))) - int(uint16(c.D[0])) - 1
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}
	c.Word(3, uint16(left))
	c.Word(4, uint16(top))
	terrain := func(raise bool) error {
		saved := c.D
		if e := cb.Terrain(raise, c); e != nil {
			return e
		}
		for reg := 0; reg <= 5; reg++ {
			c.D[reg] = saved[reg]
		}
		return nil
	}
	for row := int(size); row >= 0; row-- {
		c.Word(2, uint16(row))
		c.D[1] = uint32(size)
		for col := int(size); col >= 0; col-- {
			c.Word(1, uint16(col))
			packed := uint16(top+row)<<8 | uint16(uint8(left+col))
			c.Word(5, packed)
			c.Word(0, packed&0xc0c0)
			if uint16(c.D[0]) == 0 {
				c.Byte(5, uint8(packed)*4)
				grid := 0xf44 + int(int16(uint16(c.D[5])))
				height := m.byte(grid) & 7
				c.Word(0, uint16(height))
				if height != 0 || m.byte(grid+1) != 0 {
					for attempt := int(height); attempt >= 0; attempt-- {
						c.Word(0, uint16(attempt))
						saved := c.D
						linear := uint16(c.D[5]) >> 2
						c.Word(5, linear)
						c.Byte(0, uint8(linear))
						c.Word(1, linear>>6)
						if e := cb.Terrain(false, c); e != nil {
							return step, e
						}
						for reg := 0; reg <= 5; reg++ {
							c.D[reg] = saved[reg]
						}
					}
					c.Word(0, 0xffff)
				}
			}
		}
		c.Word(1, 0xffff)
	}
	c.Word(2, 0xffff)
	for cursor := 0x171c6; ; cursor += 4 {
		c.D[0], c.D[1] = uint32(m.byte(at+6)), uint32(m.byte(at+8))
		for offset := 1; offset <= 2; offset++ {
			direction, e := r.byte(cursor + offset)
			if e != nil {
				return step, e
			}
			if direction != 0 {
				if e := terrain(int8(direction) > 0); e != nil {
					return step, e
				}
			}
		}
		if cursor+4 > stage {
			break
		}
	}
	c.D[2] = uint32(size)
	for row := int(size); row >= 0; row-- {
		c.Word(2, uint16(row))
		c.D[1] = uint32(size)
		for col := int(size); col >= 0; col-- {
			c.Word(1, uint16(col))
			packed := uint16(top+row)<<8 | uint16(uint8(left+col))
			c.Word(5, packed)
			c.Word(0, packed&0xc0c0)
			if uint16(c.D[0]) != 0 {
				continue
			}
			c.Byte(5, uint8(packed)*4)
			grid := 0xf44 + int(int16(uint16(c.D[5])))
			tile := m.byte(grid + 1)
			c.Byte(0, tile)
			geometry, e := r.byte(0x33512 + int(tile))
			if e != nil {
				return step, e
			}
			c.Byte(0, geometry)
			c.Word(0, uint16(c.D[0])&15)
			if uint16(c.D[0]) != 0 && uint8(c.D[0]) != 15 {
				c.Word(0, uint16(c.D[0])+0xe0)
				m.putByte(grid+1, uint8(c.D[0]))
			}
		}
		c.Word(1, 0xffff)
	}
	c.Word(2, 0xffff)
	if stage < 0x171e6 {
		m.putWord(at+20, m.word(at+20)+4)
		return step, m.err
	}
	m.putByte(at+22, 0x32)
	attempts, e := r.word(0x171b2)
	if e != nil {
		return step, e
	}
	c.Word(3, attempts)
	for counter := int(attempts); counter >= 0; counter-- {
		c.Word(3, uint16(counter))
		c.Byte(0, m.byte(at+6))
		c.Byte(1, m.byte(at+8))
		c.Byte(2, m.byte(at+12))
		saved3 := uint16(c.D[3])
		if e := cb.FireColumn(c); e != nil {
			return step, e
		}
		c.Word(3, saved3)
	}
	c.Word(3, 0xffff)
	origin := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	c.Word(2, origin)
	for cursor := 0x17156; ; cursor += 6 {
		offset, e := r.word(cursor)
		if e != nil {
			return step, e
		}
		c.Word(1, offset)
		if offset == 0xff9d {
			break
		}
		packed := origin + offset
		c.Word(1, packed)
		c.Word(0, packed&0xc0c0)
		if uint16(c.D[0]) != 0 {
			continue
		}
		c.Byte(1, uint8(packed)*4)
		grid := 0xf44 + int(int16(uint16(c.D[1])))
		tile := m.byte(grid + 1)
		c.Byte(0, tile)
		geometry, e := r.byte(0x33512 + int(tile))
		if e != nil {
			return step, e
		}
		c.Byte(0, geometry)
		c.Word(0, uint16(c.D[0])&15)
		shape, e := r.byte(cursor + 3)
		if e != nil {
			return step, e
		}
		if uint8(c.D[0]) == shape {
			paint, e := r.byte(cursor + 5)
			if e != nil {
				return step, e
			}
			m.putByte(grid+1, paint)
		}
	}
	c.D[0] = uint32(cb.Random())
	modulus, e := r.word(0x17188)
	if e != nil {
		return step, e
	}
	if e := frameDivide(c, 0, modulus); e != nil {
		return step, e
	}
	c.Swap(0)
	c.Word(4, uint16(c.D[0]))
	c.Word(5, uint16(c.D[2]))
	count := uint16(c.D[4])
	for counter := int(count); counter >= 0; counter-- {
		c.Word(4, uint16(counter))
		c.D[0] = uint32(cb.Random())
		if e := frameDivide(c, 0, 32); e != nil {
			return step, e
		}
		c.Swap(0)
		c.Word(0, uint16(c.D[0])&0xfc)
		cursor := 0x17136 + int(int16(uint16(c.D[0])))
		offset, e := r.word(cursor)
		if e != nil {
			return step, e
		}
		c.Word(1, offset+uint16(c.D[5]))
		c.Byte(2, m.byte(at+12))
		direction, e := r.word(cursor + 2)
		if e != nil {
			return step, e
		}
		c.Word(3, direction)
		if e := cb.Lava(c); e != nil {
			return step, e
		}
	}
	c.Word(4, 0xffff)
	m.putWord(at+20, 0)
	return step, m.err
}
