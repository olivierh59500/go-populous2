package network

import (
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

func TestNetworkTerrainClickChecksOwnedActorsInTheSentView(t *testing.T) {
	w := networkWorld(t)
	for owner := range w.Level.Players {
		w.Level.Players[owner].Scenario = engine.ScenarioOptions{}
	}
	for y := 40; y < 48; y++ {
		for x := 40; x < 48; x++ {
			if err := w.EditorClearCell(x, y); err != nil {
				t.Fatal(err)
			}
		}
	}
	view := engine.Viewport{X: 40, Y: 40, Size: 8}
	command := Command{Kind: "terrain-click", Target: engine.PowerTarget{X: 44, Y: 44}, View: view}
	if err := validateCommands([]Command{command}, 0); err != nil {
		t.Fatal(err)
	}
	before := w.Snapshot()
	if err := applyCommand(w, 0, command); err == nil || !reflect.DeepEqual(before, w.Snapshot()) {
		t.Fatal("empty displayed view changed terrain or spent mana")
	}
	if err := w.EditorPlaceFollower(1, 44, 44, 100); err != nil {
		t.Fatal(err)
	}
	for id := range w.Followers {
		f := &w.Followers[id]
		if f.State != engine.Inactive && f.X == 44 && f.Y == 44 {
			f.State, f.Stage = engine.Town, 1
		}
	}
	before = w.Snapshot()
	if err := applyCommand(w, 0, command); err == nil || !reflect.DeepEqual(before, w.Snapshot()) {
		t.Fatal("opponent town supplied construction rights")
	}
	if err := applyCommand(w, 1, command); err != nil {
		t.Fatal("owned displayed town did not supply construction rights", err)
	}
}

func TestNetworkCannotBypassViewportWithGenericTerrainPower(t *testing.T) {
	command := Command{Kind: "power", Power: engine.RaiseLower, Target: engine.PowerTarget{X: 20, Y: 20}}
	if err := validateCommands([]Command{command}, 0); err == nil {
		t.Fatal("generic terrain command bypassed viewport validation")
	}
	command.Kind = "terrain-click"
	command.View = engine.Viewport{X: 16, Y: 16, Size: 8}
	command.Target.X = 30
	if err := validateCommands([]Command{command}, 0); err == nil {
		t.Fatal("out-of-view terrain target accepted")
	}
}
