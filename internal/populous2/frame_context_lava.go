package populous2

import "fmt"

type NativeFrameLavaCallbacks struct {
	Memory              FollowerCleanupMemory
	Random              func() uint16
	Link, Unlink        func(NativeRecordReference) error
	Move                func(NativeRecordReference, *NativeFrameRegisterContext) error
	Basalt              func(*NativeFrameRegisterContext) error
	Scorch, DestroyTown func(NativeRecordReference) error
}

// CreateFrameLava is $16f20. Its MOVEM preserves D1-D7, including on terrain,
// pool and existing-lava rejection; only the exact signed result changes D0.
func (r *NativeCommandRules) CreateFrameLava(c *NativeFrameRegisterContext, cb NativeFrameLavaCallbacks) (failure error) {
	if r == nil || c == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Link == nil || cb.Basalt == nil {
		return fmt.Errorf("native Lava creation callbacks missing")
	}
	saved := c.D
	defer func() {
		for i := 1; i < 8; i++ {
			c.D[i] = saved[i]
		}
	}()
	m := nativeWhirlwindMemory{m: cb.Memory}
	packed := uint16(c.D[1])
	c.Word(0, packed&0xc0c0)
	if uint16(c.D[0]) != 0 {
		c.D[0] = 0
		return nil
	}
	grid := tsunamiGrid(packed)
	head := m.word(grid + 2)
	seen := map[uint16]bool{}
	for head != 0 {
		if seen[head] {
			return fmt.Errorf("cyclic native Lava admission chain")
		}
		seen[head] = true
		at := cleanupRecordAddress(NativeRecordReference(head))
		if m.byte(at) == 0x38 {
			c.D[0] = 0xffffffff
			return m.err
		}
		head = m.word(at + 2)
	}
	tile := m.byte(grid + 1)
	geometry, e := r.byte(0x33512 + int(tile))
	if e != nil {
		return e
	}
	shape := geometry & 15
	if shape == 0 {
		c.D[0] = uint32(uint8(packed))
		c.Word(1, packed>>8)
		life, e := r.word(0x172a2)
		if e != nil {
			return e
		}
		c.Word(4, life)
		if e := cb.Basalt(c); e != nil {
			return e
		}
		c.D[0] = 1
		return m.err
	}
	at, e := primitiveFreeRecord(m.m, 0xc800, 0xe740, 32)
	if e != nil {
		return e
	}
	if at == 0 {
		c.D[0] = 0
		return nil
	}
	m.putByte(at+6, uint8(packed))
	m.putWord(at+8, packed)
	animation, e := r.word(0x1718a + int(shape)*2)
	if e != nil {
		return e
	}
	if animation == 0 {
		if e := m.m.Write32(at+6, 0); e != nil {
			return e
		}
		c.D[0] = 0xffffffff
		return nil
	}
	if int16(animation) < 0 {
		animation, e = r.word(0x171aa + int(int16(uint16(c.D[3]))))
		if e != nil {
			return e
		}
	}
	m.putWord(at+10, animation)
	m.putByte(at+12, uint8(c.D[2]))
	m.putByte(at+7, 0)
	m.putByte(at+9, 0)
	m.putWord(at+26, uint16(c.D[3]))
	m.putByte(at, 0x38)
	bits := cb.Random()
	modulus, e := r.word(0x171b4)
	if e != nil {
		return e
	}
	if modulus == 0 {
		return fmt.Errorf("native Lava delay divisor zero")
	}
	delay := bits%modulus + 1
	m.putWord(at+20, delay)
	m.putWord(at+24, delay)
	m.putByte(at+22, 0x34)
	if m.err != nil {
		return m.err
	}
	if e := cb.Link(NativeRecordReference(uint16(at - 0x76c0))); e != nil {
		return e
	}
	c.D[0] = 1
	return nil
}

func (r *NativeCommandRules) frameLavaPush(ref NativeRecordReference, direction uint16, c *NativeFrameRegisterContext, cb NativeFrameLavaCallbacks) error {
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	kind := m.byte(at)
	c.D[0] = uint32(kind)
	branch, e := r.word(0x17038 + int(kind))
	if e != nil {
		return e
	}
	c.Word(0, branch)
	switch 0x17038 + int(int16(branch)) {
	case 0x1707a:
		if m.byte(at+22) != 0x3c {
			if e := cb.DestroyTown(ref); e != nil {
				return e
			}
			m.putByte(at+22, 0x3c)
		}
	case 0x17092:
		state := m.byte(at + 22)
		if state == 0x3a {
			return m.err
		}
		if state != 0x3c {
			animation := uint16(0x564)
			if m.byte(at+13)&2 != 0 {
				animation, e = r.word(0x20a18 + int(int16(m.word(at+40))))
				if e != nil {
					return e
				}
				c.Word(0, animation)
				if animation == 0 {
					return m.err
				}
			}
			m.putWord(at+10, animation)
			m.putByte(at+22, 0x3c)
		}
	case 0x170fc:
		m.putByte(at, 0x1e)
		m.putWord(at+10, 0xf10)
	case 0x1710a, 0x170cc:
	case 0x1710c, 0x1710e, 0x17122:
		return m.err
	default:
		return fmt.Errorf("native Lava kind%02x dispatch%05x missing", kind, 0x17038+int(int16(branch)))
	}
	c.RestoreWord(6, m.word(at+6))
	c.RestoreWord(7, m.word(at+8))
	vx, e := r.word(0x17126 + int(int16(direction))*2)
	if e != nil {
		return e
	}
	vy, e := r.word(0x17128 + int(int16(direction))*2)
	if e != nil {
		return e
	}
	speed, e := r.word(0x17124)
	if e != nil {
		return e
	}
	c.D[0], c.D[1] = uint32(vx)*uint32(speed), uint32(vy)*uint32(speed)
	c.Word(2, speed)
	x := m.word(at+6) + uint16(c.D[0])
	c.Word(6, x)
	if int16(x) >= 0 {
		c.Word(7, m.word(at+8)+uint16(c.D[1]))
	}
	y := uint16(c.D[7])
	if int16(x) < 0 || int16(y) < 0 || int16(x) >= 0x4000 || int16(y) >= 0x4000 {
		if kind == 0x14 {
			return m.err
		}
		m.putByte(at+12, 0)
		if m.err != nil {
			return m.err
		}
		return cb.Unlink(ref)
	}
	if m.err != nil {
		return m.err
	}
	return cb.Move(ref, c)
}

