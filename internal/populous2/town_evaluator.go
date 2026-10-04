package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeTownRecord struct {
	Kind, Owner, Stage, State uint8
	Animation                 uint16
	Next                      NativeRecordReference
	X, Y                      uint16
}

type NativeTownCallbacks struct {
	Record       func(NativeRecordReference) (NativeTownRecord, bool)
	SetRecord    func(NativeRecordReference, NativeTownRecord)
	Head         func(int, int) NativeRecordReference
	ReadTile     func(int, int) uint8
	WriteTile    func(int, int, uint8)
	WriteOverlay func(int, int, uint8)
}

type NativeTownEvaluator struct {
	Stages          [27]uint8
	Footprint       [49]uint16
	ClearCounters   [19]uint8
	Properties      [256]uint16
	StructureTiles  [19][8]uint8
	OverlayOffsets  [8]uint16
	OverlayFrames   [35]AnimationFrame
	OverlayPointers [35]uint16
}

func DecodeNativeTownEvaluator(exe *amiga.Executable) (NativeTownEvaluator, error) {
	var result NativeTownEvaluator
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return result, fmt.Errorf("native settlement tables missing")
	}
	code := exe.Hunks[0].Data
	copy(result.Stages[:], code[0x13668:0x13683])
	copy(result.ClearCounters[:], code[0x13654:0x13667])
	for index := range result.Footprint {
		result.Footprint[index] = binary.BigEndian.Uint16(code[0x13684+index*2:])
	}
	for index := range result.Properties {
		result.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	for stage := range result.StructureTiles {
		copy(result.StructureTiles[stage][:], code[0x1374c+stage*8:0x1374c+(stage+1)*8])
	}
	for index := range result.OverlayOffsets {
		result.OverlayOffsets[index] = binary.BigEndian.Uint16(code[0x1382a+index*2:])
	}
	for index := range result.OverlayPointers {
		result.OverlayPointers[index] = binary.BigEndian.Uint16(code[0x137e4+index*2:])
		if index == 0 {
			continue
		}
		// $e2ba/$ee32 draws this exact record; it does not advance a sequence.
		at := 0x23d1a + int(result.OverlayPointers[index])
		if at+4 > len(code) || result.OverlayPointers[index]&3 != 0 {
			return NativeTownEvaluator{}, fmt.Errorf("native settlement overlay pointer outside animation bank")
		}
		image := int16(binary.BigEndian.Uint16(code[at:]))
		if image < 0 {
			return NativeTownEvaluator{}, fmt.Errorf("native settlement overlay points to a sequence marker")
		}
		layers, err := decodeImageLayers(code, uint16(image))
		if err != nil {
			return NativeTownEvaluator{}, err
		}
		cue := int(binary.BigEndian.Uint16(code[at+2:]))
		if cue%10 != 0 || cue/10 >= 133 {
			return NativeTownEvaluator{}, fmt.Errorf("invalid native settlement overlay cue")
		}
		result.OverlayFrames[index] = AnimationFrame{Layers: layers, SoundCue: cue / 10}
	}
	for _, stage := range result.Stages {
		if stage >= 19 {
			return NativeTownEvaluator{}, fmt.Errorf("native support stage outside nineteen-stage table")
		}
	}
	return result, nil
}

func nativeTownParcel(origin, offset uint16) (int, int, bool) {
	parcel := origin + offset
	return int(uint8(parcel)), int(uint8(parcel >> 8)), parcel&0xc0c0 == 0
}

func nativeTownRecord(reference NativeRecordReference, callbacks NativeTownCallbacks) (NativeTownRecord, error) {
	if callbacks.Record == nil {
		return NativeTownRecord{}, fmt.Errorf("native settlement record callback missing")
	}
	record, ok := callbacks.Record(reference)
	if !ok {
		return NativeTownRecord{}, fmt.Errorf("native settlement reference %04x missing", uint16(reference))
	}
	return record, nil
}

func nativeTownCallbacksValid(callbacks NativeTownCallbacks) bool {
	return callbacks.Record != nil && callbacks.SetRecord != nil && callbacks.Head != nil && callbacks.ReadTile != nil && callbacks.WriteTile != nil && callbacks.WriteOverlay != nil
}

// ClearFarms translates $135ca. It clears each visited overlay regardless of
// terrain ownership, but replaces only the record owner's exact farm tile.
// Headers, occupancy links, population, and work counters stay intact.
func (e *NativeTownEvaluator) ClearFarms(reference NativeRecordReference, replacement uint8, callbacks NativeTownCallbacks) error {
	if e == nil || !nativeTownCallbacksValid(callbacks) {
		return fmt.Errorf("native settlement cleanup callbacks missing")
	}
	record, err := nativeTownRecord(reference, callbacks)
	if err != nil {
		return err
	}
	if int(record.Stage) >= len(e.ClearCounters) {
		return fmt.Errorf("native settlement cleanup stage outside table")
	}
	ownerFarm := uint8(47)
	if record.Owner != 1 {
		ownerFarm = 63
	}
	origin := record.Y&0xff00 | record.X>>8
	for _, offset := range e.Footprint[:int(e.ClearCounters[record.Stage])+1] {
		x, y, inside := nativeTownParcel(origin, offset)
		if !inside {
			continue
		}
		callbacks.WriteOverlay(x, y, 0)
		tile := callbacks.ReadTile(x, y)
		if tile == ownerFarm && e.Properties[tile]&0x10 == 0 {
			callbacks.WriteTile(x, y, replacement)
		}
	}
	return nil
}

