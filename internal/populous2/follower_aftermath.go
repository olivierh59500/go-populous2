package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type FollowerAftermathRules struct {
	RuinTimer       uint16
	Raster          [256]uint8
	NeighborOffsets []uint16
	Frames          map[int]AnimationFrame
	Terminals       map[int]bool
}

func DecodeFollowerAftermathRules(exe *amiga.Executable) (FollowerAftermathRules, error) {
	var rules FollowerAftermathRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return rules, fmt.Errorf("native aftermath tables missing")
	}
	code := exe.Hunks[0].Data
	rules.RuinTimer = binary.BigEndian.Uint16(code[0x20d72:])
	copy(rules.Raster[:], code[0x33512:0x33612])
	for at := 0x20bb6; at < 0x20cb6; at += 2 {
		offset := binary.BigEndian.Uint16(code[at:])
		if offset == 0xff9d {
			break
		}
		rules.NeighborOffsets = append(rules.NeighborOffsets, offset)
	}
	if len(rules.NeighborOffsets) != 4 {
		return rules, fmt.Errorf("native destruction-neighbor table unterminated")
	}
	towns, err := DecodeTownCombatRules(exe)
	if err != nil {
		return rules, err
	}
	rules.Frames = towns.Frames
	rules.Terminals = make(map[int]bool)
	for terminal := range towns.Loops {
		rules.Terminals[terminal] = true
	}
	prepass, err := DecodeCommonPrepassRules(exe)
	if err != nil {
		return rules, err
	}
	starts := []int{0x1e74, 0x9d4, 0x1b2c, 0x1cd0, 0x178, 0xf10, 0x7f4, 0x7dc, 0x7cc, 0x7d4}
	for _, table := range [][6]uint16{prepass.Swimming, prepass.Fatal, prepass.Swamp} {
		for _, pointer := range table {
			if pointer != 0 {
				starts = append(starts, int(pointer))
			}
		}
	}
	for _, pointer := range starts {
		frames, err := DecodeAnimation(exe, pointer)
		if err != nil {
			return FollowerAftermathRules{}, err
		}
		for index, frame := range frames {
			rules.Frames[pointer+index*4] = frame
		}
		rules.Terminals[pointer+len(frames)*4] = true
	}
	return rules, nil
}

func aftermathHandler(state uint8) uint8 {
	switch state {
	case 8, 0x2c, 0x2e:
		return 8 // $1199e: retained animation followed by complete cleanup0.
	case 0x18, 0x20, 0x32, 0x38, 0x3e, 0x40:
		return 0x18 // $11e00: terminal clears leader/owner/map, no second loss.
	case 0x1a, 0x2a, 0x42:
		return 0x42 // $11ce8: terminal resumes ordinary state2 next update.
	case 0x28, 0x30:
		return state
	default:
		return 0
	}
}

type FollowerAftermathCallbacks struct {
	Read  func(NativeRecordReference) (FollowerEntryActor, error)
	Write func(NativeRecordReference, FollowerEntryActor) error
	// Prepass is $12c3c at the initial $112b8 dispatch. The caller may omit
	// it only when it has already executed that same prepass. A changed state
	// is re-read and handed back if its handler is outside this controller.
	Prepass     func(NativeRecordReference) error
	Head        func(NativePackedTile) (NativeRecordReference, error)
	Tile        func(NativePackedTile) (uint8, error)
	DestroyTown func(NativeRecordReference) error
	// ClearLeader is $140ae, including the original $13fe4 magnet relocation.
	ClearLeader func(NativeRecordReference) error
	// Unlink is $125da, not cleanup $124a2. Retained deaths were already
	// accounted for when created; their terminal must not charge statistics.
	Unlink  func(NativeRecordReference) error
	Cleanup func(NativeRecordReference, uint16) error
}

type FollowerAftermathStep struct {
	Handled, Prepassed, CurrentTotal, NextFollower, Removed bool
	// Search is deferred for winner$42: terminal becomes state2 and ends
	// this update at $123b4. Hero$24 remains an external native handler.
	DeferredSearch bool
}

func (r *FollowerAftermathRules) nextAnimation(animation int) (int, bool, error) {
	if _, ok := r.Frames[animation]; !ok {
		return 0, false, fmt.Errorf("native aftermath animation unknown")
	}
	next := int(uint16(animation + 4))
	if r.Terminals[next] {
		return next, true, nil
	}
	if _, ok := r.Frames[next]; !ok {
		return 0, false, fmt.Errorf("native aftermath animation outside bank")
	}
	return next, false, nil
}

// DamageNeighbors translates $173b0's four cardinal cells and linked-record
// scan. Owner and population do not filter targets. Type2 nonheroes become
// type6/state8 with zero population; type4 calls complete $16184; type$16
// changes only kind to$1e and animation to$f10. No generic area damage is used.
func (r *FollowerAftermathRules) DamageNeighbors(reference NativeRecordReference, cb FollowerAftermathCallbacks) error {
	if r == nil || cb.Read == nil || cb.Write == nil || cb.Head == nil {
		return fmt.Errorf("native destruction-neighbor callbacks missing")
	}
	source, err := cb.Read(reference)
	if err != nil {
		return err
	}
	origin := uint16(source.Motion.Y)&0xff00 | uint16(source.Motion.X)>>8
	for _, offset := range r.NeighborOffsets {
		packed := origin + offset
		if packed&0xc0c0 != 0 {
			continue
		}
		ref, err := cb.Head(NativePackedTile(packed))
		if err != nil {
			return err
		}
		seen := map[NativeRecordReference]bool{}
		for ref != 0 {
			if seen[ref] {
				return fmt.Errorf("cyclic native destruction-neighbor chain")
			}
			seen[ref] = true
			a, err := cb.Read(ref)
			if err != nil {
				return err
			}
			if a.Motion.Kind == 2 && a.Motion.Flags&2 == 0 {
				a.Motion.Kind, a.Motion.State, a.Motion.Animation, a.Motion.Population = 6, 8, 0x178, 0
				if err := cb.Write(ref, a); err != nil {
					return err
				}
			}
			if a.Motion.Kind == 4 {
				if cb.DestroyTown == nil {
					return fmt.Errorf("native neighbor town destruction missing")
				}
				if err := cb.DestroyTown(ref); err != nil {
					return err
				}
			}
			if a.Motion.Kind == 0x16 {
				a.Motion.Kind, a.Motion.Animation = 0x1e, 0xf10
				if err := cb.Write(ref, a); err != nil {
					return err
				}
			}
			// Native reads +2 after the target routine, retaining any genuine
			// link changes instead of advancing through a stale local copy.
			a, err = cb.Read(ref)
			if err != nil {
				return err
			}
			ref = NativeRecordReference(a.Motion.Next)
		}
	}
	return nil
}

