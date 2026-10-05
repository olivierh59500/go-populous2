package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeTownFrameState retains distinct writable CODE words and the 50-word
// parcel scratch. The outer actor loop clears $13350; $13550 is the actual
// high-byte property cache used by $13538 and is not cleared there.
type NativeTownFrameState struct {
	OuterFlag13350 uint16
	Property13550  uint16
	Scratch136E8   [50]uint16
	MinimapVariant uint16
}

// NativeFollowerTownFrameRules reads original CODE with the selected LAND
// resource overlaid at $3365a, including signed offsets outside its 19 stages.
type NativeFollowerTownFrameRules struct{ code, land []byte }
type NativeFollowerTownFrameCallbacks struct {
	Memory FollowerCleanupMemory
	Frame  *NativeFrameRegisterContext
	State  *NativeTownFrameState
	// Insert performs the actual $125a0 linked-list mutation. The wrapper
	// supplies its exact D0/D1 continuation before invoking this callback.
	Insert func(NativeRecordReference) error
	// LandAI performs the complete $131cc creator with the supplied registers.
	LandAI func(*NativeFrameRegisterContext) error
}
type NativeFollowerTownFrameStep struct {
	Redispatch, Worked, PoolExhausted, RequestedLandAI bool
	Born                                               NativeRecordReference
	Continuation                                       uint32
}

func DecodeNativeFollowerTownFrameRules(exe *amiga.Executable, land []byte) (*NativeFollowerTownFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33886 || len(land) != 556 {
		return nil, fmt.Errorf("native town frame CODE/LAND missing")
	}
	return &NativeFollowerTownFrameRules{code: exe.Hunks[0].Data, land: append([]byte(nil), land...)}, nil
}
func (r *NativeFollowerTownFrameRules) byte(at int, s *NativeTownFrameState) (byte, error) {
	if r == nil || s == nil || at < 0 || at >= len(r.code) {
		return 0, fmt.Errorf("native town CODE read %x outside retained bytes", at)
	}
	if at >= 0x3365a && at < 0x3365a+len(r.land) {
		return r.land[at-0x3365a], nil
	}
	if at == 0x13350 {
		return byte(s.OuterFlag13350 >> 8), nil
	}
	if at == 0x13351 {
		return byte(s.OuterFlag13350), nil
	}
	if at == 0x13550 {
		return byte(s.Property13550 >> 8), nil
	}
	if at == 0x13551 {
		return byte(s.Property13550), nil
	}
	if at >= 0x136e8 && at < 0x1374c {
		index := (at - 0x136e8) / 2
		if at&1 == 0 {
			return byte(s.Scratch136E8[index] >> 8), nil
		}
		return byte(s.Scratch136E8[index]), nil
	}
	return r.code[at], nil
}
func (r *NativeFollowerTownFrameRules) word(at int, s *NativeTownFrameState) (uint16, error) {
	if at&1 != 0 {
		return 0, fmt.Errorf("native town odd CODE word %x", at)
	}
	hi, err := r.byte(at, s)
	if err != nil {
		return 0, err
	}
	lo, err := r.byte(at+1, s)
	return uint16(hi)<<8 | uint16(lo), err
}
func townFrameValid(cb NativeFollowerTownFrameCallbacks) bool {
	return winMemoryValid(cb.Memory) && cb.Frame != nil && cb.State != nil
}

// ClearFarms is $135ca, including adjacent-table reads for the native stage
// byte. Its MOVEM saves all eight complete data registers, including errors
// reported by the portable retained-memory boundary.
func (r *NativeFollowerTownFrameRules) ClearFarms(ref NativeRecordReference, replacement uint8, cb NativeFollowerTownFrameCallbacks) error {
	if !townFrameValid(cb) {
		return fmt.Errorf("native town farm-clear callbacks missing")
	}
	saved := cb.Frame.D
	defer func() { cb.Frame.D = saved }()
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	farm := uint8(47)
	if m.byte(at+12) != 1 {
		farm = 63
	}
	stage := m.byte(at + 1)
	count, err := r.byte(0x13654+int(stage), cb.State)
	if err != nil {
		return err
	}
	origin := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	for i := 0; i <= int(count); i++ {
		offset, err := r.word(0x13684+i*2, cb.State)
		if err != nil {
			return err
		}
		if offset == 0xff9d {
			break
		}
		parcel := origin + offset
		if parcel&0xc0c0 != 0 {
			continue
		}
		scaled := parcel&0xff00 | uint16(uint8(parcel)*4)
		grid := 0xf44 + int(int16(scaled))
		overlay := 0x4f44 + int(scaled>>2)
		m.putByte(overlay, 0)
		tile := m.byte(grid + 1)
		if tile == farm {
			property, err := r.word(0x33312+int(tile)*2, cb.State)
			if err != nil {
				return err
			}
			if property&16 == 0 {
				m.putByte(grid+1, replacement)
			}
		}
	}
	return m.err
}

