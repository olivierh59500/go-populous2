package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFungusRules struct {
	CollectWait uint16
	BasePeriod  uint8
	Neighbors   [8]int16
	Properties  [256]uint16
	Ground      NativeGroundRules
}

func DecodeNativeFungusRules(exe *amiga.Executable) (NativeFungusRules, error) {
	var r NativeFungusRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return r, fmt.Errorf("native fungus tables missing")
	}
	code := exe.Hunks[0].Data
	r.CollectWait = binary.BigEndian.Uint16(code[0x20d50:])
	r.BasePeriod = code[0x20d53]
	for i := range r.Neighbors {
		r.Neighbors[i] = int16(binary.BigEndian.Uint16(code[0x20f28+i*2:]))
	}
	if binary.BigEndian.Uint16(code[0x20f38:]) != 0xff9d {
		return r, fmt.Errorf("native fungus neighbor terminator missing")
	}
	for i := range r.Properties {
		r.Properties[i] = binary.BigEndian.Uint16(code[0x33312+i*2:])
	}
	var err error
	r.Ground, err = DecodeNativeGroundRules(exe)
	return r, err
}

type NativeFungusCallbacks struct{ Memory FollowerCleanupMemory }
type NativeFungusCreation struct {
	Reference                          NativeRecordReference
	Planted, Created, Reused, PoolFull bool
}
type NativeFungusScratch struct {
	MinX, MaxX uint8
	MinY, MaxY uint16
}
type NativeFungusStep struct {
	Started, Aged, Generated, Finished              bool
	ReadOutsideGrid, WrittenOutsideGrid, TileWrites int
	Scratch                                         NativeFungusScratch
}

