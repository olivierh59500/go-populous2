package populous2

import "fmt"

const NativeWorldFollowerCapacity = 399 // Native follower slot zero is reserved.

// NativeWorldOccupancyRecord keeps graph membership separate from an actor's
// population or active flag. Native death animations remain mapped until their
// final cleanup, and some active effect controllers are deliberately unmapped.
type NativeWorldOccupancyRecord struct {
	Record NativeOccupancyRecord
	Linked bool
}

// NativeWorldOccupancy is the persistent actor/marker map graph. Its links and
// full coordinate words are authoritative for map operations; a controller
// can propose a new position before Move without losing the old splice point.
// Followers uses Go index zero for native slot one, reference $0034.
type NativeWorldOccupancy struct {
	Grid      NativeOccupancyState
	Walls     [WallCapacity]NativeWorldOccupancyRecord
	Scenery   [SceneryCapacity]NativeWorldOccupancyRecord
	Followers [NativeWorldFollowerCapacity]NativeWorldOccupancyRecord
	Effects   [NativeEffectCapacity]NativeWorldOccupancyRecord
	Magnets   [NativeMagnetCount]NativeWorldOccupancyRecord
}

// NativeWorldReference converts a Go pool-array index to the original signed
// byte offset from $76c0. It never returns follower IDs or slot+1 effect IDs.
func NativeWorldReference(pool NativeRecordPool, index int) (NativeRecordReference, bool) {
	var start, stride, capacity int
	switch pool {
	case NativeWallPool:
		start, stride, capacity = -6000, 16, WallCapacity
	case NativeSceneryPool:
		start, stride, capacity = -2800, 14, SceneryCapacity
	case NativeFollowerPool:
		start, stride, capacity = 52, 52, NativeWorldFollowerCapacity
	case NativeEffectPool:
		start, stride, capacity = 20800, 32, NativeEffectCapacity
	case NativeMagnetPool:
		start, stride, capacity = NativeMagnetImageStart-0x76c0, NativeMagnetStride, NativeMagnetCount
	default:
		return 0, false
	}
	if index < 0 || index >= capacity {
		return 0, false
	}
	return NativeRecordReference(uint16(start + index*stride)), true
}

func (state *NativeWorldOccupancy) entry(reference NativeRecordReference) (*NativeWorldOccupancyRecord, bool) {
	if state == nil {
		return nil, false
	}
	if location, ok := LocateNativeMagnet(reference); ok {
		return &state.Magnets[location.Index], true
	}
	location, ok := LocateNativeRecord(reference)
	if !ok {
		return nil, false
	}
	switch location.Pool {
	case NativeWallPool:
		return &state.Walls[location.Index], true
	case NativeSceneryPool:
		return &state.Scenery[location.Index], true
	case NativeFollowerPool:
		return &state.Followers[location.Index-1], true
	case NativeEffectPool:
		return &state.Effects[location.Index], true
	}
	return nil, false
}

// Record includes inactive and unlinked pool members. Activity belongs to the
// actor controller and must not hide records during native unlink operations.
func (state *NativeWorldOccupancy) Record(reference NativeRecordReference) (NativeOccupancyRecord, bool) {
	entry, ok := state.entry(reference)
	if !ok {
		return NativeOccupancyRecord{}, false
	}
	return entry.Record, true
}

func (state *NativeWorldOccupancy) Linked(reference NativeRecordReference) (bool, bool) {
	entry, ok := state.entry(reference)
	if !ok {
		return false, false
	}
	return entry.Linked, true
}

// Access adapts the actor pools and markers to the verified native graph
// operations. SetLinks and SetPosition preserve unrelated record fields.
// Callers should use Place/Move/Remove to change membership, rather than
// directly changing a mapped record's links or high coordinate bytes.
func (state *NativeWorldOccupancy) Access() NativeRecordAccess {
	return NativeRecordAccess{
		Record: state.Record,
		SetLinks: func(reference, next, previous NativeRecordReference) {
			if entry, ok := state.entry(reference); ok {
				entry.Record.Next, entry.Record.Previous = next, previous
			}
		},
		SetPosition: func(reference NativeRecordReference, x, y uint16) {
			if entry, ok := state.entry(reference); ok {
				entry.Record.X, entry.Record.Y = x, y
			}
		},
	}
}

