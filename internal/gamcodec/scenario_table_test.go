package gamcodec

import (
	"encoding/binary"
	"testing"

	"go-populous2/internal/engine"
)

func TestGAMRetainsAllFiftyScenarioSlotsAndTailCursor(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	w.Editor = true
	w.Scenario.Events[49] = engine.ScenarioEvent{Time: 123, Kind: engine.ScenarioEarthquake, X: 24, Y: 25, Direction: 3}
	w.Scenario.Cursor = 49
	doc, err := NewDocument(w, catalog, engine.NewDeity("EDITOR"), 0, 4, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint16(data[0xf0a-fileStart:]) != 49*6 || data[0xdde+49*6+4-fileStart] != 24|3<<6 {
		t.Fatal("tail cursor or earthquake direction did not match original file fields")
	}
	got, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if got.World.Scenario.Events[49] != w.Scenario.Events[49] || got.World.Scenario.Cursor != 49 {
		t.Fatal("late editor event after a disabled first row was lost")
	}
	got.World.Editor = true
	got.World.Scenario.Events[49] = engine.ScenarioEvent{Time: 1, Kind: engine.ScenarioFireColumn, X: 20, Y: 20}
	got.World.Step()
	if got.World.Scenario.Cursor != 50 || got.World.Scenario.Err != "" {
		t.Fatal("restored final event did not execute and reach the original fifty-record limit")
	}
	var columns int
	for _, column := range got.World.Fire.Columns {
		if column.Active {
			columns++
		}
	}
	if columns != 1 {
		t.Fatal("restored last scenario slot did not create its actual controller", columns)
	}
	bad := append([]byte(nil), data...)
	binary.BigEndian.PutUint16(bad[0xf0a-fileStart:], 301)
	if _, err := Decode(bad, catalog); err == nil {
		t.Fatal("scenario cursor beyond the saved table accepted")
	}
}
