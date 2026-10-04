package populous2

import "fmt"

// NativeRecordReference is a byte offset from the original shared record base
// $76c0. It is not a Go follower index. Zero is the native null reference.
type NativeRecordReference uint16

// NativePackedTile stores X in the low byte and Y in the high byte.
type NativePackedTile uint16

type WhirlwindInteractionRules struct {
	PickupAnimations [6]uint16
	ReleaseOffsets   [8]uint16
}

func (rules WhirlwindRules) InteractionRules() WhirlwindInteractionRules {
	var result WhirlwindInteractionRules
	for index, animation := range rules.PickupAnimations {
		result.PickupAnimations[index] = uint16(animation)
	}
	for index, offset := range rules.ReleaseOffsets {
		result.ReleaseOffsets[index] = uint16(int16(offset[0] + offset[1]*256))
	}
	return result
}

type WhirlwindLift struct {
	Kind, State, Weapon uint8
	Animation           uint16
	EffectReference     NativeRecordReference
}

// Lift translates $16132. The caller admits native kinds 2, 10, or 4 and
// clears a town's farms separately. Population, owner, and flags stay intact.
func (rules WhirlwindInteractionRules) Lift(state uint8, isHero bool, heroByteOffset uint16, effect NativeRecordReference) (WhirlwindLift, bool) {
	if state == 0x3a {
		return WhirlwindLift{}, false
	}
	animation := uint16(0x4d4)
	if isHero {
		if heroByteOffset&1 != 0 || int(heroByteOffset/2) >= len(rules.PickupAnimations) {
			return WhirlwindLift{}, false
		}
		animation = rules.PickupAnimations[heroByteOffset/2]
		if animation == 0 {
			return WhirlwindLift{}, false
		}
	}
	return WhirlwindLift{Kind: 8, State: 0x14, Weapon: 1, Animation: animation, EffectReference: effect}, true
}

type WhirlwindWordSourceKind uint8

const (
	WhirlwindReleaseTable WhirlwindWordSourceKind = iota
	WhirlwindOccupancyPrefix
	WhirlwindRecordPrefix
)

// WhirlwindWordSource preserves the native A2 register without unsafe pointers.
// OccupancyPrefix starts at $f46: parcel 0's head, then alternating raw
// height/tile and head words. RecordPrefix starts at the named 52-byte record.
type WhirlwindWordSource struct {
	Kind      WhirlwindWordSourceKind
	Reference NativeRecordReference
}

type WhirlwindReleaseRecord struct {
	Kind uint8
	Next NativeRecordReference
}

type WhirlwindReleaseCallbacks struct {
	DeactivateAndUnlinkEffect func(NativeRecordReference)
	Head                      func(NativePackedTile) NativeRecordReference
	Record                    func(NativeRecordReference) WhirlwindReleaseRecord
	ReadWord                  func(WhirlwindWordSource, uint16) uint16
	Random                    func() int
	// Move returns true only when the packed tile changes. Native $12518 then
	// leaves A2 at $f46; a move within the same tile preserves its old value.
	Move func(NativeRecordReference, uint16, uint16) bool
	// Land sets state $1a and animation $68c, retaining kind, weapon, flags,
	// reference, and population. The later follower handler completes landing.
	Land func(NativeRecordReference)
	// Death performs native $124a2 cleanup with D0=0 and returns the resulting
	// low word of D1. For ordinary nonleaders this is owner*$13a. Leader cleanup
	// can change D1 further, so the dispatcher must not reconstruct it.
	Death func(NativeRecordReference) uint16
}

type WhirlwindReleaseResult struct {
	Visited, Released, Removed int
	Source                     WhirlwindWordSource
	CoordinateBase             NativePackedTile
}

// Release translates $14c20 for both an edge exit and the final outro frame.
// It scans every kind-8 record on the old tile, without checking which effect
// that record references. It preserves the native A2 and D1 side effects.
func (rules WhirlwindInteractionRules) Release(effect NativeRecordReference, tile NativePackedTile, callbacks WhirlwindReleaseCallbacks) (WhirlwindReleaseResult, error) {
	result := WhirlwindReleaseResult{Source: WhirlwindWordSource{Kind: WhirlwindReleaseTable}, CoordinateBase: tile}
	if callbacks.DeactivateAndUnlinkEffect == nil || callbacks.Head == nil || callbacks.Record == nil || callbacks.ReadWord == nil || callbacks.Random == nil || callbacks.Move == nil || callbacks.Land == nil || callbacks.Death == nil {
		return result, fmt.Errorf("incomplete native whirlwind release callbacks")
	}
	callbacks.DeactivateAndUnlinkEffect(effect)
	reference := callbacks.Head(tile)
	for reference != 0 {
		// A valid graph has at most 400 follower records and 250 effect records.
		// Bound malformed input rather than looping forever after callbacks mutate it.
		if result.Visited >= 400+NativeEffectCapacity {
			return result, fmt.Errorf("native whirlwind occupancy chain exceeds record capacity")
		}
		record := callbacks.Record(reference)
		result.Visited++
		// $14c54 saves the next reference before moving or removing the record.
		next := record.Next
		if record.Kind == 8 {
			selector := uint16(callbacks.Random()) & 0x0e
			offset := callbacks.ReadWord(result.Source, selector)
			// Word addition wraps exactly as native ADD.W. The unsigned bit mask
			// rejects either coordinate outside the original 64 by 64 map.
			destination := uint16(result.CoordinateBase) + offset
			if destination&0xc0c0 != 0 {
				result.Source = WhirlwindWordSource{Kind: WhirlwindRecordPrefix, Reference: reference}
				result.CoordinateBase = NativePackedTile(callbacks.Death(reference))
				result.Removed++
			} else {
				x := uint16(uint8(destination))*256 + 128
				y := uint16(uint8(destination>>8))*256 + 128
				if callbacks.Move(reference, x, y) {
					result.Source = WhirlwindWordSource{Kind: WhirlwindOccupancyPrefix}
				}
				callbacks.Land(reference)
				result.Released++
			}
		}
		reference = next
	}
	return result, nil
}
