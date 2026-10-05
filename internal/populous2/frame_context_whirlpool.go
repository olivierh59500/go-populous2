package populous2

import "fmt"

type NativeFrameWhirlpoolCallbacks struct {
	Memory FollowerCleanupMemory
	Random func() uint16
	// Lower is the complete direct $d7f0 body with its actual register return.
	Lower func(*NativeFrameRegisterContext) error
	// Sound increments the original mutable descriptor flag at CODE$18a8a.
	Sound func() error
}

// TickFrameWhirlpool translates $14cae-$14e12 against retained raw bytes.
// The coastline call saves D1-D6, while D0 and D7 retain the terrain return.
func (r WhirlpoolRules) TickFrameWhirlpool(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameWhirlpoolCallbacks) (NativeFrameFXStep, error) {
	step := NativeFrameFXStep{Draw: true, Color: 5}
	if c == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Lower == nil {
		return step, fmt.Errorf("native Whirlpool frame callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	c.RestoreWord(0, m.word(0x5f44))
	c.RestoreWord(1, m.word(0x5f46))
	dx := int(int8(uint8(c.D[0]))) - int(int8(m.byte(at+6)))
	c.Byte(0, uint8(dx))
	if dx <= 0 {
		dy := int(int8(uint8(c.D[1]))) - int(int8(m.byte(at+8)))
		c.Byte(1, uint8(dy))
		if dy <= 0 {
			c.Byte(0, uint8(c.D[0])+8)
			if int(int8(uint8(dx)))+8 > 0 {
				c.Byte(1, uint8(c.D[1])+8)
				if int(int8(uint8(dy)))+8 > 0 {
					if cb.Sound == nil {
						return step, fmt.Errorf("native Whirlpool descriptor callback missing")
					}
					if e := cb.Sound(); e != nil {
						return step, e
					}
				}
			}
		}
	}
	animation := m.word(at + 10)
	c.Word(1, animation)
	c.Word(4, animation)
	c.Word(1, animation+4)
	if int16(uint16(c.D[1])) >= 0xa7 {
		c.Word(1, 0x97)
	}
	m.putWord(at+10, uint16(c.D[1]))
	origin := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	c.Word(3, origin)
	for _, offset := range r.Footprint {
		c.Word(4, uint16(c.D[4])+1)
		parcel := origin + offset
		c.Word(2, parcel)
		c.Word(0, parcel&0xc0c0)
		if uint16(c.D[0]) != 0 {
			continue
		}
		c.Byte(2, uint8(parcel)*4)
		grid := 0xf44 + int(int16(uint16(c.D[2])))
		if uint8(c.D[4]) == m.byte(grid+1) {
			m.putByte(grid+1, 0)
		}
	}
	c.Word(4, uint16(c.D[4])+1)
	c.Word(2, 0xff9d)
	life := m.word(at + 24)
	m.putWord(at+24, life-1)
	if int16(life) <= 1 {
		m.putByte(at+12, 0)
		step.Draw = false
		return step, m.err
	}
	timer := m.word(at + 20)
	m.putWord(at+20, timer-1)
	if int16(timer) <= 1 {
		m.putByte(at+21, m.byte(at+18))
		c.D[0] = uint32(cb.Random())
		c.Word(0, uint16(c.D[0])&0x0e)
		origin += r.Neighbors[uint16(c.D[0])/2]
		c.Word(3, origin)
		c.Word(0, origin&0xc0c0)
		if uint16(c.D[0]) == 0 && m.byte(0xf45+int(int16(origin))) != 0xe0 {
			m.putByte(at+6, uint8(origin))
			m.putWord(at+8, origin)
			m.putByte(at+9, 0)
		}
	}
	origin = m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	c.Word(3, origin)
	c.D[5] = 0xfffffff8
	for i, offset := range r.Footprint {
		c.Word(1, uint16(c.D[1])+1)
		c.Word(5, uint16(c.D[5])+8)
		parcel := origin + offset
		c.Word(2, parcel)
		c.Word(0, parcel&0xc0c0)
		if uint16(c.D[0]) != 0 {
			continue
		}
		c.Byte(2, uint8(parcel)*4)
		grid := 0xf44 + int(int16(uint16(c.D[2])))
		c.Byte(0, m.byte(grid+1))
		c.Word(0, uint16(c.D[0])*2)
		if r.TileProperties[m.byte(grid+1)]&8 != 0 {
			m.putByte(grid+1, uint8(c.D[1]))
			continue
		}
		c.D[6] = 3
		cursor := i * 4
		for {
			if cursor >= len(r.CornerWords) {
				return step, fmt.Errorf("native Whirlpool corner CODE window exhausted")
			}
			vertex := origin + r.CornerWords[cursor]
			cursor++
			c.Word(0, vertex)
			c.Word(4, vertex&0xc0c0)
			if uint16(c.D[4]) != 0 {
				continue
			}
			saved := c.D
			c.Word(1, uint16(c.D[0])>>8)
			c.Word(0, uint16(c.D[0])&0xff)
			if e := cb.Lower(c); e != nil {
				return step, e
			}
			for reg := 1; reg <= 6; reg++ {
				c.D[reg] = saved[reg]
			}
			c.Word(6, uint16(c.D[6])-1)
			if uint16(c.D[6]) == 0xffff {
				break
			}
		}
		c.Byte(0, m.byte(grid+1))
		c.Word(0, uint16(c.D[0])&0xff)
		c.Word(0, uint16(c.D[0])*2)
		if r.TileProperties[m.byte(grid+1)]&8 != 0 {
			m.putByte(grid+1, uint8(c.D[1]))
		}
		return step, m.err
	}
	c.Word(1, uint16(c.D[1])+1)
	c.Word(5, uint16(c.D[5])+8)
	c.Word(2, 0xff9d)
	return step, m.err
}