// Evaluate translates $13352/$133c2. It owns native farm/overlay writes and
// competitor town demotion; the caller owns founding/ordinary town dispatch.
// It never resets population, owner, flags, or the work word at record+$14.
func (e *NativeTownEvaluator) Evaluate(reference NativeRecordReference, clock uint16, callbacks NativeTownCallbacks) (uint8, error) {
	if e == nil || !nativeTownCallbacksValid(callbacks) {
		return 0, fmt.Errorf("native settlement evaluator callbacks missing")
	}
	record, err := nativeTownRecord(reference, callbacks)
	if err != nil {
		return 0, err
	}
	location, ok := LocateNativeRecord(reference)
	if !ok || location.Pool != NativeFollowerPool || record.Stage >= 19 {
		return 0, fmt.Errorf("invalid native settlement follower reference or stage")
	}
	x, y := int(record.X>>8), int(record.Y>>8)
	if !inside(x, y) {
		return 0, fmt.Errorf("native settlement outside map")
	}
	if record.Stage != 0 && clock&3 != uint16(location.Index)&3 && e.Properties[callbacks.ReadTile(x, y)]&7 != 0 {
		return record.Stage, nil
	}
	origin := record.Y&0xff00 | record.X>>8
	farm, forbiddenHighBit := uint8(47), uint16(0x400)
	if record.Owner != 1 {
		farm, forbiddenHighBit = 63, 0x200
	}
	pending := [][2]int{}
	count := 0
	visit := func(offset uint16, mutate bool) (bool, error) {
		xx, yy, inside := nativeTownParcel(origin, offset)
		if !inside {
			return false, nil
		}
		properties := e.Properties[callbacks.ReadTile(xx, yy)]
		if properties&0x17 == 0 || properties&0x10 != 0 {
			return false, nil
		}
		seen := make(map[NativeRecordReference]bool)
		for other := callbacks.Head(xx, yy); other != 0; {
			if seen[other] {
				return false, fmt.Errorf("cyclic native settlement occupancy chain")
			}
			seen[other] = true
			occupant, err := nativeTownRecord(other, callbacks)
			if err != nil {
				return false, err
			}
			if mutate && occupant.Kind == 4 && other != reference {
				if err := e.ClearFarms(other, 15, callbacks); err != nil {
					return false, err
				}
				occupant.Kind, occupant.State, occupant.Animation = 2, 2, 0
				callbacks.SetRecord(other, occupant)
			}
			if occupant.Kind == 0x18 {
				return false, nil
			}
			other = occupant.Next
		}
		if mutate {
			// $13538 tests a byte alias of the cached property's high byte,
			// giving word bit10 for owner1 and word bit9 for owner2.
			if properties&forbiddenHighBit != 0 {
				return false, nil
			}
			if callbacks.ReadTile(xx, yy) != farm {
				pending = append(pending, [2]int{xx, yy})
			}
		}
		return true, nil
	}
	pass := func(start, length int) error {
		for _, offset := range e.Footprint[start : start+length] {
			supported, err := visit(offset, true)
			if err != nil {
				return err
			}
			if supported {
				count++
			}
		}
		return nil
	}
	if err := pass(0, 1); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}
	if err := pass(1, 8); err != nil {
		return 0, err
	}
	if count == 9 {
		if err := pass(9, 16); err != nil {
			return 0, err
		}
		if count == 25 {
			clear := true
			for _, offset := range e.Footprint[25:] {
				accepted, err := visit(offset, false)
				if err != nil {
					return 0, err
				}
				if !accepted {
					clear = false
					break
				}
			}
			if clear {
				if err := pass(25, 24); err != nil {
					return 0, err
				}
				count = 26
			}
		}
	}
	stage := e.Stages[count]
	if stage < record.Stage {
		start := 9
		if stage >= 9 {
			start = 25
		}
		for _, offset := range e.Footprint[start:] {
			xx, yy, inside := nativeTownParcel(origin, offset)
			if inside && callbacks.ReadTile(xx, yy) == farm {
				// The apparent $135bc cross-routine branch tests D0 after
				// AND$c0c0; valid cells give zero, including old stages16..18.
				callbacks.WriteTile(xx, yy, 15)
			}
		}
	}
	for index := len(pending) - 1; index >= 0; index-- {
		callbacks.WriteTile(pending[index][0], pending[index][1], farm)
	}
	for index, offset := range e.OverlayOffsets {
		xx, yy, inside := nativeTownParcel(origin, offset)
		if inside {
			callbacks.WriteOverlay(xx, yy, e.StructureTiles[stage][index])
		}
	}
	if stage != 0 && stage != record.Stage {
		// Reload after competitor callbacks so only the native stage byte is
		// changed; all unrelated fields remain under the record adapter's care.
		record, err = nativeTownRecord(reference, callbacks)
		if err != nil {
			return 0, err
		}
		record.Stage = stage
		callbacks.SetRecord(reference, record)
	}
	return stage, nil
}
