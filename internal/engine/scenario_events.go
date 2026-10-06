package engine

import (
	"encoding/binary"
	"fmt"
)

type ScenarioEventKind uint8

const (
	ScenarioNoEvent ScenarioEventKind = iota
	ScenarioFireColumn
	ScenarioWhirlwind
	ScenarioEarthquake
	ScenarioTrees
	ScenarioVolcano
	ScenarioStorm
	ScenarioFireRain
	ScenarioRoadMaker
	ScenarioLandLowerer
	ScenarioWhirlwindMaker
	ScenarioTreePlanter
	ScenarioFireMaker
	ScenarioMonster
	ScenarioWhirlpool
	ScenarioBatholith
	ScenarioBatholithAlternate
	ScenarioBaptism
	ScenarioSwamp
	ScenarioTsunami
	ScenarioBasalt
	ScenarioWind
	ScenarioFlowers
	ScenarioCreateBlueFollower
	ScenarioCreateRedFollower
	ScenarioPlantTree
	ScenarioPlantRock
	ScenarioRemoveActor
)

type ScenarioEvent struct {
	Time            uint16
	Kind            ScenarioEventKind
	X, Y, Direction uint8
}

const ScenarioEventCapacity = 50

type ScenarioState struct {
	Events [ScenarioEventCapacity]ScenarioEvent
	Cursor uint8
	Err    string
}

func DecodeScenarioEvents(parameters [60]byte) (ScenarioState, error) {
	return DecodeScenarioRecords(parameters[:])
}

// DecodeScenarioRecords accepts the original six-byte data records. Campaign
// resources supply ten; custom maps and saved games can supply fifty.
func DecodeScenarioRecords(parameters []byte) (ScenarioState, error) {
	var result ScenarioState
	if len(parameters)%6 != 0 || len(parameters) > ScenarioEventCapacity*6 {
		return result, fmt.Errorf("scenario record count exceeds supported capacity")
	}

	for index := 0; index < len(parameters)/6; index++ {
		at := index * 6
		time := binary.BigEndian.Uint16(parameters[at:])
		if time == 0 {
			break
		}
		event := ScenarioEvent{Time: time, X: parameters[at+4], Y: parameters[at+5]}
		switch command := parameters[at+3]; command {
		case 24:
			event.Kind = ScenarioWhirlpool
		case 48:
			event.Kind = ScenarioBatholith
		case 50:
			event.Kind = ScenarioBatholithAlternate
		case 52:
			event.Kind = ScenarioBaptism
		case 54:
			event.Kind = ScenarioSwamp
		case 56:
			event.Kind = ScenarioTsunami
		case 74:
			event.Kind = ScenarioBasalt
			event.Direction = (event.X >> 5) & 3
		case 76:
			rawX := event.X
			event.Kind = ScenarioWind
			event.Direction = ((rawX >> 6) + 1) & 3
			event.X &= 63
			if event.Direction == 3 {
				return result, fmt.Errorf("wind record%d uses an off-map source front", index)
			}
		case 80:
			event.Kind = ScenarioFlowers
		case 82:
			event.Kind = ScenarioCreateBlueFollower
		case 84:
			event.Kind = ScenarioCreateRedFollower
		case 86:
			event.Kind = ScenarioPlantTree
		case 88:
			event.Kind = ScenarioPlantRock
		case 102:
			event.Kind = ScenarioRemoveActor
		case 0:
			event.Kind = ScenarioNoEvent
		case 6:
			event.Kind = ScenarioFireColumn
		case 22:
			event.Kind = ScenarioWhirlwind
		case 40:
			event.Kind = ScenarioEarthquake
			event.Direction = event.X >> 6
			event.X &= 63
		case 46:
			event.Kind = ScenarioTrees
		case 62:
			event.Kind = ScenarioVolcano
		case 64:
			event.Kind = ScenarioStorm
		case 38:
			event.Kind = ScenarioFireRain
		case 90:
			event.Kind = ScenarioRoadMaker
		case 92:
			event.Kind = ScenarioLandLowerer
		case 94:
			event.Kind = ScenarioWhirlwindMaker
		case 96:
			event.Kind = ScenarioTreePlanter
		case 98:
			event.Kind = ScenarioFireMaker
		case 100:
			event.Kind = ScenarioMonster
		default:
			return result, fmt.Errorf("unknown campaign event command %d at event %d", command, index)
		}
		result.Events[index] = event
	}
	return result, nil
}

