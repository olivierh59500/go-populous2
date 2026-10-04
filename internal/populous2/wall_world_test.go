package populous2

import (
	"bytes"
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestNativeWallCastRenderingStateAndSave(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	if !w.Cast(0, Wall, Target{X: 32, Y: 32}) || !w.Cast(0, Wall, Target{X: 33, Y: 32}) {
		t.Fatal("connected native walls rejected")
	}
	if w.Core.MapBk2[32+32*64] == legacy.RockBlock || w.TerrainCell(32, 32).Code != 15 {
		t.Fatal("wall replaced ground with a first-game rock overlay")
	}
	for _, a := range w.Walls.Actors {
		if a.Active && len(w.WallRules.Layers(a)) == 0 {
			t.Fatal("native wall art missing")
		}
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil || restored.Walls != w.Walls {
		t.Fatalf("wall save failed: %v", err)
	}
	for range 12 {
		w.Walls.Tick(&w.WallRules, w.nativeTileAt)
		restored.Walls.Tick(&restored.WallRules, restored.nativeTileAt)
	}
	if restored.Walls != w.Walls {
		t.Fatal("save changed wall construction animation")
	}
}

func TestWallRejectsSculptWithoutMutatingTerrainOrMana(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 32 + 32*64, Flags: legacy.OnMove}}
	w.Core.MapWho[32+32*64] = 1
	if !w.Cast(0, Wall, Target{X: 32, Y: 32}) {
		t.Fatal("first wall cast rejected")
	}
	before := encodeSnapshot(t, w)
	if w.Sculpt(0, 32, 32, true) {
		t.Fatal("wall ground was raised")
	}
	if !bytes.Equal(before, encodeSnapshot(t, w)) {
		t.Fatal("rejected wall sculpt changed state")
	}
}

func TestWallCrossingCandidatesDoNotBreakActors(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 31 + 32*64, Flags: legacy.OnMove}}
	w.bindWallMovement()
	if !w.Walls.Place(&w.WallRules, 1, 32, 32, w.nativeTileAt) {
		t.Fatal("enemy wall placement rejected")
	}
	target := 32 + 32*64
	if w.Core.MovementAllowed(0, target, false) {
		t.Fatal("weak group passed enemy wall")
	}
	w.Core.Peeps[0].Population = 100000
	before := w.Walls
	if !w.Core.MovementAllowed(0, target, false) || w.Walls != before {
		t.Fatal("candidate crossing mutated wall")
	}
	if !w.Core.MovementAllowed(0, target, true) || w.Walls.At(32, 32) >= 0 {
		t.Fatal("strong committed crossing did not break wall")
	}
	restored, err := Restore(testBundle(t), w.Snapshot())
	if err != nil || restored.Core.MovementAllowed == nil {
		t.Fatalf("save lost wall movement binding: %v", err)
	}
}
