package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeFireColumnRules keeps the native packed roulette table and signed
// animation markers. Victim damage and scorching share the original Storm
// helpers; a fire column does not invoke the tree's later neighbor burn.
type NativeFireColumnRules struct {
	Neighbors [16]uint16
	Vectors   [16][2]int16
	Storm     StormRules
	code      []byte
}

func DecodeNativeFireColumnRules(exe *amiga.Executable) (NativeFireColumnRules, error) {
	var r NativeFireColumnRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native fire column tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.Storm, err = DecodeStormRules(exe)
	if err != nil {
		return NativeFireColumnRules{}, err
	}
	for i := range r.Neighbors {
		r.Neighbors[i] = binary.BigEndian.Uint16(r.code[0x20dd6+i*2:])
		r.Vectors[i] = [2]int16{int16(binary.BigEndian.Uint16(r.code[0x20df6+i*4:])), int16(binary.BigEndian.Uint16(r.code[0x20df8+i*4:]))}
	}
	return r, nil
}

type NativeFireColumnCallbacks struct {
	Memory FollowerCleanupMemory
	Random func() uint16
	Move   func(NativeRecordReference, uint16, uint16) (bool, error)
	Unlink func(NativeRecordReference) error
	// DestroyTown runs $16184, including full farm clearing and mode1 cleanup.
	// Its wrapper restores the incoming source and victim register contexts.
	DestroyTown func(NativeRecordReference) error
}

type NativeFireColumnStep struct {
	Advanced, Routed, Moved, Expired, Water, Removed bool
	DamageScans, RandomDraws                         int
	Hits                                             uint16
}

func (r *NativeFireColumnRules) advance(m *nativeWhirlwindMemory, at int, loop bool) (bool, error) {
	next := m.word(at+10) + 4
	address := 0x23d1a + int(int16(next))
	if m.err != nil {
		return false, m.err
	}
	if address < 0 || address&1 != 0 || address+2 > len(r.code) {
		return false, fmt.Errorf("native fire column animation outside retained CODE: %x", address)
	}
	marker := int16(binary.BigEndian.Uint16(r.code[address:]))
	if marker < 0 {
		if !loop {
			return false, nil
		}
		next += uint16(marker)
	}
	m.putWord(at+10, next)
	return true, m.err
}

// route translates $14926. Only raw header altitude participates; raster
// corners are ignored. Equal-altitude neighbors alone shift the RNG bits.
func (r *NativeFireColumnRules) route(at int, cb NativeFireColumnCallbacks, step *NativeFireColumnStep) error {
	m := nativeWhirlwindMemory{m: cb.Memory}
	origin := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	height := uint16(m.byte(nativeWhirlwindGrid(origin)) & 7)
	if m.err != nil {
		return m.err
	}
	bits := cb.Random()
	step.RandomDraws++
	start, selected := int(bits&0x0e)/2, uint16(0)
	for i := 0; i < 8; i++ {
		parcel := origin + r.Neighbors[start+i]
		if parcel&0xc0c0 != 0 {
			continue
		}
		candidate := uint16(m.byte(nativeWhirlwindGrid(parcel)) & 7)
		if candidate < height {
			continue
		}
		if candidate == height {
			accept := bits&1 != 0
			bits >>= 1
			if !accept {
				continue
			}
		}
		height, selected = candidate, uint16((start+i)*4)
	}
	if selected == 0 {
		selected = bits & 0x3c
	}
	speed := m.byte(at + 18)
	if m.err != nil {
		return m.err
	}
	vector := r.Vectors[selected/4]
	m.putWord(at+14, uint16(int32(vector[0])*int32(speed)))
	m.putWord(at+16, uint16(int32(vector[1])*int32(speed)))
	if m.err != nil {
		return m.err
	}
	if speed == 0 {
		return fmt.Errorf("native fire column route division by zero")
	}
	m.putWord(at+20, uint16(255/uint16(speed))*2)
	step.Routed = true
	return m.err
}

// Tick translates $14890/$148bc/$149fa, preserving recycled record bytes.
// Active updates inspect the old cell for water before rerouting/moving, then
// scorch and damage only the new cell's actual linked actors. Dead walkers
// retain owner and graph membership; their later follower state removes them.
func (r *NativeFireColumnRules) Tick(ref NativeRecordReference, cb NativeFireColumnCallbacks) (NativeFireColumnStep, error) {
	var step NativeFireColumnStep
	if r == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Move == nil || cb.Unlink == nil || cb.DestroyTown == nil {
		return step, fmt.Errorf("native fire column controller callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	if m.byte(at+12) == 0 {
		return step, m.err
	}
	remove := func() error {
		m.putByte(at+12, 0)
		if m.err != nil {
			return m.err
		}
		step.Removed = true
		return cb.Unlink(ref)
	}
	ending := func() error {
		advanced, err := r.advance(&m, at, false)
		if err != nil {
			return err
		}
		step.Advanced = advanced
		if !advanced {
			return remove()
		}
		return m.err
	}
	state := m.byte(at + 22)
	if state == 2 {
		advanced, err := r.advance(&m, at, false)
		if err != nil {
			return step, err
		}
		if advanced {
			step.Advanced = true
			return step, nil
		}
		m.putWord(at+10, 0x4b8)
		m.putByte(at+22, 4)
		state = 4
	}
	if state == 4 {
		life := m.word(at + 24)
		m.putWord(at+24, life-1)
		// SUBI.W/BGT honors overflow, so the original signed $8000 expires.
		if int16(life) <= 1 {
			step.Expired = true
			m.putByte(at+22, 6)
			m.putWord(at+10, 0x660)
			err := ending()
			return step, err
		}
		advanced, err := r.advance(&m, at, true)
		if err != nil {
			return step, err
		}
		step.Advanced = advanced
		grid, err := stormCell(cb.Memory, at)
		if err != nil {
			return step, err
		}
		if r.Storm.Properties[m.byte(grid+1)]&8 != 0 {
			step.Water = true
			m.putByte(at+22, 6)
			m.putWord(at+10, 0x660)
			err = ending()
			return step, err
		}
		timer := m.word(at + 20)
		m.putWord(at+20, timer-1)
		if m.err != nil {
			return step, m.err
		}
		if int16(timer) <= 1 {
			if err := r.route(at, cb, &step); err != nil {
				return step, err
			}
		}
		x, y := m.word(at+6)+m.word(at+14), m.word(at+8)+m.word(at+16)
		if m.err != nil {
			return step, m.err
		}
		if int16(x) < 0 || int16(y) < 0 || int16(x) >= 0x4000 || int16(y) >= 0x4000 {
			err = remove()
			return step, err
		}
		if _, err := cb.Move(ref, x, y); err != nil {
			return step, err
		}
		step.Moved = true
		if err := r.Storm.Scorch(ref, cb.Memory); err != nil {
			return step, err
		}
		step.DamageScans++
		step.Hits, err = r.Storm.Damage(ref, StormCallbacks{Memory: cb.Memory, DestroyTown: cb.DestroyTown})
		return step, err
	}
	if state == 6 {
		err := ending()
		return step, err
	}
	return step, fmt.Errorf("unknown native fire column state %02x", state)
}