// Create is $15fda. Raw terrain eligibility precedes deity lookup and shared
// pool admission; planting survives pool exhaustion. A nonzero pending word
// reuses that raw address without checking its kind, owner or current phase.
func (r *NativeFungusRules) Create(owner uint16, x, y uint8, cb NativeFungusCallbacks) (NativeFungusCreation, error) {
	var step NativeFungusCreation
	if r == nil || !winMemoryValid(cb.Memory) {
		return step, fmt.Errorf("native fungus creator memory missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	grid := nativeWhirlwindGrid(uint16(y)<<8 | uint16(x))
	if r.Properties[m.byte(grid+1)]&0x27 == 0 {
		return step, m.err
	}
	m.putWord(0xf2e, m.word(0xf2e)+1)
	m.putByte(grid+1, 145)
	step.Planted = m.err == nil
	god := heroGodAddress(uint8(owner))
	pending := m.word(god + 14)
	if m.err != nil {
		return step, m.err
	}
	if pending != 0 {
		step.Reference, step.Reused = NativeRecordReference(pending), true
		at := cleanupRecordAddress(step.Reference)
		minX, minY := m.byte(at+6), m.byte(at+8)
		if x <= minX {
			m.putByte(at+6, x)
			m.putByte(at+26, m.byte(at+26)+minX-x)
		} else {
			n := x - minX + 2
			if n > m.byte(at+26) {
				m.putByte(at+26, n)
			}
		}
		if int8(y) < int8(minY) {
			m.putByte(at+8, y)
			m.putByte(at+27, m.byte(at+27)+minY-y)
		} else {
			n := y - minY + 2
			if n > m.byte(at+27) {
				m.putByte(at+27, n)
			}
		}
		m.putByte(at+7, m.byte(at+6))
		m.putByte(at+9, m.byte(at+8))
		m.putByte(at+28, m.byte(at+26))
		m.putByte(at+29, m.byte(at+27))
		return step, m.err
	}
	at, err := primitiveFreeRecord(cb.Memory, 0xc800, 0xe740, 32)
	if err != nil {
		return step, err
	}
	if at == 0 {
		step.PoolFull = true
		return step, nil
	}
	for _, w := range []struct {
		offset int
		value  uint8
	}{{12, uint8(owner)}, {6, x - 1}, {8, y - 1}, {7, x - 1}, {9, y - 1}, {28, 2}, {29, 2}, {26, 2}, {27, 2}} {
		m.putByte(at+w.offset, w.value)
	}
	m.putWord(at+20, r.CollectWait)
	m.putByte(at+18, r.BasePeriod)
	m.putByte(at+18, m.byte(at+18)-(m.byte(god+0x53)>>5))
	m.putByte(at, 0x26)
	m.putByte(at+22, 0x12)
	step.Reference = NativeRecordReference(uint16(at - 0x76c0))
	m.putWord(god+14, uint16(step.Reference))
	step.Created = m.err == nil
	return step, m.err
}

type nativeFungusSurface struct {
	m    *nativeWhirlwindMemory
	step *NativeFungusStep
}

func (s nativeFungusSurface) read(address int) uint8 {
	if address < 0xf44 || address >= 0x4f44 {
		s.step.ReadOutsideGrid++
	}
	return s.m.byte(address)
}
func (s nativeFungusSurface) write(address int, tile uint8, revision bool) {
	if address < 0xf44 || address >= 0x4f44 {
		s.step.WrittenOutsideGrid++
	}
	if revision {
		s.m.putWord(0xf2e, s.m.word(0xf2e)+1)
	}
	s.m.putByte(address, tile)
	s.step.TileWrites++
}

func (r *NativeFungusRules) age(at int, s nativeFungusSurface) error {
	x, y := s.m.byte(at+7), s.m.byte(at+9)
	start := nativeWhirlwindGrid(uint16(y)<<8 | uint16(x))
	dx, dy := s.m.byte(at+28), s.m.byte(at+29)
	for row := 0; row <= int(dy) && s.m.err == nil; row++ {
		for col := 0; col <= int(dx) && s.m.err == nil; col++ {
			address := start + row*256 + col*4 + 1
			tile := s.read(address)
			if tile > 144 && tile <= 148 {
				s.write(address, tile+1, true)
			} else if tile > 149 && tile <= 151 {
				next := tile + 1
				if next == 152 {
					next = 15
				}
				s.write(address, next, true)
			}
		}
	}
	return s.m.err
}

func (r *NativeFungusRules) generation(at int, s nativeFungusSurface) error {
	m := s.m
	start := nativeWhirlwindGrid(m.word(at+8)&0xff00 | uint16(m.byte(at+6)))
	dx, dy := m.byte(at+26), m.byte(at+27)
	scratch := NativeFungusScratch{MinX: 255, MinY: 0x4100, MaxY: 0xffff}
	for row := 0; row <= int(dy) && m.err == nil; row++ {
		for col := 0; col <= int(dx) && m.err == nil; col++ {
			address := start + row*256 + col*4
			properties := r.Properties[s.read(address+1)] & 0x37
			if properties == 0 {
				continue
			}
			neighbors := 0
			for _, delta := range r.Neighbors {
				if r.Properties[s.read(address+int(delta)+1)] == 0x10 {
					neighbors++
				}
			}
			if m.err != nil {
				return m.err
			}
			if properties&0x10 != 0 {
				if neighbors != 2 && neighbors != 3 {
					s.write(address+1, 150, false)
					continue
				}
			} else {
				if neighbors != 3 {
					continue
				}
				s.write(address+1, 145, false)
			}
			offset := uint16(address - 0xf44)
			x, y := uint8(offset), offset&0xff00
			if x <= scratch.MinX {
				scratch.MinX = x
			}
			if x > scratch.MaxX {
				scratch.MaxX = x
			}
			if int16(y) < int16(scratch.MinY) {
				scratch.MinY = y
			}
			if int16(y) > int16(scratch.MaxY) {
				scratch.MaxY = y
			}
		}
	}
	s.step.Scratch = scratch
	if m.err != nil {
		return m.err
	}
	if scratch.MinX == 255 {
		m.putByte(at+12, 0)
		s.step.Finished = true
		return m.err
	}
	oldX, oldY := m.byte(at+6), m.byte(at+8)
	newX := (scratch.MinX >> 2) - 1
	if int8(newX) < 0 {
		newX = 0
	}
	workX := newX
	if newX > oldX {
		workX = oldX
	}
	m.putByte(at+7, workX)
	m.putByte(at+6, newX)
	newDX := (scratch.MaxX >> 2) - newX + 1
	workDX := newDX
	if newDX <= dx {
		workDX = dx
	}
	m.putByte(at+28, workDX)
	m.putByte(at+26, newDX)
	if int8(newX+newDX) >= 64 {
		m.putByte(at+26, 63-newX)
	}
	newY := uint8(scratch.MinY>>8) - 1
	if int8(newY) < 0 {
		newY = 0
	}
	workY := newY
	if newY > oldY {
		workY = oldY
	}
	m.putByte(at+9, workY)
	m.putByte(at+8, newY)
	newDY := uint8(scratch.MaxY>>8) - newY + 1
	workDY := newDY
	if newDY <= dy {
		workDY = dy
	}
	m.putByte(at+29, workDY)
	m.putByte(at+27, newDY)
	// Native $150b0 subtracts X when clipping Y. Working extents are already
	// stored and remain independent of this current-rectangle typo.
	if int8(newY+newDY) >= 64 {
		m.putByte(at+27, 63-newX)
	}
	return m.err
}

// Tick owns $14e16/$14e52. Aging increments the original revision word;
// staged generation writes do not. Signed SUBI byte overflow controls the
// phase branch. Raw linear reads/writes may enter clock, overlay or actor BSS;
// unsupported memory spans return explicit errors instead of fabricated water.
func (r *NativeFungusRules) Tick(ref NativeRecordReference, cb NativeFungusCallbacks) (NativeFungusStep, error) {
	var step NativeFungusStep
	if r == nil || !winMemoryValid(cb.Memory) {
		return step, fmt.Errorf("native fungus controller memory missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	if m.byte(at+12) == 0 {
		return step, m.err
	}
	state := m.byte(at + 22)
	if state == 0x12 {
		god := heroGodAddress(m.byte(at + 12))
		if m.word(god+14) != 0 {
			wait := m.word(at + 20)
			m.putWord(at+20, wait-1)
			if int16(wait) > 1 {
				return step, m.err
			}
		}
		m.putWord(god+14, 0)
		m.putByte(at+22, 0x14)
		m.putWord(at+20, 0)
		m.putByte(at+21, m.byte(at+18))
		step.Started = true
		state = 0x14
	}
	if state != 0x14 {
		return step, fmt.Errorf("unknown native fungus phase %02x", state)
	}
	phase := m.byte(at + 21)
	m.putByte(at+21, phase-1)
	if m.err != nil {
		return step, m.err
	}
	surface := nativeFungusSurface{m: &m, step: &step}
	if int8(phase) <= 0 {
		m.putByte(at+21, m.byte(at+18))
		step.Generated = true
		err := r.generation(at, surface)
		return step, err
	}
	divisor := m.byte(at+18) / 3
	if m.err != nil {
		return step, m.err
	}
	if divisor == 0 {
		return step, fmt.Errorf("native fungus age division by zero")
	}
	if (phase-1)%divisor == 0 {
		step.Aged = true
		err := r.age(at, surface)
		return step, err
	}
	return step, m.err
}
