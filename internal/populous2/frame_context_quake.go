package populous2

import "fmt"

type NativeFrameQuakeCallbacks struct {
	Memory FollowerCleanupMemory
	Random func() uint16
	Lower  func(*NativeFrameRegisterContext) error
	// Create executes $165da and returns its original A1 BSS address. Clipped
	// coordinates retain the parent address; an exhausted pool returns $e740.
	Create func(int, *NativeFrameRegisterContext) (int, error)
	Sound  func() error
}

func frameCameraCue(at int, c *NativeFrameRegisterContext, m *nativeWhirlwindMemory, sound func() error) error {
	c.RestoreWord(0, m.word(0x5f44))
	c.RestoreWord(1, m.word(0x5f46))
	dx := int(int8(uint8(c.D[0]))) - int(int8(m.byte(at+6)))
	c.Byte(0, uint8(dx))
	if dx > 0 {
		return m.err
	}
	dy := int(int8(uint8(c.D[1]))) - int(int8(m.byte(at+8)))
	c.Byte(1, uint8(dy))
	if dy > 0 {
		return m.err
	}
	c.Byte(0, uint8(c.D[0])+8)
	if int(int8(uint8(dx)))+8 <= 0 {
		return m.err
	}
	c.Byte(1, uint8(c.D[1])+8)
	if int(int8(uint8(dy)))+8 <= 0 {
		return m.err
	}
	if sound == nil {
		return fmt.Errorf("native frame camera cue callback missing")
	}
	return sound()
}

func (r *NativeCommandRules) frameQuakeSurface(at int, c *NativeFrameRegisterContext, cb NativeFrameQuakeCallbacks) (int, error) {
	m := nativeWhirlwindMemory{m: cb.Memory}
	y, x := m.word(at+8), m.byte(at+6)
	packed := y&0xff00 | uint16(x*4)
	c.Word(4, packed)
	grid := 0xf44 + int(int16(packed))
	tile := m.byte(grid + 1)
	c.D[3] = uint32(tile)
	if tile >= 0xac && tile <= 0xc4 {
		c.D[0] = 0
		return grid, m.err
	}
	raster, e := r.byte(0x33512 + int(tile))
	if e != nil {
		return grid, e
	}
	c.Byte(3, raster)
	c.Word(3, uint16(c.D[3])&15)
	if uint16(c.D[3]) == 15 {
		m.putByte(grid+1, m.byte(at+11)+0xac)
		c.D[0] = 0
		return grid, m.err
	}
	packed = y&0xff00 | uint16(uint8(y)+x)
	packed = packed&0xff00 | uint16(uint8(packed)*4)
	c.Word(4, packed)
	grid = 0xf44 + int(int16(packed))
	c.Byte(5, m.byte(grid))
	c.Word(5, uint16(c.D[5])&7)
	tile = m.byte(grid + 1)
	c.D[0] = uint32(tile)
	raster, e = r.byte(0x33512 + int(tile))
	if e != nil {
		return grid, e
	}
	if raster&1 != 0 {
		c.Word(5, uint16(c.D[5])+1)
	}
	if uint16(c.D[5]) == 0 {
		m.putByte(at+12, 0)
		c.D[0] = 0xffffffff
		return grid, m.err
	}
	if uint16(c.D[5]) != 1 {
		c.D[0], c.D[1] = uint32(x), uint32(uint8(y>>8))
		saved5 := c.D[5]
		if m.err != nil {
			return grid, m.err
		}
		if e := cb.Lower(c); e != nil {
			return grid, e
		}
		c.D[5] = saved5
	}
	c.Word(0, uint16(c.D[5]))
	return grid, m.err
}

// TickFrameQuake translates $152ea/$153a4/$153f0 and complete $16694.
// Cracks remain in the raw terrain after owner-only expiry. Branch allocation
// retains signed source indices and its original returned-pointer comparison.
func (r *NativeCommandRules) TickFrameQuake(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameQuakeCallbacks) (NativeFrameFXStep, error) {
	step := NativeFrameFXStep{Draw: true, Color: 5}
	if r == nil || c == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Lower == nil || cb.Create == nil {
		return step, fmt.Errorf("native Earthquake frame callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	state := m.byte(at + 22)
	if state != 0x22 && state != 0x24 && state != 0x26 {
		return step, fmt.Errorf("native Earthquake frame state%02x missing", state)
	}
	if e := frameCameraCue(at, c, &m, cb.Sound); e != nil {
		return step, e
	}
	life := m.word(at + 24)
	m.putWord(at+24, life-1)
	if state != 0x26 && int16(life) <= 1 {
		fade, e := r.word(0x20d6c)
		if e != nil {
			return step, e
		}
		m.putWord(at+24, fade)
		m.putByte(at+22, 0x26)
		state = 0x26
		if e := frameCameraCue(at, c, &m, cb.Sound); e != nil {
			return step, e
		}
		life = m.word(at + 24)
		m.putWord(at+24, life-1)
	}
	if state == 0x26 && int16(life) <= 1 {
		m.putByte(at+12, 0)
		step.Draw = false
		return step, m.err
	}
	if state == 0x24 {
		_, e := r.frameQuakeSurface(at, c, cb)
		return step, e
	}
	timer := m.word(at + 20)
	m.putWord(at+20, timer-1)
	if int16(timer) > 1 {
		return step, m.err
	}
	m.putByte(at+21, m.byte(at+18))
	grid, e := r.frameQuakeSurface(at, c, cb)
	if e != nil {
		return step, e
	}
	if state == 0x26 {
		if uint16(c.D[0]) != 0 {
			return step, m.err
		}
		descriptor := m.word(at + 10)
		next, e := r.byte(0x20eb6 + int(int16(descriptor)))
		if e != nil {
			return step, e
		}
		c.Byte(0, next)
		if uint16(c.D[0]) != descriptor {
			m.putByte(at+11, next)
			c.Byte(0, uint8(c.D[0])+0xac)
			m.putWord(0xf2e, m.word(0xf2e)+1)
			m.putByte(grid+1, uint8(c.D[0]))
		}
		return step, m.err
	}
	height := int16(uint16(c.D[0]))
	if height < 0 || height > 1 {
		return step, m.err
	}
	m.putByte(at+22, 0x24)
	direction := m.byte(at + 26)
	table := 0x20e36 + int(direction)*8
	c.D[0] = uint32(cb.Random())
	if e := frameDivide(c, 0, 6); e != nil {
		return step, e
	}
	c.Swap(0)
	c.D[0] = uint32(int32(int16(uint16(c.D[0]))))
	childDirection, e := r.byte(table + int(int16(uint16(c.D[0]))))
	if e != nil {
		return step, e
	}
	c.Byte(3, childDirection)
	dx, e := r.byte(table + 6)
	if e != nil {
		return step, e
	}
	dy, e := r.byte(table + 7)
	if e != nil {
		return step, e
	}
	c.Byte(0, dx+m.byte(at+6))
	c.Byte(1, dy+m.byte(at+8))
	c.Byte(2, m.byte(at+12))
	c.Word(4, m.word(at+24))
	if m.err != nil {
		return step, m.err
	}
	child, e := cb.Create(at, c)
	if e != nil {
		return step, e
	}
	if child >= at {
		m.putWord(child+24, m.word(child+24)+1)
	}
	return step, m.err
}
