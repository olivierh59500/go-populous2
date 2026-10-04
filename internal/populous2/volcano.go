package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type VolcanoStage struct {
	Size   uint8
	Center [2]int8
}
type VolcanoOverlay struct {
	Offset      uint16
	Shape, Tile uint8
}
type VolcanoLavaSource struct{ Offset, Direction uint16 }

// VolcanoRules decodes the original command62/spell26 controller. Its
// allocation is unlinked and does not initialize the retained kind, fractions
// or other bytes in the shared32-byte slot.
type VolcanoRules struct {
	Stages                               []VolcanoStage
	Overlays                             []VolcanoOverlay
	LavaSources                          [8]VolcanoLavaSource
	Geometry                             [256]uint8
	FireColumnAttempts, LavaCountModulus uint16
}

func DecodeVolcanoRules(exe *amiga.Executable) (VolcanoRules, error) {
	var r VolcanoRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native volcano tables missing")
	}
	code := exe.Hunks[0].Data
	for at := 0x171c6; at <= 0x171e6; at += 4 {
		r.Stages = append(r.Stages, VolcanoStage{Size: code[at], Center: [2]int8{int8(code[at+1]), int8(code[at+2])}})
	}
	for at := 0x17156; ; at += 6 {
		if at+2 > 0x17188 {
			return VolcanoRules{}, fmt.Errorf("native volcano overlay sentinel missing")
		}
		offset := binary.BigEndian.Uint16(code[at:])
		if offset == 0xff9d {
			break
		}
		r.Overlays = append(r.Overlays, VolcanoOverlay{Offset: offset, Shape: code[at+3], Tile: code[at+5]})
	}
	for index := range r.LavaSources {
		at := 0x17136 + index*4
		r.LavaSources[index] = VolcanoLavaSource{binary.BigEndian.Uint16(code[at:]), binary.BigEndian.Uint16(code[at+2:])}
	}
	copy(r.Geometry[:], code[0x33512:0x33612])
	r.FireColumnAttempts = binary.BigEndian.Uint16(code[0x171b2:]) + 1
	r.LavaCountModulus = binary.BigEndian.Uint16(code[0x17188:])
	if len(r.Stages) != 9 || len(r.Overlays) != 8 || r.LavaCountModulus == 0 {
		return VolcanoRules{}, fmt.Errorf("native volcano table dimensions differ")
	}
	return r, nil
}

type VolcanoCallbacks struct {
	Memory         FollowerCleanupMemory
	FireExperience func(uint8) (uint8, error)
	Cell           func(NativePackedTile) (NativeOccupancyCell, error)
	WriteTile      func(NativePackedTile, uint8) error
	// Lower receives the original primitive arguments, including X64..255
	// from the crater's linear-index low byte. Those must not be masked63.
	Lower, Raise func(int, int) error
	// The final paint-loop DBF leaves D2.W=$ffff; the native eruption
	// replaces only its low owner byte, so the creator receives $ff01/$ff02.
	CreateFireColumn func(uint16, uint8, uint8) error
	CreateLava       func(uint8, NativePackedTile, uint16) error
	Random           func() uint16
}

type VolcanoStep struct {
	Finished, Erupted bool
	Stage             int
}

func volcanoAddress(ref NativeRecordReference) (int, error) {
	slot, ok := LocateNativeRecord(ref)
	if !ok || slot.Pool != NativeEffectPool {
		return 0, fmt.Errorf("native volcano reference outside effect pool")
	}
	return 0x76c0 + int(int16(ref)), nil
}

// Create translates $16cc8 and returns the first slot whose raw owner is0.
// It consumes no randomness and performs no terrain/admission check.
func (r *VolcanoRules) Create(owner, x, y uint8, cb VolcanoCallbacks) (NativeRecordReference, bool, error) {
	if r == nil || cb.Memory.Read8 == nil || cb.Memory.Write8 == nil || cb.Memory.Write16 == nil {
		return 0, false, fmt.Errorf("native volcano allocation callbacks missing")
	}
	for index := 0; index < NativeEffectCapacity; index++ {
		at := 0xc800 + index*32
		oldOwner, err := cb.Memory.Read8(at + 12)
		if err != nil {
			return 0, false, err
		}
		if oldOwner != 0 {
			continue
		}
		if err := cb.Memory.Write8(at+12, owner); err != nil {
			return 0, false, err
		}
		if err := cb.Memory.Write8(at+6, x); err != nil {
			return 0, false, err
		}
		if err := cb.Memory.Write8(at+8, y); err != nil {
			return 0, false, err
		}
		timer := uint16(12)
		if owner <= 2 {
			if cb.FireExperience == nil {
				return 0, false, fmt.Errorf("native volcano fire experience callback missing")
			}
			xp, err := cb.FireExperience(owner)
			if err != nil {
				return 0, false, err
			}
			timer = uint16(3-(xp>>6)) * 4
		}
		if err := cb.Memory.Write16(at+20, timer); err != nil {
			return 0, false, err
		}
		if err := cb.Memory.Write8(at+22, 0x30); err != nil {
			return 0, false, err
		}
		return NativeRecordReference(at - 0x76c0), true, nil
	}
	return 0, false, nil
}