// Evaluate preserves the actual $13352 eight-register continuation. Cache
// hits retain D3's incoming high byte/word; recomputation owns raw farms,
// competitor demotion, overlays and persistent property/scratch writes.
func (r *NativeFollowerTownFrameRules) Evaluate(ref NativeRecordReference, cb NativeFollowerTownFrameCallbacks) error {
	if !townFrameValid(cb) {
		return fmt.Errorf("native town evaluator frame callbacks missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	savedD4 := uint16(c.D[4])
	// MOVEM.W restores D4 by sign-extending the saved low word.
	defer c.RestoreWord(4, savedD4)
	c.Byte(3, m.byte(at+1))
	recompute := uint8(c.D[3]) == 0
	if !recompute {
		c.D[0] = uint32(ref)
		if err := frameDivide(c, 0, 52); err != nil {
			return err
		}
		c.Word(0, uint16(c.D[0])&3)
		c.Word(1, m.word(0xf42))
		c.Word(1, uint16(c.D[1])&3)
		recompute = uint16(c.D[1]) == uint16(c.D[0])
		if !recompute {
			c.Word(0, m.word(at+8))
			c.Byte(0, m.byte(at+6))
			c.Byte(0, uint8(c.D[0])*4)
			c.Byte(0, m.byte(0xf45+int(int16(uint16(c.D[0])))))
			c.Word(0, uint16(c.D[0])&0xff)
			c.Word(0, uint16(c.D[0])*2)
			property, err := r.word(0x33312+int(int16(uint16(c.D[0]))), cb.State)
			if err != nil {
				return err
			}
			c.Word(0, property)
			c.Word(0, uint16(c.D[0])&7)
			recompute = uint16(c.D[0]) == 0
		}
	}
	if !recompute {
		c.Word(0, uint16(c.D[3]))
		return m.err
	}
	c.Word(4, 47)
	c.Word(5, 0x17)
	c.Word(6, 2)
	if m.byte(at+12) != 1 {
		c.Word(6, 1)
		c.Word(4, 63)
	}
	c.Word(2, m.word(at+8))
	c.Byte(2, m.byte(at+6))
	c.D[3], c.D[7] = 0, 0
	footprint, pending := 0, 0
	visit := func(offset uint16, mutate bool) (bool, error) {
		c.Word(1, offset)
		c.Word(1, uint16(c.D[1])+uint16(c.D[2]))
		c.Word(0, uint16(c.D[1]))
		c.Word(0, uint16(c.D[0])&0xc0c0)
		if uint16(c.D[0]) != 0 {
			return false, nil
		}
		c.Byte(1, uint8(c.D[1])*4)
		grid := 0xf44 + int(int16(uint16(c.D[1])))
		c.Byte(0, m.byte(grid+1))
		if mutate {
			c.Word(0, uint16(c.D[0])*2)
		} else {
			c.Byte(0, uint8(c.D[0])*2)
		}
		property, err := r.word(0x33312+int(int16(uint16(c.D[0]))), cb.State)
		if err != nil {
			return false, err
		}
		c.Word(0, property)
		if mutate {
			cb.State.Property13550 = uint16(c.D[0])
		}
		c.Word(0, uint16(c.D[0])&uint16(c.D[5]))
		if uint16(c.D[0]) == 0 || c.D[0]&16 != 0 {
			return false, nil
		}
		c.Word(0, m.word(grid+2))
		seen := map[uint16]bool{}
		for uint16(c.D[0]) != 0 {
			other := uint16(c.D[0])
			if seen[other] {
				return false, fmt.Errorf("cyclic native town chain")
			}
			seen[other] = true
			otherAt := cleanupRecordAddress(NativeRecordReference(other))
			if mutate && m.byte(otherAt) == 4 && otherAt != at {
				savedD2 := c.D[2]
				c.Word(2, 15)
				if err := r.ClearFarms(NativeRecordReference(other), 15, cb); err != nil {
					return false, err
				}
				c.D[2] = savedD2
				m.putByte(otherAt, 2)
				m.putByte(otherAt+22, 2)
				m.putWord(otherAt+10, 0)
			}
			if m.byte(otherAt) == 0x18 {
				return false, nil
			}
			c.Word(0, m.word(otherAt+2))
		}
		if mutate {
			if byte(cb.State.Property13550>>8)&(1<<uint(uint16(c.D[6])&7)) != 0 {
				return false, nil
			}
			if m.byte(grid+1) != uint8(c.D[4]) {
				if pending >= len(cb.State.Scratch136E8) {
					return false, fmt.Errorf("native town parcel scratch overflow")
				}
				cb.State.Scratch136E8[pending] = uint16(c.D[1])
				pending++
			}
			c.Word(3, uint16(c.D[3])+1)
		}
		return true, m.err
	}
	pass := func(counter int) error {
		c.D[7] = uint32(counter)
		for i := 0; i <= counter; i++ {
			offset, err := r.word(0x13684+footprint*2, cb.State)
			if err != nil {
				return err
			}
			footprint++
			if _, err := visit(offset, true); err != nil {
				return err
			}
			c.Word(7, uint16(c.D[7])-1)
		}
		return nil
	}
	if err := pass(0); err != nil {
		return err
	}
	if uint16(c.D[3]) == 0 {
		c.Word(0, uint16(c.D[3]))
		return m.err
	}
	if err := pass(7); err != nil {
		return err
	}
	if int16(uint16(c.D[3])) >= 9 {
		if err := pass(15); err != nil {
			return err
		}
		if int16(uint16(c.D[3])) >= 25 {
			probe := footprint
			clear := true
			for {
				offset, err := r.word(0x13684+probe*2, cb.State)
				if err != nil {
					return err
				}
				probe++
				c.Word(1, offset)
				if offset == 0xff9d {
					c.D[0] = 1
					break
				}
				ok, err := visit(offset, false)
				if err != nil {
					return err
				}
				if !ok {
					c.D[0] = 0
					clear = false
					break
				}
			}
			if clear {
				if err := pass(23); err != nil {
					return err
				}
				c.Word(3, 26)
			}
		}
	}
	stage, err := r.byte(0x13668+int(int16(uint16(c.D[3]))), cb.State)
	if err != nil {
		return err
	}
	c.Byte(3, stage)
	c.Byte(0, m.byte(at+1))
	if int8(uint8(c.D[3])) < int8(uint8(c.D[0])) {
		start, counter := 9, 39
		if int8(uint8(c.D[3])) >= 9 {
			start, counter = 25, 23
		}
		c.Word(7, uint16(counter))
		for i := 0; i <= counter; i++ {
			offset, err := r.word(0x13684+(start+i)*2, cb.State)
			if err != nil {
				return err
			}
			c.Word(1, offset+uint16(c.D[2]))
			c.Word(0, uint16(c.D[1])&0xc0c0)
			if uint16(c.D[0]) == 0 {
				c.Byte(1, uint8(c.D[1])*4)
				grid := 0xf44 + int(int16(uint16(c.D[1])))
				if m.byte(grid+1) == uint8(c.D[4]) {
					m.putByte(grid+1, 15)
				}
			}
			c.Word(7, uint16(c.D[7])-1)
		}
	}
	for pending > 0 {
		pending--
		c.Word(1, cb.State.Scratch136E8[pending])
		m.putByte(0xf45+int(int16(uint16(c.D[1]))), uint8(c.D[4]))
	}
	c.Word(0, uint16(c.D[3]))
	c.Word(0, uint16(c.D[0])<<3)
	structure := 0x1374c + int(int16(uint16(c.D[0])))
	for i := 0; ; i++ {
		value, err := r.byte(structure+i, cb.State)
		if err != nil {
			return err
		}
		c.Byte(4, value)
		offset, err := r.word(0x1382a+i*2, cb.State)
		if err != nil {
			return err
		}
		c.Word(1, offset)
		if offset == 0xff9d {
			break
		}
		c.Word(1, uint16(c.D[1])+uint16(c.D[2]))
		c.Word(0, uint16(c.D[1])&0xc0c0)
		if uint16(c.D[0]) != 0 {
			continue
		}
		c.Byte(1, uint8(c.D[1])*4)
		c.Word(1, uint16(c.D[1])>>2)
		m.putByte(0x4f44+int(int16(uint16(c.D[1]))), uint8(c.D[4]))
	}
	if int16(uint16(c.D[3])) > 0 && uint8(c.D[3]) != m.byte(at+1) {
		m.putByte(at+1, uint8(c.D[3]))
	}
	c.Word(0, uint16(c.D[3]))
	return m.err
}

// Tick is the complete state $06 body at $11738..$1199a. Its caller has already
// performed the common prepass. Failed town support returns $1131c for immediate
// search redispatch; other paths return $123b4 for the outer-loop epilogue.
func (r *NativeFollowerTownFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerTownFrameCallbacks) (NativeFollowerTownFrameStep, error) {
	step := NativeFollowerTownFrameStep{Continuation: 0x123b4}
	if !townFrameValid(cb) {
		return step, fmt.Errorf("native town frame callbacks missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	at := cleanupRecordAddress(ref)
	cb.State.MinimapVariant = 4
	c.Byte(4, m.byte(at+1))
	if err := r.Evaluate(ref, cb); err != nil {
		return step, err
	}
	if int16(uint16(c.D[0])) <= 0 {
		m.putByte(at, 2)
		m.putByte(at+22, 2)
		m.putWord(at+10, 0)
		c.Byte(2, 15)
		if err := r.ClearFarms(ref, 15, cb); err != nil {
			return step, err
		}
		step.Redispatch = true
		step.Continuation = 0x1131c
		return step, m.err
	}
	c.Byte(1, m.byte(at+12))
	c.ExtendWord(1)
	c.D[1] = uint32(uint16(c.D[1])) * 314
	god := 0xe76a + int(int16(uint16(c.D[1])))
	if uint8(c.D[4]) == 0 || int8(m.byte(at+1)) < 17 {
		c.Byte(0, m.byte(at+1))
		if uint8(c.D[0]) != m.byte(at+19) {
			c.D[0] = uint32(ref)
			m.putWord(god+0x2e, uint16(c.D[0]))
		}
	}
	c.D[2] = m.long(at + 26)
	if int16(uint16(c.D[2])) > int16(m.word(god+0x1c)) {
		m.putWord(god+0x1c, uint16(c.D[2]))
		c.D[0] = uint32(ref)
		m.putWord(god+0x1e, uint16(c.D[0]))
	}
	c.Word(2, m.word(at+46))
	if uint16(c.D[2]) <= m.word(god+0x20) {
		m.putWord(god+0x20, uint16(c.D[2]))
		c.D[0] = uint32(ref)
		m.putWord(god+0x22, uint16(c.D[0]))
	}
	m.putWord(god+0x24, m.word(god+0x24)+1)
	c.Word(3, uint16(c.D[3])*2)
	// A cached stage retains the incoming upper byte. The resulting signed
	// word can address adjacent CODE instead of the selected LAND resource.
	landAt := 0x3365a + int(int16(uint16(c.D[3])))
	readLand := func(offset int) (uint16, error) { return r.word(landAt+offset, cb.State) }
	c.D[0] = 0
	work, err := readLand(0xbe)
	if err != nil {
		return step, err
	}
	c.Word(0, work)
	m.putWord(at+20, m.word(at+20)+1)
	if uint16(c.D[0]) > m.word(at+20) {
		return step, m.err
	}
	m.putWord(at+20, 0)
	step.Worked = true
	c.D[0] = 0
	mana, err := readLand(0)
	if err != nil {
		return step, err
	}
	c.Word(0, mana)
	if m.byte(at+13)&16 == 0 {
		m.putLong(god, m.long(god)+c.D[0])
	}
	c.D[0] = 0
	growth, err := readLand(0x26)
	if err != nil {
		return step, err
	}
	c.Word(0, growth)
	c.D[0] += m.long(at + 26)
	c.D[3] = 0
	limit, err := readLand(0x4c)
	if err != nil {
		return step, err
	}
	c.Word(3, limit)
	forced := m.byte(at+13)&4 != 0
	m.putByte(at+13, m.byte(at+13)&^4)
	if !forced && int32(c.D[0]) <= int32(c.D[3]) || m.word(0xdc2) != 0 {
		m.putLong(at+26, c.D[0])
		return step, m.err
	}
	divisor, err := readLand(0x72)
	if err != nil {
		return step, err
	}
	if err := frameDivide(c, 0, divisor); err != nil {
		return step, err
	}
	// Native DIVU overflow leaves the dividend intact; AND.L still selects
	// its low word. frameDivide preserves this behavior rather than clamping.
	c.D[0] &= 0xffff
	m.putLong(at+26, m.long(at+26)-c.D[0])
	c.Word(2, 52)
	c.Word(1, 398)
	child := -1
	for slot := 0x76f4; slot < 0xc800; slot += 52 {
		if m.byte(slot+12) == 0 {
			child = slot
			break
		}
		c.Word(1, uint16(c.D[1])-1)
	}
	if child < 0 {
		m.putLong(at+26, m.long(at+26)+c.D[0])
		m.putWord(0xdc2, 1)
		step.PoolExhausted = true
		return step, m.err
	}
	c.Word(1, 25)
	for i := 0; i < 26; i++ {
		m.putWord(child+i*2, 0)
		c.Word(1, uint16(c.D[1])-1)
	}
	step.Born = NativeRecordReference(uint16(child - 0x76c0))
	if m.long(0xf36) == c.AddressBase+uint32(at) {
		m.putLong(0xf36, c.AddressBase+uint32(child))
	}
	m.putLong(child+26, c.D[0])
	m.putByte(child, 2)
	m.putByte(child+22, 2)
	m.putByte(child+12, m.byte(at+12))
	m.putLong(child+6, m.long(at+6))
	m.putLong(child+14, 0)
	m.putByte(child+18, m.byte(at+18))
	c.D[0] = uint32(step.Born)
	if err := frameDivide(c, 0, 52); err != nil {
		return step, err
	}
	c.Word(0, uint16(c.D[0])&14)
	m.putWord(child+50, uint16(c.D[0]))
	c.Byte(0, m.byte(at+1))
	m.putByte(child+25, uint8(c.D[0]))
	c.Byte(0, uint8(c.D[0])*2)
	m.putByte(child+24, uint8(c.D[0]))
	m.putWord(child+10, 0)
	m.putByte(child+1, 0)
	m.putByte(child+13, m.byte(at+13))
	m.putWord(child+48, m.word(at+48))
	m.putByte(at+13, m.byte(at+13)&0xfc)
	m.putByte(child+13, m.byte(child+13)&0x13)
	if m.byte(child+13) != 0 {
		if m.byte(child+13)&1 != 0 {
			c.D[0] = uint32(step.Born)
			m.putWord(god+8, uint16(c.D[0]))
		} else if m.byte(child+13)&2 != 0 {
			m.putWord(child+40, m.word(at+40))
			c.Word(0, m.word(at+50))
			m.putWord(at+50, m.word(child+50))
			m.putWord(child+50, uint16(c.D[0]))
			m.putWord(at+40, 0)
		}
	}
	c.D[0] = uint32(step.Born)
	if c.D[0] == 0x32c8 {
		c.D[0] = m.long(0xf40)
		if int32(c.D[0]) >= int32(m.long(0xdd8)) {
			if cb.LandAI == nil {
				return step, fmt.Errorf("native rare town birth $131cc callback missing")
			}
			saved := c.D
			c.D[0] += 50
			m.putLong(0xdd8, c.D[0])
			for draw := 0; draw < 2; draw++ {
				seed := m.long(0xeb28)
				if seed == 0 {
					seed = 12345678
				}
				seed *= 0xbb40e62d
				m.putLong(0xeb28, seed)
				c.D[0] = seed >> 8 & 0x7fff
				if draw == 0 {
					c.Word(0, uint16(c.D[0])&63)
				}
			}
			c.Word(1, uint16(c.D[0]))
			c.Word(1, uint16(c.D[1])&63)
			if err := frameDivide(c, 0, 12); err != nil {
				return step, err
			}
			c.Swap(0)
			c.D[0] &= ^uint32(1)
			c.Word(0, uint16(c.D[0])+2)
			c.Word(2, uint16(c.D[0]))
			err := cb.LandAI(c)
			c.D = saved
			if err != nil {
				return step, err
			}
			step.RequestedLandAI = true
		}
	}
	if cb.Insert == nil {
		return step, fmt.Errorf("native newborn $125a0 callback missing")
	}
	grid := 0xf44 + int(int16(m.word(child+8)&0xff00|uint16(m.byte(child+6)*4)))
	c.D[0] = uint32(step.Born)
	c.Word(1, m.word(grid+2))
	if err := cb.Insert(step.Born); err != nil {
		return step, err
	}
	return step, m.err
}

type nativeTownFrameMemory struct{ nativeWhirlwindMemory }

func (m *nativeTownFrameMemory) long(at int) uint32 {
	v, e := m.m.Read32(at)
	if e != nil && m.err == nil {
		m.err = e
	}
	return v
}
func (m *nativeTownFrameMemory) putLong(at int, value uint32) {
	if e := m.m.Write32(at, value); e != nil && m.err == nil {
		m.err = e
	}
}