// TickFrameLava translates $1576a through the linked $17022 dispatch. It reads
// each next reference after pushing, retaining the original mutable traversal.
func (r *NativeCommandRules) TickFrameLava(ref NativeRecordReference, c *NativeFrameRegisterContext, cb NativeFrameLavaCallbacks) (NativeFrameFXStep, error) {
	step := NativeFrameFXStep{Draw: true, Color: 5}
	if r == nil || c == nil || !winMemoryValid(cb.Memory) || cb.Unlink == nil || cb.Move == nil || cb.Scorch == nil || cb.DestroyTown == nil {
		return step, fmt.Errorf("native Lava frame callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	direction := m.word(at + 26)
	finish := func() (NativeFrameFXStep, error) {
		m.putByte(at+22, 0x36)
		m.putByte(at+12, 0)
		if m.err != nil {
			return step, m.err
		}
		return step, cb.Unlink(ref)
	}
	timer := m.word(at + 20)
	m.putWord(at+20, timer-1)
	if timer == 1 {
		packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
		c.Word(1, packed)
		c.Byte(2, m.byte(at+12))
		c.Word(3, direction)
		c.Word(0, direction)
		offset, e := r.word(0x171b6 + int(int16(direction)))
		if e != nil {
			return step, e
		}
		c.Word(1, packed+offset)
		if e := r.CreateFrameLava(c, cb); e != nil {
			return step, e
		}
		if c.D[0] != 0 {
			delay, e := r.word(0x171b4)
			if e != nil {
				return step, e
			}
			c.Word(0, delay*2)
			m.putWord(at+20, uint16(c.D[0]))
		}
	}
	packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	c.Word(1, packed)
	c.Word(2, packed)
	c.Byte(1, uint8(packed)*4)
	grid := 0xf44 + int(int16(uint16(c.D[1])))
	tile := m.byte(grid + 1)
	c.D[0] = uint32(tile)
	geometry, e := r.byte(0x33512 + int(tile))
	if e != nil {
		return step, e
	}
	c.Byte(0, geometry)
	c.Word(0, (uint16(c.D[0])&15)*2)
	animation, e := r.word(0x1718a + int(uint16(c.D[0])))
	if e != nil {
		return step, e
	}
	c.Word(0, animation)
	if animation == 0 {
		return finish()
	}
	if int16(animation) < 0 {
		animation, e = r.word(0x171aa + int(int16(direction)))
		if e != nil {
			return step, e
		}
		c.Word(0, animation)
	}
	m.putWord(at+10, animation)
	c.Word(0, m.word(0xf42)<<2)
	c.Word(0, (uint16(c.D[0])+uint16(grid)+uint16(c.AddressBase))&4)
	m.putWord(at+10, m.word(at+10)+uint16(c.D[0]))
	life := m.word(at + 24)
	m.putWord(at+24, life-1)
	if int16(life) <= 1 {
		if e := cb.Scorch(ref); e != nil {
			return step, e
		}
		m.putWord(at+24, 3)
		offset, e := r.word(0x171be + int(int16(direction)))
		if e != nil {
			return step, e
		}
		back := packed + offset
		c.Word(2, back)
		c.Word(0, back&0xc0c0)
		if uint16(c.D[0]) != 0 {
			return finish()
		}
		c.Byte(2, uint8(back)*4)
		backGrid := 0xf44 + int(int16(uint16(c.D[2])))
		if m.byte(backGrid+1) != 0xdc {
			head := m.word(backGrid + 2)
			c.Word(0, head)
			found := false
			seen := map[uint16]bool{}
			for head != 0 {
				if seen[head] {
					return step, fmt.Errorf("cyclic native Lava rear chain")
				}
				seen[head] = true
				target := cleanupRecordAddress(NativeRecordReference(head))
				if m.byte(target) == 0x38 {
					found = true
					break
				}
				head = m.word(target + 2)
				c.Word(0, head)
			}
			if !found {
				return finish()
			}
		}
	}
	head := m.word(grid + 2)
	c.Word(0, head)
	visits := 0
	for head != 0 {
		visits++
		if visits > 65536 {
			return step, fmt.Errorf("native Lava linked traversal does not terminate")
		}
		target := NativeRecordReference(head)
		if e := r.frameLavaPush(target, direction, c, cb); e != nil {
			return step, e
		}
		head = m.word(cleanupRecordAddress(target) + 2)
		c.Word(0, head)
	}
	return step, m.err
}