// Tick translates $16d2c/$16e6e and the following state$32/$17446.
// Direct terrain operations and shared-pool child creators are explicit
// original callback boundaries. The controller itself is never map-linked.
func (r *VolcanoRules) Tick(ref NativeRecordReference, cb VolcanoCallbacks) (VolcanoStep, error) {
	var step VolcanoStep
	if r == nil || cb.Memory.Read8 == nil || cb.Memory.Read16 == nil || cb.Memory.Write8 == nil || cb.Memory.Write16 == nil {
		return step, fmt.Errorf("native volcano controller callbacks missing")
	}
	at, err := volcanoAddress(ref)
	if err != nil {
		return step, err
	}
	phase, err := cb.Memory.Read8(at + 22)
	if err != nil {
		return step, err
	}
	if phase == 0x32 {
		step.Finished = true
		return step, cb.Memory.Write8(at+12, 0)
	}
	if phase != 0x30 {
		return step, fmt.Errorf("native volcano phase outside controller")
	}
	timer, err := cb.Memory.Read16(at + 20)
	if err != nil {
		return step, err
	}
	if timer&3 != 0 || int(timer/4) >= len(r.Stages) {
		return step, fmt.Errorf("native volcano stage outside table")
	}
	step.Stage = int(timer / 4)
	x, err := cb.Memory.Read8(at + 6)
	if err != nil {
		return step, err
	}
	y, err := cb.Memory.Read8(at + 8)
	if err != nil {
		return step, err
	}
	if cb.Cell == nil || cb.WriteTile == nil || cb.Lower == nil || cb.Raise == nil {
		return step, fmt.Errorf("native volcano terrain callbacks missing")
	}
	size := int(r.Stages[step.Stage].Size)
	left, top := max(0, int(x)-size/2-1), max(0, int(y)-size/2-1)
	visit := func(f func(NativePackedTile, NativeOccupancyCell) error) error {
		for dy := size; dy >= 0; dy-- {
			for dx := size; dx >= 0; dx-- {
				packed := NativePackedTile(uint16(top+dy)<<8 | uint16(uint8(left+dx)))
				if uint16(packed)&0xc0c0 != 0 {
					continue
				}
				cell, err := cb.Cell(packed)
				if err != nil {
					return err
				}
				if err := f(packed, cell); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(func(p NativePackedTile, cell NativeOccupancyCell) error {
		height := cell.Header & 7
		if height == 0 && cell.Tile == 0 {
			return nil
		}
		linear := uint16(uint8(p)) + uint16(uint8(p>>8))*64
		for attempt := 0; attempt <= int(height); attempt++ {
			if err := cb.Lower(int(uint8(linear)), int(linear>>6)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return step, err
	}
	for _, stage := range r.Stages[:step.Stage+1] {
		for _, direction := range stage.Center {
			if direction > 0 {
				err = cb.Raise(int(x), int(y))
			} else if direction < 0 {
				err = cb.Lower(int(x), int(y))
			}
			if err != nil {
				return step, err
			}
		}
	}
	if err := visit(func(p NativePackedTile, cell NativeOccupancyCell) error {
		shape := r.Geometry[cell.Tile] & 15
		if shape == 0 || shape == 15 {
			return nil
		}
		return cb.WriteTile(p, 0xe0+shape)
	}); err != nil {
		return step, err
	}
	if step.Stage < len(r.Stages)-1 {
		return step, cb.Memory.Write16(at+20, timer+4)
	}
	if err := cb.Memory.Write8(at+22, 0x32); err != nil {
		return step, err
	}
	if err := r.erupt(ref, x, y, cb); err != nil {
		return step, err
	}
	step.Erupted = true
	return step, cb.Memory.Write16(at+20, 0)
}

func (r *VolcanoRules) erupt(ref NativeRecordReference, x, y uint8, cb VolcanoCallbacks) error {
	if cb.CreateFireColumn == nil || cb.CreateLava == nil || cb.Random == nil {
		return fmt.Errorf("native volcano eruption callbacks missing")
	}
	at, _ := volcanoAddress(ref)
	for attempt := 0; attempt < int(r.FireColumnAttempts); attempt++ {
		owner, err := cb.Memory.Read8(at + 12)
		if err != nil {
			return err
		}
		if err := cb.CreateFireColumn(0xff00|uint16(owner), x, y); err != nil {
			return err
		}
	}
	origin := uint16(y)<<8 | uint16(x)
	for _, overlay := range r.Overlays {
		packed := origin + overlay.Offset
		if packed&0xc0c0 != 0 {
			continue
		}
		cell, err := cb.Cell(NativePackedTile(packed))
		if err != nil {
			return err
		}
		if r.Geometry[cell.Tile]&15 == overlay.Shape {
			if err := cb.WriteTile(NativePackedTile(packed), overlay.Tile); err != nil {
				return err
			}
		}
	}
	attempts := int(cb.Random()%r.LavaCountModulus) + 1
	for attempt := 0; attempt < attempts; attempt++ {
		source := r.LavaSources[(cb.Random()%32)/4]
		owner, err := cb.Memory.Read8(at + 12)
		if err != nil {
			return err
		}
		if err := cb.CreateLava(owner, NativePackedTile(origin+source.Offset), source.Direction); err != nil {
			return err
		}
	}
	return nil
}
