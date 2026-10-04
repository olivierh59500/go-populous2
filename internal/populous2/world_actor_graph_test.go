package populous2

import (
	"bytes"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldMixedActorGraphAndNativePressure(t *testing.T) {
	w := oceanWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	w.Scenery[0] = SceneryActor{Active: true, Kind: SceneryTree, X: 32, Y: 32, Age: 20, Animation: w.SceneryBank.Trees.Animations[0]}
	w.placeActor(NativeSceneryPool, 0, 8320, 8320)
	w.NativeEffects[0] = NativeEffectActor{Active: true, Kind: 0x20, Player: 0, X: 8320, Y: 8320, State: 8, Animation: 0x4c8, Speed: 24, Timer: 1, Life: 200}
	w.linkEffect(0)
	if err := w.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
	if w.Occupancy.Grid.Cells[pos].Header != 0 || w.Occupancy.Grid.Cells[pos].Head != nativeActorReference(NativeEffectPool, 0) {
		t.Fatal("creation incorrectly added movement pressure or replaced mixed occupants")
	}
	w.moveActor(NativeEffectPool, 0, 8576, 8320)
	w.NativeEffects[0].X = 8576
	if w.Occupancy.Grid.Cells[pos+1].Header != 8 || w.Occupancy.Grid.Cells[pos].Header != 0 {
		t.Fatal("movement pressure changed the wrong cell")
	}
	w.unlinkActor(NativeEffectPool, 0)
	w.NativeEffects[0].Active = false
	if w.Occupancy.Grid.Cells[pos+1].Header != 8 || w.Occupancy.Grid.Cells[pos].Head != nativeActorReference(NativeSceneryPool, 0) {
		t.Fatal("unlink recomputed pressure or lost the remaining native chain")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("save/load lost mixed graph or accumulated pressure")
	}
}

func TestBasaltSculptKeepsNativePrefixAndResetsOnlyChangedPressure(t *testing.T) {
	w := oceanWorld(t, 4311)
	pos := 32 + 32*64
	if !w.Cast(0, Basalt, Target{X: 32, Y: 32, Direction: 2}) || w.isWaterAt(pos) {
		t.Fatal("basalt cast still behaves as ordinary zero-height water")
	}
	w.Occupancy.Grid.Cells[pos].Header = 0xa8
	if !w.Sculpt(0, 32, 32, true) || w.nativeTileAt(32, 32) != 0xe1 || w.Occupancy.Grid.Cells[pos].Header != 0 {
		t.Fatal("sculpt discarded basalt or retained stale pressure/shape")
	}
	if !w.Sculpt(0, 32, 32, false) || w.nativeTileAt(32, 32) != 0xe0 {
		t.Fatal("lowering failed to preserve the native basalt family")
	}
	w.Occupancy.Grid.Cells[pos].Header = 0xa8
	w.lowerWaterVertex(32, 32)
	if w.Occupancy.Grid.Cells[pos].Header != 0xa8 {
		t.Fatal("zero-height no-op reset movement pressure")
	}
}

func TestWorldGraphSaveRejectsValidLinksWithWrongActorMembership(t *testing.T) {
	w := oceanWorld(t, 4311)
	if !w.Cast(0, Basalt, Target{X: 32, Y: 32, Direction: 2}) {
		t.Fatal("native basalt cast rejected")
	}
	s := w.Snapshot()
	s.NativeEffects[0].Active = false
	if _, err := Restore(testBundle(t), s); err == nil {
		t.Fatal("saved graph allowed an inactive effect to remain mapped")
	}
	s = w.Snapshot()
	s.Occupancy.Grid.Cells[32+32*64].Head = 0
	if _, err := Restore(testBundle(t), s); err == nil {
		t.Fatal("saved graph silently accepted orphaned membership")
	}
}

func TestWorldGraphSaveRejectsContradictoryMotionFractions(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	s := w.Snapshot()
	s.NativeFollowers[0].Actor.X++ // Same tile, different full native coordinate.
	if _, err := Restore(testBundle(t), s); err == nil {
		t.Fatal("saved motion and graph kept contradictory fractions")
	}
	s = w.Snapshot()
	s.NativeFollowers[0].Actor.Next = uint16(nativeActorReference(NativeEffectPool, 0))
	if _, err := Restore(testBundle(t), s); err == nil {
		t.Fatal("saved motion and graph kept contradictory links")
	}
}

func TestFireDeathRetainsFractionalGraphCoordinates(t *testing.T) {
	w := oceanWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	x, y := uint16(32*256+37), uint16(32*256+219)
	w.moveActor(NativeFollowerPool, 0, x, y)
	w.NativeFollowers[0].Actor.X, w.NativeFollowers[0].Actor.Y = int16(x), int16(y)
	w.burnFireCell(32, 32)
	before := w.Occupancy.Followers[0].Record
	s := w.Snapshot()
	if s.Occupancy.Followers[0].Record.X != x || s.Occupancy.Followers[0].Record.Y != y || w.Occupancy.Followers[0].Record != before {
		t.Fatal("fire death/snapshot recentered the original fractional record")
	}
	restored, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Snapshot().Occupancy.Followers[0].Record != before {
		t.Fatal("loading fire death changed fractional position or links")
	}
}

func TestMotionReactivationPreservesMixedChainAndFractions(t *testing.T) {
	w := oceanWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	w.moveActor(NativeFollowerPool, 0, 32*256+51, 32*256+197)
	w.Scenery[0] = SceneryActor{Active: true, Kind: SceneryTree, X: 32, Y: 32, Age: 20, Animation: w.SceneryBank.Trees.Animations[0]}
	w.placeActor(NativeSceneryPool, 0, 8320, 8320)
	before := w.Occupancy
	w.NativeFollowers[0].Active = false
	w.initializeNativeFollower(0)
	if w.Occupancy != before || uint16(w.NativeFollowers[0].Actor.X) != 32*256+51 || uint16(w.NativeFollowers[0].Actor.Y) != 32*256+197 {
		t.Fatal("motion reactivation inserted/recentered an existing mapped record")
	}
}
