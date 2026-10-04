package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

// FollowerRuinVictimRules covers state$46's direct $12074 dispatch. Its
// creator retains owner/allocation with population0 and the static $2c34
// image. Unlike the state$28 town body, this handler resets no timer, advances
// no animation and performs no neighbor destruction or farm cleanup.
type FollowerRuinVictimRules struct {
	Raster [256]uint8
	Frame  AnimationFrame
}

func DecodeFollowerRuinVictimRules(exe *amiga.Executable) (FollowerRuinVictimRules, error) {
	var rules FollowerRuinVictimRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return rules, fmt.Errorf("native retained victim tables missing")
	}
	copy(rules.Raster[:], exe.Hunks[0].Data[0x33512:0x33612])
	frames, err := DecodeAnimation(exe, 0x2c34)
	if err != nil {
		return rules, err
	}
	if len(frames) != 1 {
		return rules, fmt.Errorf("native retained victim image is not static")
	}
	rules.Frame = frames[0]
	return rules, nil
}

type FollowerRuinVictimCallbacks struct {
	Memory FollowerCleanupMemory
	// The initial $12c3c prepass may replace state$46 or remove a record.
	// A town's farm cleanup belongs there, not in this countdown handler.
	Prepass     func(NativeRecordReference) error
	Tile        func(NativePackedTile) (uint8, error)
	ClearLeader func(NativeRecordReference) error
	Unlink      func(NativeRecordReference) error
}

type FollowerRuinVictimStep struct {
	Handled, Prepassed, Removed, NextFollower bool
}

func (r *FollowerRuinVictimRules) Tick(reference NativeRecordReference, cb FollowerRuinVictimCallbacks) (FollowerRuinVictimStep, error) {
	var step FollowerRuinVictimStep
	m := cb.Memory
	if r == nil || m.Read8 == nil || m.Read16 == nil || m.Write8 == nil || m.Write16 == nil || m.Write32 == nil {
		return step, fmt.Errorf("native retained victim callbacks missing")
	}
	if cb.Prepass != nil {
		if err := cb.Prepass(reference); err != nil {
			return step, err
		}
		step.Prepassed = true
	}
	source := cleanupRecordAddress(reference)
	state, err := m.Read8(source + 22)
	if err != nil {
		return step, err
	}
	if state != 0x46 {
		return step, nil
	}
	step.Handled, step.NextFollower = true, true
	previous, err := m.Read16(source + 20)
	if err != nil {
		return step, err
	}
	if err := m.Write16(source+20, previous-1); err != nil {
		return step, err
	}
	remove := int16(previous) <= 1 // SUBI.W/BLE keeps the signed-overflow case.
	if !remove {
		if cb.Tile == nil {
			return step, fmt.Errorf("native retained victim tile callback missing")
		}
		x, err := m.Read8(source + 6)
		if err != nil {
			return step, err
		}
		y, err := m.Read16(source + 8)
		if err != nil {
			return step, err
		}
		tile, err := cb.Tile(NativePackedTile(y&0xff00 | uint16(x)))
		if err != nil {
			return step, err
		}
		remove = r.Raster[tile]&15 != 15
	}
	if !remove {
		return step, nil
	}
	flags, err := m.Read8(source + 13)
	if err != nil {
		return step, err
	}
	if flags&1 != 0 {
		if cb.ClearLeader == nil {
			return step, fmt.Errorf("native retained victim leader relocation missing")
		}
		if err := cb.ClearLeader(reference); err != nil {
			return step, err
		}
	}
	if err := m.Write8(source+12, 0); err != nil {
		return step, err
	}
	if err := m.Write32(source+26, 0); err != nil {
		return step, err
	}
	if cb.Unlink == nil {
		return step, fmt.Errorf("native retained victim unlink missing")
	}
	if err := cb.Unlink(reference); err != nil {
		return step, err
	}
	step.Removed = true
	return step, nil
}
