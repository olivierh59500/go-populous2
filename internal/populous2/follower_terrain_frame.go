package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type NativeFollowerTerrainFrameRules struct {
	code      []byte
	Aftermath NativeFollowerAftermathFrameRules
}

func DecodeNativeFollowerTerrainFrameRules(exe *amiga.Executable) (NativeFollowerTerrainFrameRules, error) {
	var r NativeFollowerTerrainFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native terrain follower frame CODE missing")
	}
	r.code = exe.Hunks[0].Data
	var e error
	r.Aftermath, e = DecodeNativeFollowerAftermathFrameRules(exe)
	return r, e
}

func (r *NativeFollowerTerrainFrameRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native terrain follower frame word outside CODE")
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}

type NativeFollowerTerrainFrameCallbacks struct {
	Memory               FollowerCleanupMemory
	Frame                *NativeFrameRegisterContext
	Cleanup, ClearLeader func(NativeRecordReference, *NativeFrameRegisterContext) error
	Move                 func(NativeRecordReference, *NativeFrameRegisterContext) error
	Unlink               func(NativeRecordReference) error
}

func (r *NativeFollowerTerrainFrameRules) advance(at int, loop bool, c *NativeFrameRegisterContext, m *nativeWhirlwindMemory) (bool, error) {
	c.Word(0, m.word(at+10)+4)
	marker, e := r.word(0x23d1a + int(int16(uint16(c.D[0]))))
	if e != nil {
		return false, e
	}
	if int16(marker) < 0 {
		if !loop {
			return false, m.err
		}
		c.Word(0, uint16(c.D[0])+marker)
	}
	m.putWord(at+10, uint16(c.D[0]))
	return true, m.err
}

