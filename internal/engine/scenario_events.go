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
)

type ScenarioEvent struct {
	Time            uint16
	Kind            ScenarioEventKind
	X, Y, Direction uint8
}
type ScenarioState struct {
	Events [10]ScenarioEvent
	Cursor uint8
	Err    string
}

func DecodeScenarioEvents(parameters [60]byte) (ScenarioState, error) {
	var result ScenarioState
	for index := range result.Events {
		at := index * 6
		time := binary.BigEndian.Uint16(parameters[at:])
		if time == 0 {
			break
		}
		event := ScenarioEvent{Time: time, X: parameters[at+4], Y: parameters[at+5]}
		switch command := parameters[at+3]; command {
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
	switch event.Kind {
	case ScenarioNoEvent:
		return nil
	case ScenarioFireColumn:
		w.Fire.createColumn(2, x, y, false, worldFireHabitat{w})
	case ScenarioWhirlwind:
		w.Air.CreateWhirlwind(2, x, y, worldAirHabitat{w})
	case ScenarioVolcano:
		w.Fire.CreateVolcano(2, x, y, worldFireHabitat{w})
	case ScenarioStorm:
		w.Air.CreateStorm(2, x, y, worldAirHabitat{w})
	case ScenarioFireRain:
		w.Fire.CreateRain(2, x, y, worldFireHabitat{w})
	case ScenarioRoadMaker, ScenarioLandLowerer, ScenarioWhirlwindMaker, ScenarioTreePlanter, ScenarioFireMaker, ScenarioMonster:
		_, err := w.CreateNeutral(NeutralKind(event.Kind-ScenarioRoadMaker+1), x, y)
		if err != nil && inside(x, y) {
			return nil
		} // Exhausted original pool leaves event consumed.
		return err
	case ScenarioTrees:
		_ = w.CastNeutralTrees(x, y)
	case ScenarioEarthquake:
		w.CreateEarthquake(2, x, y, int(event.Direction), 33)
	default:
		return fmt.Errorf("unknown scenario event kind %d", event.Kind)
	}
	return nil
}