// tickScenario executes at most one due event after scenery and before player
// commands. Advancing the cursor does not depend on successful allocation.
func (w *World) tickScenario() {
	s := &w.Scenario
	if s.Err != "" || int(s.Cursor) >= len(s.Events) {
		return
	}
	event := s.Events[s.Cursor]
	if event.Time == 0 || event.Time > uint16(w.Tick) {
		return
	}
	s.Cursor++
	if err := w.executeScenarioEvent(event); err != nil {
		s.Err = err.Error()
	}
}
func (w *World) executeScenarioEvent(event ScenarioEvent) error {
	x, y := int(event.X), int(event.Y)
	owner := 2
	if w.Editor {
		owner = 1
	}
	switch event.Kind {
	case ScenarioWhirlpool:
		_ = w.createWhirlpool(owner, x, y)
		return nil
	case ScenarioBatholith, ScenarioBatholithAlternate:
		_ = w.createBatholith(owner, x, y)
		return nil
	case ScenarioBaptism:
		return w.createBaptism(owner, x, y)
	case ScenarioSwamp:
		return w.createSwamp(owner, x, y)
	case ScenarioTsunami:
		return w.createTsunami(owner, x, y)
	case ScenarioBasalt:
		if w.CreateBasalt(uint8(owner), x, y, x>>5, 100) {
			return nil
		}
		return nil
	case ScenarioWind:
		_ = w.createWind(owner, x, y, int(event.Direction))
		return nil
	case ScenarioFlowers:
		return w.createFlowers(owner, x, y)
	case ScenarioCreateBlueFollower, ScenarioCreateRedFollower:
		owner := 0
		if event.Kind == ScenarioCreateRedFollower {
			owner = 1
		}
		_ = w.EditorPlaceFollower(owner, x, y, w.Level.Players[owner].Population)
		return nil
	case ScenarioPlantTree:
		_ = w.cycleSceneryBrush(SceneryTree, x, y)
		return nil
	case ScenarioPlantRock:
		_ = w.cycleSceneryBrush(SceneryBoulder, x, y)
		return nil
	case ScenarioRemoveActor:
		return w.removeFirstPaintActor(x, y)
	case ScenarioNoEvent:
		return nil
	case ScenarioFireColumn:
		w.Fire.createColumn(uint8(owner), x, y, false, worldFireHabitat{w})
	case ScenarioWhirlwind:
		w.Air.CreateWhirlwind(uint8(owner), x, y, worldAirHabitat{w})
	case ScenarioVolcano:
		w.Fire.CreateVolcano(uint8(owner), x, y, worldFireHabitat{w})
	case ScenarioStorm:
		w.Air.CreateStorm(uint8(owner), x, y, worldAirHabitat{w})
	case ScenarioFireRain:
		w.Fire.CreateRain(uint8(owner), x, y, worldFireHabitat{w})
	case ScenarioRoadMaker, ScenarioLandLowerer, ScenarioWhirlwindMaker, ScenarioTreePlanter, ScenarioFireMaker, ScenarioMonster:
		_, err := w.CreateNeutral(NeutralKind(event.Kind-ScenarioRoadMaker+1), x, y)
		if err != nil && inside(x, y) {
			return nil
		} // Exhausted original pool leaves event consumed.
		return err
	case ScenarioTrees:
		if owner == 2 {
			_ = w.CastNeutralTrees(x, y)
		} else {
			_ = w.CastTrees(owner, x, y)
		}
	case ScenarioEarthquake:
		strength := 33
		if owner < 2 {
			strength += int(w.Players[owner].Experience[Earth])
		}
		w.CreateEarthquake(uint8(owner), x, y, int(event.Direction), strength)
	default:
		return fmt.Errorf("unknown scenario event kind %d", event.Kind)
	}
	return nil
}

// DecodeStoredScenarioEvents retains each custom-map slot, including inactive
// entries after a zero-time terminator. Runtime scheduling still stops there.
func DecodeStoredScenarioEvents(records [ScenarioEventCapacity * 6]byte) (ScenarioState, error) {
	var result ScenarioState
	for index := range result.Events {
		record := append([]byte(nil), records[index*6:index*6+6]...)
		time := binary.BigEndian.Uint16(record)
		// The single-record campaign decoder normally stops at time zero. Use a
		// temporary active time to decode the named fields, then restore its value.
		binary.BigEndian.PutUint16(record, 1)
		decoded, err := DecodeScenarioRecords(record)
		if err != nil {
			return result, fmt.Errorf("stored event%d: %w", index, err)
		}
		result.Events[index] = decoded.Events[0]
		result.Events[index].Time = time
	}
	return result, nil
}

// EncodeScenarioEvent returns one original six-byte file record, not a
// controller address or executable program. Unknown semantic kinds reject.
func EncodeScenarioEvent(event ScenarioEvent) ([6]byte, error) {
	var result [6]byte
	codes := map[ScenarioEventKind]uint16{ScenarioNoEvent: 0, ScenarioFireColumn: 6, ScenarioWhirlwind: 22, ScenarioEarthquake: 40, ScenarioTrees: 46, ScenarioVolcano: 62, ScenarioStorm: 64, ScenarioFireRain: 38, ScenarioRoadMaker: 90, ScenarioLandLowerer: 92, ScenarioWhirlwindMaker: 94, ScenarioTreePlanter: 96, ScenarioFireMaker: 98, ScenarioMonster: 100,
		ScenarioWhirlpool: 24, ScenarioBatholith: 48, ScenarioBatholithAlternate: 50, ScenarioBaptism: 52, ScenarioSwamp: 54, ScenarioTsunami: 56, ScenarioBasalt: 74, ScenarioWind: 76, ScenarioFlowers: 80,
		ScenarioCreateBlueFollower: 82, ScenarioCreateRedFollower: 84, ScenarioPlantTree: 86, ScenarioPlantRock: 88, ScenarioRemoveActor: 102,
	}
	code, ok := codes[event.Kind]
	if !ok || event.X >= MapSize || event.Y >= MapSize || event.Direction > 3 {
		return result, fmt.Errorf("invalid scenario event record")
	}
	binary.BigEndian.PutUint16(result[:], event.Time)
	binary.BigEndian.PutUint16(result[2:], code)
	result[4], result[5] = event.X, event.Y
	if event.Kind == ScenarioEarthquake {
		result[4] |= event.Direction << 6
	}
	if event.Kind == ScenarioWind {
		if event.Direction == 3 {
			return result, fmt.Errorf("west wind cannot encode a bounded original front")
		}
		result[4] |= ((event.Direction + 3) & 3) << 6
	}
	return result, nil
}