// Place positions a newly allocated record, then runs native $125a0 insertion.
// It preserves existing mixed occupants and does not increment pressure.
// Byte-scaled coordinate aliases match the native address calculation; normal
// creators separately reject positions outside their allowed 64x64 map.
func (state *NativeWorldOccupancy) Place(reference NativeRecordReference, x, y uint16) error {
	entry, ok := state.entry(reference)
	if !ok {
		return fmt.Errorf("unknown native occupancy placement reference %04x", uint16(reference))
	}
	if entry.Linked {
		return fmt.Errorf("native occupancy record %04x already mapped", uint16(reference))
	}
	entry.Record.X, entry.Record.Y = x, y
	index, err := nativeOccupancyIndex(x, y)
	if err != nil {
		return err
	}
	if err := state.Grid.Insert(reference, index%64, index/64, state.Access()); err != nil {
		return err
	}
	entry.Linked = true
	return nil
}

// Move writes the full coordinate words and runs $12518 only for a mapped
// actor. Its boolean result distinguishes fractional movement from a changed
// packed tile, including a native address alias that maps back to the same cell.
func (state *NativeWorldOccupancy) Move(reference NativeRecordReference, x, y uint16) (bool, error) {
	entry, ok := state.entry(reference)
	if !ok || !entry.Linked {
		return false, fmt.Errorf("native occupancy movement requires a mapped record %04x", uint16(reference))
	}
	return state.Grid.Move(reference, x, y, state.Access())
}

// Remove translates $125da and also clears membership. It can be called after
// an actor's active/owner field is cleared, or for an already unlinked record.
// Movement pressure is never decremented during removal.
func (state *NativeWorldOccupancy) Remove(reference NativeRecordReference) error {
	entry, ok := state.entry(reference)
	if !ok {
		return fmt.Errorf("unknown native occupancy removal reference %04x", uint16(reference))
	}
	if err := state.Grid.Remove(reference, state.Access()); err != nil {
		return err
	}
	entry.Linked = false
	return nil
}

// SetTile changes an overlay or effect tile without changing height, pressure
// or the mixed actor head. The original ground controllers write only byte1.
func (state *NativeWorldOccupancy) SetTile(x, y int, tile uint8) error {
	if state == nil || !inside(x, y) {
		return fmt.Errorf("native occupancy tile outside map")
	}
	state.Grid.Cells[x+y*64].Tile = tile
	return nil
}

// SetTerrain updates geometry without rebuilding membership. resetPressure
// follows native sculpting's ANDI.B #7 at $cee4/$cf40; false preserves the
// upper five bits for an explicit height-only synchronization. Initial map
// generation also starts with zero pressure. Overlay-only writes use SetTile.
func (state *NativeWorldOccupancy) SetTerrain(x, y int, height, tile uint8, resetPressure bool) error {
	if state == nil || !inside(x, y) || height > 7 {
		return fmt.Errorf("invalid native occupancy terrain coordinates/height")
	}
	cell := &state.Grid.Cells[x+y*64]
	pressure := cell.Header & 0xf8
	if resetPressure {
		pressure = 0
	}
	cell.Header, cell.Tile = pressure|height, tile
	return nil
}

func (state *NativeWorldOccupancy) Byte(offset int) (uint8, bool) {
	if state == nil {
		return 0, false
	}
	return state.Grid.Byte(offset)
}

// Word reads an aligned big-endian word from the native four-byte map grid.
// Whirlwind's occupancy-prefix source starts at byte2, not at a follower ID.
func (state *NativeWorldOccupancy) Word(offset int) (uint16, bool) {
	if offset&1 != 0 {
		return 0, false
	}
	high, ok := state.Byte(offset)
	if !ok {
		return 0, false
	}
	low, ok := state.Byte(offset + 1)
	if !ok {
		return 0, false
	}
	return uint16(high)<<8 | uint16(low), true
}

