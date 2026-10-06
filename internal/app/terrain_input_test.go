package app

import (
	"testing"

	"go-populous2/internal/engine"
)

func TestConquestTerrainClickCannotUseAnOffscreenOwnedTown(t *testing.T) {
	g := menuTestGame(t)
	level := g.Assets.Levels[0]
	for owner := range level.Players {
		level.Players[owner].Groups = 0
	}
	var err error
	g.World, err = engine.NewWorld(level, g.Assets.Landscapes[0])
	if err != nil {
		t.Fatal(err)
	}
	g.World.Editor = false
	g.World.Level.Players[0].Scenario = engine.ScenarioOptions{}
	g.World.Level.Players[0].Powers[engine.RaiseLower] = true
	g.World.Players[0].Mana = 10000
	var heights [engine.CornerSize * engine.CornerSize]uint8
	for i := range heights {
		heights[i] = 2
	}
	if err := g.World.EditorSetTerrain(heights); err != nil {
		t.Fatal(err)
	}
	if err := g.World.EditorPlaceFollower(0, 20, 20, 100); err != nil {
		t.Fatal(err)
	}
	for id := 1; id < engine.FollowerCapacity; id++ {
		if f := &g.World.Followers[id]; f.State != engine.Inactive && f.X == 20 && f.Y == 20 {
			f.State, f.Stage = engine.Town, 1
		}
	}
	g.CameraX, g.CameraY = 40, 40
	before := g.World.Snapshot()
	if err := g.applyTerrainClick(44, 44, false); err == nil {
		t.Fatal("empty conquest view accepted terrain modification")
	}
	if g.World.Snapshot() != before {
		t.Fatal("rejected terrain click changed state or mana")
	}
	g.CameraX, g.CameraY = 16, 16
	g.Screen = Playing
	g.presentation.Capture(g, false)
	// The following physics pass removes the town, while the last main image
	// still displays it. Admission belongs to that image; edits affect live state.
	for id := 1; id < engine.FollowerCapacity; id++ {
		if f := &g.World.Followers[id]; f.State != engine.Inactive && f.X == 20 && f.Y == 20 {
			f.State = engine.Inactive
		}
	}
	if err := g.applyTerrainClick(20, 20, false); err != nil {
		t.Fatal("owned town in displayed view did not grant construction rights", err)
	}
	if g.World.Heights[20+20*engine.CornerSize] != 3 {
		t.Fatal("admitted click did not raise the target")
	}
	if g.presentation.World.Heights[20+20*engine.CornerSize] != 2 {
		t.Fatal("terrain edit mutated the displayed image instead of the live world")
	}
	g.presentation.Capture(g, false)
	before = g.World.Snapshot()
	if err := g.applyTerrainClick(20, 20, false); err == nil || g.World.Snapshot() != before {
		t.Fatal("new empty presentation retained obsolete construction rights", err)
	}
}
