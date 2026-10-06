package app

import (
	"testing"

	"go-populous2/internal/engine"
)

func TestEditorPaintAndCancelKeepLiveWorldAndCamera(t *testing.T) {
	g := menuTestGame(t)
	g.CameraX, g.CameraY = 12, 13
	before := g.World.Snapshot()
	if err := g.openEditor(); err != nil {
		t.Fatal(err)
	}
	g.Editor.Tool = EditorLevel
	g.Editor.Height = 4
	if err := g.Editor.paint(32, 32); err != nil {
		t.Fatal(err)
	}
	if g.World.Heights[32+32*engine.CornerSize] != before.World.Heights[32+32*engine.CornerSize] {
		t.Fatal("editor paint changed live world")
	}
	g.CameraX, g.CameraY = 40, 41
	g.cancelEditor()
	if g.World.Snapshot() != before || g.CameraX != 12 || g.CameraY != 13 || g.Screen != Playing {
		t.Fatal("editor cancel lost continuation or camera")
	}
}

func TestEditorApplyAdoptsValidatedDetachedTerrainAndObjects(t *testing.T) {
	g := menuTestGame(t)
	if err := g.openEditor(); err != nil {
		t.Fatal(err)
	}
	live := g.World
	g.Editor.Tool = EditorLevel
	g.Editor.Height = 3
	if err := g.Editor.paint(32, 32); err != nil {
		t.Fatal(err)
	}
	g.Editor.Tool = EditorRed
	g.Editor.Population = 250
	if err := g.Editor.paint(32, 32); err != nil {
		t.Fatal(err)
	}
	if err := g.applyEditor(); err != nil {
		t.Fatal(err)
	}
	if g.World == live || g.World.Editor || g.Screen != Playing || g.World.Heights[32+32*engine.CornerSize] != 3 {
		t.Fatal("editor apply did not adopt validated detached world")
	}
	var ids [engine.FollowerCapacity]int
	count := g.World.FollowersAt(32, 32, ids[:])
	found := false
	for _, id := range ids[:count] {
		f := g.World.Followers[id]
		if f.Owner == 1 && f.Population == 250 {
			found = true
		}
	}
	if !found {
		t.Fatal("edited red group did not reach play")
	}
}