func aftermathPacked(a FollowerEntryActor) NativePackedTile {
	return NativePackedTile(uint16(a.Motion.Y)&0xff00 | uint16(a.Motion.X)>>8)
}

func aftermathRemove(reference NativeRecordReference, cb FollowerAftermathCallbacks) error {
	a, err := cb.Read(reference)
	if err != nil {
		return err
	}
	if a.Motion.Flags&1 != 0 {
		if cb.ClearLeader == nil {
			return fmt.Errorf("native terminal leader relocation missing")
		}
		if err := cb.ClearLeader(reference); err != nil {
			return err
		}
		a, err = cb.Read(reference)
		if err != nil {
			return err
		}
	}
	a.Owner, a.Motion.Population = 0, 0
	if err := cb.Write(reference, a); err != nil {
		return err
	}
	if cb.Unlink == nil {
		return fmt.Errorf("native terminal unlink missing")
	}
	return cb.Unlink(reference)
}

// Tick owns states$18/$40/$42/$28/$30 and the type6 neighbor's state8. The
// common prepass may replace them before dispatch; no hero$24 decision is
// substituted. It reports the exact $123b4 versus $12462 return boundary.
func (r *FollowerAftermathRules) Tick(reference NativeRecordReference, cb FollowerAftermathCallbacks) (FollowerAftermathStep, error) {
	var step FollowerAftermathStep
	if r == nil || cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native aftermath callbacks missing")
	}
	if cb.Prepass != nil {
		if err := cb.Prepass(reference); err != nil {
			return step, err
		}
		step.Prepassed = true
	}
	a, err := cb.Read(reference)
	if err != nil {
		return step, err
	}
	family := aftermathHandler(a.Motion.State)
	if family == 0 {
		return step, nil
	}
	step.Handled = true
	if a.Motion.State == 0x30 {
		return r.tickRuin(reference, cb, step)
	}
	if a.Motion.State == 0x28 {
		a.Motion.Timer = int16(r.RuinTimer)
		if err := cb.Write(reference, a); err != nil {
			return step, err
		}
		if err := r.DamageNeighbors(reference, cb); err != nil {
			return step, err
		}
		a, err = cb.Read(reference)
		if err != nil {
			return step, err
		}
		a.Motion.Population = 0
	}
	next, terminal, err := r.nextAnimation(a.Motion.Animation)
	if err != nil {
		return step, err
	}
	if !terminal {
		a.Motion.Animation = next
		if err := cb.Write(reference, a); err != nil {
			return step, err
		}
	} else {
		switch family {
		case 0x18:
			if err := aftermathRemove(reference, cb); err != nil {
				return step, err
			}
			step.Removed = true
		case 8:
			if cb.Cleanup == nil {
				return step, fmt.Errorf("native neighbor terminal cleanup missing")
			}
			if err := cb.Cleanup(reference, 0); err != nil {
				return step, err
			}
			step.Removed = true
		case 0x42:
			a.Motion.Kind, a.Motion.State, a.Motion.Animation = 2, 2, 0
			if err := cb.Write(reference, a); err != nil {
				return step, err
			}
			step.DeferredSearch = true
		case 0x28:
			a.Motion.State = 0x30
			if err := cb.Write(reference, a); err != nil {
				return step, err
			}
		}
	}
	if a.Motion.State == 0x28 || a.Motion.State == 0x30 {
		return r.tickRuin(reference, cb, step)
	}
	step.CurrentTotal = family == 0x42 || a.Motion.State == 2 || family == 8 && !terminal
	step.NextFollower = !step.CurrentTotal
	return step, nil
}

func (r *FollowerAftermathRules) tickRuin(reference NativeRecordReference, cb FollowerAftermathCallbacks, step FollowerAftermathStep) (FollowerAftermathStep, error) {
	a, err := cb.Read(reference)
	if err != nil {
		return step, err
	}
	previous := a.Motion.Timer
	a.Motion.Timer = int16(uint16(previous) - 1)
	if err := cb.Write(reference, a); err != nil {
		return step, err
	}
	remove := previous <= 1 // SUBI.W/BLE includes the signed-overflow case.
	if !remove {
		if cb.Tile == nil {
			return step, fmt.Errorf("native ruin tile callback missing")
		}
		tile, err := cb.Tile(aftermathPacked(a))
		if err != nil {
			return step, err
		}
		remove = r.Raster[tile]&15 != 15
	}
	if remove {
		if err := aftermathRemove(reference, cb); err != nil {
			return step, err
		}
		step.Removed = true
	}
	step.NextFollower = true
	return step, nil
}
