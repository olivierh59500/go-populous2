package engine

import "testing"

func TestEditorScenarioMutationIsBoundedAndPreservesCampaignEncoding(t *testing.T) {
	w := &World{Editor: true}
	event := ScenarioEvent{Time: 200, Kind: ScenarioEarthquake, X: 31, Y: 32, Direction: 2}
	if err := w.EditorSetScenarioEvent(1, event); err != nil {
		t.Fatal(err)
	}
	data, err := DecodeScenarioEvents(w.Level.WorldParameters)
	if err != nil {
		t.Fatal(err)
	}
	// The leading zero event is an original terminator; inspect its named record
	// through the stored data after making that earlier entry active.
	if err := w.EditorSetScenarioEvent(0, ScenarioEvent{Time: 1, Kind: ScenarioNoEvent}); err != nil {
		t.Fatal(err)
	}
	data, err = DecodeScenarioEvents(w.Level.WorldParameters)
	if err != nil || data.Events[1] != event {
		t.Fatal("typed event and file data diverged", err, data.Events[1])
	}
	before := *w
	if err := w.EditorSetScenarioEvent(ScenarioEventCapacity, event); err == nil || *w != before {
		t.Fatal("outside event index changed detached map")
	}
	event.Kind = 255
	if err := w.EditorSetScenarioEvent(1, event); err == nil || *w != before {
		t.Fatal("unsupported effect partially changed map")
	}
}

func TestEditorSceneryBrushCyclesArtWithoutMovingOrAgingObject(t *testing.T) {
	w := &World{Editor: true}
	if err := w.EditorCycleScenery(SceneryTree, 32, 32); err != nil {
		t.Fatal(err)
	}
	before := w.Actors
	for variant := 1; variant <= 4; variant++ {
		if err := w.EditorCycleScenery(SceneryTree, 32, 32); err != nil {
			t.Fatal(err)
		}
		if w.Nature.Scenery[0].Variant != uint8(variant&3) || w.Nature.Scenery[0].Age != 24 || w.Actors != before {
			t.Fatal("original brush art cycle changed age or membership")
		}
	}
}

func TestEditorSupportsOriginalFiftiethEventAndSavedContinuation(t *testing.T) {
	w := testFlatWorld()
	w.Editor = true
	event := ScenarioEvent{Time: 2, Kind: ScenarioRoadMaker, X: 30, Y: 30}
	campaign := w.Level.WorldParameters
	if err := w.EditorSetScenarioEvent(49, event); err != nil {
		t.Fatal(err)
	}
	if w.Level.WorldParameters != campaign {
		t.Fatal("custom tail event overwrote ten-record campaign data")
	}
	w.Scenario.Cursor = 49
	w.Tick = 2
	restored, err := w.Snapshot().Restore()
	if err != nil {
		t.Fatal(err)
	}
	w.tickScenario()
	restored.tickScenario()
	if w.Scenario.Cursor != 50 || restored.Scenario.Cursor != 50 || w.Followers[1].Neutral.Kind != NeutralRoadMaker || w.Snapshot() != restored.Snapshot() {
		t.Fatal("fiftieth event did not execute and save consistently")
	}
}

func TestOriginalEditorRemovalKeepsOtherActorsAndTerrain(t *testing.T) {
	w := testFlatWorld()
	w.Editor = true
	if err := w.EditorPlaceFollower(0, 32, 32, 100); err != nil {
		t.Fatal(err)
	}
	if err := w.EditorCycleScenery(SceneryTree, 32, 32); err != nil {
		t.Fatal(err)
	}
	before := w.Heights
	if err := w.EditorRemoveFirst(32, 32); err != nil {
		t.Fatal(err)
	}
	if w.Nature.Scenery[0].Kind != SceneryNone || w.Followers[1].State == Inactive || w.Heights != before {
		t.Fatal("right-click erased more than the first eligible actor")
	}
	if _, err := w.Snapshot().Restore(); err != nil {
		t.Fatal(err)
	}
}
