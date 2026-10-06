package engine

import "testing"

func TestAddedEditorEffectsPreserveLedgersAndRoundTripNamedFileRecords(t *testing.T) {
	for _, kind := range []ScenarioEventKind{ScenarioWhirlpool, ScenarioBatholith, ScenarioBatholithAlternate, ScenarioBaptism, ScenarioSwamp, ScenarioTsunami, ScenarioBasalt, ScenarioWind, ScenarioFlowers, ScenarioCreateBlueFollower, ScenarioCreateRedFollower, ScenarioPlantTree, ScenarioPlantRock, ScenarioRemoveActor} {
		t.Run(string(rune('A'+kind)), func(t *testing.T) {
			w := testFlatWorld()
			w.Editor = true
			w.Level.Players[0].Population, w.Level.Players[1].Population = 100, 200
			if kind == ScenarioWhirlpool || kind == ScenarioBasalt || kind == ScenarioTsunami {
				var water [CornerSize * CornerSize]uint8
				if err := w.EditorSetTerrain(water); err != nil {
					t.Fatal(err)
				}
			}
			event := ScenarioEvent{Time: 2, Kind: kind, X: 30, Y: 30}
			if kind == ScenarioWind {
				event.Direction = 1
			}
			before := [2]int{w.Players[0].Mana, w.Players[1].Mana}
			if err := w.EditorSetScenarioEvent(49, event); err != nil {
				t.Fatal(err)
			}
			encoded, err := EncodeScenarioEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			var bytes [ScenarioEventCapacity * 6]byte
			copy(bytes[49*6:], encoded[:])
			decoded, err := DecodeStoredScenarioEvents(bytes)
			if err != nil || decoded.Events[49] != event {
				t.Fatal("added named event did not round trip", err, decoded.Events[49], event)
			}
			if err := w.executeScenarioEvent(event); err != nil {
				t.Fatal(err)
			}
			if [2]int{w.Players[0].Mana, w.Players[1].Mana} != before {
				t.Fatal("unpriced scenario event changed player ledgers")
			}
			if _, err := w.Snapshot().Restore(); err != nil {
				t.Fatal("added scenario effect is not saveable", err)
			}
		})
	}
}

func TestOriginalWindFileDirectionRejectsOffMapWestFront(t *testing.T) {
	var records [ScenarioEventCapacity * 6]byte
	records[1], records[3], records[4], records[5] = 1, 76, 128, 30
	if _, err := DecodeStoredScenarioEvents(records); err == nil {
		t.Fatal("off-map source wind was silently approximated")
	}
	records[4] = 192
	state, err := DecodeStoredScenarioEvents(records)
	if err != nil || state.Events[0].Direction != 0 || state.Events[0].X != 0 {
		t.Fatal("north wind file encoding differs", err, state.Events[0])
	}
}

func TestRejectedScriptPlacementStillConsumesRecordAndRunsNextEvent(t *testing.T) {
	w := testFlatWorld()
	w.Tick = 1
	w.Scenario.Events[0] = ScenarioEvent{Time: 1, Kind: ScenarioWhirlpool, X: 30, Y: 30}
	w.Scenario.Events[1] = ScenarioEvent{Time: 1, Kind: ScenarioPlantTree, X: 30, Y: 30}
	w.tickScenario()
	if w.Scenario.Err != "" || w.Scenario.Cursor != 1 || w.Water.Whirlpools[0].Active {
		t.Fatal("inadmissible original effect stopped the scenario")
	}
	w.tickScenario()
	if w.Scenario.Err != "" || w.Scenario.Cursor != 2 || w.Nature.Scenery[0].Kind != SceneryTree {
		t.Fatal("following original event did not execute")
	}
}
