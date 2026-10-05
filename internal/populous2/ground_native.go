package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeGroundRules struct {
	FontCount, SwampCount     uint16
	FontOffsets, SwampOffsets [45]uint16
	Properties                [256]uint16
	Prepass                   CommonPrepassRules
	Terrain                   FollowerTerrainRules
	Aftermath                 FollowerAftermathRules
}

func DecodeNativeGroundRules(exe *amiga.Executable) (NativeGroundRules, error) {
	var r NativeGroundRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return r, fmt.Errorf("native ground tables missing")
	}
	code := exe.Hunks[0].Data
	r.FontCount = binary.BigEndian.Uint16(code[0x21002:])
	r.SwampCount = binary.BigEndian.Uint16(code[0x20fa6:])
	if r.FontCount == 0 || r.SwampCount == 0 {
		return NativeGroundRules{}, fmt.Errorf("native ground count division by zero")
	}
	for i := range r.FontOffsets {
		r.FontOffsets[i] = binary.BigEndian.Uint16(code[0x21004+i*2:])
		r.SwampOffsets[i] = binary.BigEndian.Uint16(code[0x20fa8+i*2:])
	}
	for i := range r.Properties {
		r.Properties[i] = binary.BigEndian.Uint16(code[0x33312+i*2:])
	}
	var err error
	r.Prepass, err = DecodeCommonPrepassRules(exe)
	if err != nil {
		return NativeGroundRules{}, err
	}
	r.Terrain, err = DecodeFollowerTerrainRules(exe)
	if err != nil {
		return NativeGroundRules{}, err
	}
	r.Aftermath, err = DecodeFollowerAftermathRules(exe)
	if err != nil {
		return NativeGroundRules{}, err
	}
	return r, nil
}

type NativeGroundCallbacks struct {
	Memory        FollowerCleanupMemory
	Random        func() uint16
	SourceD2Upper uint16 // Incoming upper word when the XP/MULU branch is skipped.
	Prepass       CommonPrepassCallbacks
	Terrain       FollowerTerrainCallbacks
	Aftermath     FollowerAftermathCallbacks
}

type NativeGroundCreation struct {
	Attempts, RandomDraws, Written int
	Tiles                          []NativePackedTile
}

// Create executes $16938 or $169cc. Any nonzero raw occupancy head rejects a
// parcel, including a dead, neutral, scenery, wall or dangling reference.
// Repeated writes count as actual attempts; no generic radius is filled.
func (r *NativeGroundRules) Create(id SpellID, owner uint16, x, y uint8, cb NativeGroundCallbacks) (NativeGroundCreation, error) {
	step := NativeGroundCreation{Tiles: []NativePackedTile{}}
	if r == nil || !winMemoryValid(cb.Memory) || cb.Random == nil {
		return step, fmt.Errorf("native ground creator callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	base, mask, tile, offsets := uint16(0), uint16(0), uint8(0), [45]uint16{}
	prefix := uint32(cb.SourceD2Upper) << 16
	switch id {
	case Baptism:
		base, mask, tile, offsets = r.FontCount, 0x67, 143, r.FontOffsets
		if int16(owner) <= 2 {
			prefix = uint32(owner) * 314 & 0xffff0000
			base += uint16(m.byte(primitiveDeityAddress(owner)+0x57) >> 5)
		}
	case Swamp:
		base, mask, tile, offsets = r.SwampCount, 0x27, 168, r.SwampOffsets
		if int8(uint8(owner)) <= 2 {
			prefix = uint32(owner) * 314 & 0xffff0000
			// $169fe's DIVU stores quotient in the low word. ADD.W XP alters
			// that quotient only; SWAP then discards it before the DBF loop.
			_ = m.byte(primitiveDeityAddress(owner)+0x53) >> 5
		}
	default:
		return step, fmt.Errorf("unsupported native ground effect %d", id)
	}
	if m.err != nil {
		return step, m.err
	}
	bits := cb.Random()
	step.RandomDraws++
	if base == 0 {
		return step, fmt.Errorf("native ground count division by zero")
	}
	dividend := prefix | uint32(bits)
	remainder := uint16(dividend >> 16)
	// DIVU overflow leaves D2 unchanged. Its subsequent SWAP therefore uses
	// the preserved MULU upper word as the attempt count, rather than modulus.
	if dividend/uint32(base) <= 0xffff {
		remainder = uint16(dividend % uint32(base))
	}
	count := remainder + base/2
	origin := uint16(y)<<8 | uint16(x)
	for attempt := 0; attempt <= int(count); attempt++ {
		bits = cb.Random()
		step.RandomDraws++
		step.Attempts++
		packed := origin + offsets[(bits%90)/2]
		if packed&0xc0c0 != 0 {
			continue
		}
		grid := nativeWhirlwindGrid(packed)
		if m.word(grid+2) != 0 {
			if m.err != nil {
				return step, m.err
			}
			continue
		}
		current := m.byte(grid + 1)
		if m.err != nil {
			return step, m.err
		}
		if r.Properties[current]&mask == 0 {
			continue
		}
		m.putByte(grid+1, tile)
		if m.err != nil {
			return step, m.err
		}
		step.Written++
		step.Tiles = append(step.Tiles, NativePackedTile(packed))
	}
	return step, m.err
}

type NativeGroundFollowerStep struct {
	Prepass                                     CommonPrepassStep
	Terrain                                     FollowerTerrainStep
	Aftermath                                   FollowerAftermathStep
	State                                       uint8
	Handled, Search, CurrentTotal, NextFollower bool
}

// TickFollower composes the actual $112b8 terrain entry and its resulting
// handler once. Fonts center and retain kind14/state36 until animation ends;
// conversion then relocates the leader and changes owner without population
// damage. Swamps retain their death allocation until the native terminal.
// Ordinary search/hero/motion is returned to the main dispatcher unchanged.
func (r *NativeGroundRules) TickFollower(ref NativeRecordReference, cb NativeGroundCallbacks) (NativeGroundFollowerStep, error) {
	var step NativeGroundFollowerStep
	if r == nil || cb.Prepass.Read == nil {
		return step, fmt.Errorf("native ground follower callbacks missing")
	}
	var err error
	step.Prepass, err = r.Prepass.Tick(ref, cb.Prepass)
	if err != nil {
		return step, err
	}
	a, err := cb.Prepass.Read(ref)
	if err != nil {
		return step, err
	}
	step.State = a.Motion.State
	switch step.State {
	case 0x36:
		step.Handled = true
		step.Terrain, err = r.Terrain.TickConversion(ref, cb.Terrain)
	case 0x16:
		step.Handled = true
		step.Terrain, err = r.Terrain.TickWater(ref, cb.Terrain)
	case 0x3c:
		step.Handled = true
		step.Terrain, err = r.Terrain.TickBurning(ref, cb.Terrain)
	case 8, 0x18, 0x1a, 0x20, 0x28, 0x2a, 0x2c, 0x2e, 0x30, 0x32, 0x38, 0x3e, 0x40, 0x42:
		step.Handled = true
		aftermath := cb.Aftermath
		aftermath.Prepass = nil
		step.Aftermath, err = r.Aftermath.Tick(ref, aftermath)
		step.CurrentTotal, step.NextFollower, step.Search = step.Aftermath.CurrentTotal, step.Aftermath.NextFollower, step.Aftermath.DeferredSearch
		return step, err
	default:
		return step, nil
	}
	step.CurrentTotal, step.NextFollower, step.Search = step.Terrain.CurrentTotal, step.Terrain.NextFollower, step.Terrain.Search
	return step, err
}