// Tick translates the raw water16, conversion36 and burning3c controllers.
// The caller already ran the common prepass. Return boundaries are the actual
// search1131c, population/map123b4 and next-record12462 source continuations.
func (r *NativeFollowerTerrainFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerTerrainFrameCallbacks) (uint32, error) {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return 0, fmt.Errorf("native terrain follower frame backing missing")
	}
	m, c := nativeWhirlwindMemory{m: cb.Memory}, cb.Frame
	at := cleanupRecordAddress(ref)
	switch state := m.byte(at + 22); state {
	case 0x3c:
		if _, e := r.advance(at, true, c, &m); e != nil {
			return 0, e
		}
		population, e := m.m.Read32(at + 26)
		if e != nil {
			return 0, e
		}
		c.D[0] = population
		shift, e := r.word(0x20d54)
		if e != nil {
			return 0, e
		}
		c.Word(1, shift)
		c.D[0] >>= uint16(c.D[1]) & 63
		c.Word(0, uint16(c.D[0])+uint16(c.D[1]))
		damage := c.D[0]
		if e := m.m.Write32(at+26, population-damage); e != nil {
			return 0, e
		}
		if int64(int32(population))-int64(int32(damage)) <= 0 {
			if cb.Cleanup == nil {
				return 0, fmt.Errorf("native burning cleanup frame callback missing")
			}
			c.D[0] = 0
			if e := cb.Cleanup(ref, c); e != nil {
				return 0, e
			}
		}
		return 0x123b4, m.err
	case 0x36:
		advanced, e := r.advance(at, false, c, &m)
		if e != nil {
			return 0, e
		}
		if advanced {
			return 0x12462, m.err
		}
		if m.byte(at+13)&1 != 0 {
			if cb.ClearLeader == nil {
				return 0, fmt.Errorf("native conversion leader frame callback missing")
			}
			if e := cb.ClearLeader(ref, c); e != nil {
				return 0, e
			}
		}
		c.Byte(0, 2)
		if m.byte(at+12) != 1 {
			c.Byte(0, 1)
		}
		m.putByte(at+12, uint8(c.D[0]))
		m.putByte(at, 2)
		m.putByte(at+22, 2)
		m.putWord(at+10, 0)
		axis := func(reg, posAt, velocityAt int) {
			c.D[reg] = 1
			velocity := int16(m.word(at + velocityAt))
			if velocity <= 0 {
				value := uint8(0)
				if velocity < 0 {
					value = 0xff
				}
				c.Byte(reg, value)
			}
			c.Byte(reg, uint8(c.D[reg])+m.byte(at+posAt))
			cell := uint8(c.D[reg])
			if int8(cell) < 0 || int8(cell) >= 64 {
				c.Byte(reg, m.byte(at+posAt))
			}
			c.Word(reg, uint16(c.D[reg])<<8)
			c.Byte(reg, m.byte(at+posAt+1))
		}
		axis(6, 6, 14)
		axis(7, 8, 16)
		if m.err != nil {
			return 0, m.err
		}
		if cb.Move == nil {
			return 0, fmt.Errorf("native conversion move frame callback missing")
		}
		if e := cb.Move(ref, c); e != nil {
			return 0, e
		}
		return 0x1131c, nil
	case 0x16:
		if _, e := r.advance(at, true, c, &m); e != nil {
			return 0, e
		}
		owner := m.byte(at + 12)
		c.D[0] = uint32(owner)
		scenario := m.word(0xeb2c)
		if owner != 1 {
			scenario = m.word(0xeb2e)
		}
		c.Word(2, scenario)
		alive := false
		if scenario&0x20 == 0 {
			c.D[0] *= 314
			god := 0xe76a + int(int16(uint16(c.D[0])))
			population, e := m.m.Read32(at + 26)
			if e != nil {
				return 0, e
			}
			amount, e := m.m.Read32(god + 20)
			if e != nil {
				return 0, e
			}
			c.D[0] = population - amount
			alive = int64(int32(population))-int64(int32(amount)) > 0
		}
		if !alive {
			m.putByte(at+22, 0x2e)
			m.putWord(at+10, 0x196c)
			if m.byte(at+13)&2 != 0 {
				animation, e := r.word(0x20a90 + int(int16(m.word(at+40))))
				if e != nil {
					return 0, e
				}
				m.putWord(at+10, animation)
			}
			if m.err != nil {
				return 0, m.err
			}
			if cb.Cleanup == nil {
				return 0, fmt.Errorf("native water cleanup frame callback missing")
			}
			c.D[0] = 1
			if e := cb.Cleanup(ref, c); e != nil {
				return 0, e
			}
			flow, e := r.Aftermath.Tick(ref, NativeFollowerAftermathFrameCallbacks{Memory: cb.Memory, Frame: c, Cleanup: cb.Cleanup, ClearLeader: cb.ClearLeader, Unlink: cb.Unlink})
			if e != nil {
				return 0, e
			}
			if flow == NativeFollowerCount {
				return 0x123b4, nil
			}
			return 0x12462, nil
		}
		if e := m.m.Write32(at+26, c.D[0]); e != nil {
			return 0, e
		}
		packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
		c.Word(0, packed&0xff00|uint16(uint8(packed)*4))
		grid := 0xf44 + int(int16(uint16(c.D[0])))
		tile := m.byte(grid + 1)
		c.D[0] = uint32(tile)
		c.Word(0, uint16(c.D[0])*2)
		property, e := r.word(0x33312 + int(uint16(c.D[0])))
		if e != nil {
			return 0, e
		}
		if property&8 == 0 {
			m.putByte(at, 2)
			m.putWord(at+10, 0)
			m.putByte(at+22, 2)
			return 0x1131c, m.err
		}
		c.D[0] = uint32(m.byte(at + 12))
		c.ExtendWord(0)
		c.D[0] = uint32(uint16(c.D[0])) * 314
		god := 0xe76a + int(int16(uint16(c.D[0])))
		c.D[0] = uint32(int32(at - 0x76c0))
		m.putWord(god+0x36, uint16(c.D[0]))
		return 0x123b4, m.err
	default:
		return 0, fmt.Errorf("native terrain follower state%02x outside family", state)
	}
}