// NativeFollowerPrefix contains the non-graph fields within the first sixteen
// native follower bytes. Byte1 is state-specific (including town stage), while
// Flags is native byte13, as in FollowerMotionActor, not inherited Go state
// flags. Owner is the original byte (0 inactive,1/2 deities,3 special owner).
// Word14 retains its exact native value, ordinarily the X velocity word.
type NativeFollowerPrefix struct {
	Kind, Byte1, Owner, Flags uint8
	Animation, Word14         uint16
}

// FollowerPrefixWord supplies WhirlwindRecordPrefix reads at offsets0..14.
// Links and coordinates come from the authoritative graph, even after death
// bookkeeping changes the caller's kind, owner, animation or other fields.
func (state *NativeWorldOccupancy) FollowerPrefixWord(reference NativeRecordReference, offset uint16, prefix NativeFollowerPrefix) (uint16, bool) {
	location, ok := LocateNativeRecord(reference)
	if !ok || location.Pool != NativeFollowerPool || offset > 14 || offset&1 != 0 {
		return 0, false
	}
	record, ok := state.Record(reference)
	if !ok {
		return 0, false
	}
	switch offset {
	case 0:
		return uint16(prefix.Kind)<<8 | uint16(prefix.Byte1), true
	case 2:
		return uint16(record.Next), true
	case 4:
		return uint16(record.Previous), true
	case 6:
		return record.X, true
	case 8:
		return record.Y, true
	case 10:
		return prefix.Animation, true
	case 12:
		return uint16(prefix.Owner)<<8 | uint16(prefix.Flags), true
	default:
		return prefix.Word14, true
	}
}

// Validate checks serialized graph structure without repairing or recomputing
// it. A reference must appear once in exactly its coordinate-addressed cell,
// with a zero head predecessor and matching forward/back links. Unmapped pool
// members can retain arbitrary coordinates but must have cleared graph links.
func (state *NativeWorldOccupancy) Validate() error {
	if state == nil {
		return fmt.Errorf("missing native world occupancy")
	}
	seen := make(map[NativeRecordReference]bool)
	for index, cell := range state.Grid.Cells {
		previous := NativeRecordReference(0)
		for reference := cell.Head; reference != 0; {
			entry, ok := state.entry(reference)
			if !ok {
				return fmt.Errorf("native occupancy cell %d has unknown reference %04x", index, uint16(reference))
			}
			if seen[reference] {
				return fmt.Errorf("native occupancy duplicate/cyclic reference %04x", uint16(reference))
			}
			seen[reference] = true
			if !entry.Linked || entry.Record.Previous != previous {
				return fmt.Errorf("native occupancy reference %04x has inconsistent membership/predecessor", uint16(reference))
			}
			position, err := nativeOccupancyIndex(entry.Record.X, entry.Record.Y)
			if err != nil || position != index {
				return fmt.Errorf("native occupancy reference %04x is linked to a different coordinate cell", uint16(reference))
			}
			previous, reference = reference, entry.Record.Next
		}
	}
	for _, pool := range []struct {
		kind    NativeRecordPool
		records []NativeWorldOccupancyRecord
	}{
		{NativeWallPool, state.Walls[:]},
		{NativeSceneryPool, state.Scenery[:]},
		{NativeFollowerPool, state.Followers[:]},
		{NativeEffectPool, state.Effects[:]},
		{NativeMagnetPool, state.Magnets[:]},
	} {
		for index, entry := range pool.records {
			reference, _ := NativeWorldReference(pool.kind, index)
			if entry.Linked != seen[reference] {
				return fmt.Errorf("native occupancy reference %04x has orphaned membership", uint16(reference))
			}
			if !entry.Linked && (entry.Record.Next != 0 || entry.Record.Previous != 0) {
				return fmt.Errorf("unmapped native occupancy reference %04x retains graph links", uint16(reference))
			}
		}
	}
	return nil
}
