package engine

import (
	"reflect"
	"testing"
)

func TestEditorTerrainIgnoresCampaignRestrictionsWithoutSpendingMana(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario.ForbidRaise = true
	w.Players[0].Mana = 7
	if err := w.EditorSetHeight(20, 20, 4); err != nil {
		t.Fatal(err)
	}
	if w.Heights[20+20*CornerSize] != 4 || w.Players[0].Mana != 7 {
		t.Fatal("editor height or mana")
	}
	before := w.Heights
	if err := w.EditorSetHeight(20, 20, 9); err == nil || w.Heights != before {
		t.Fatal("invalid editor height mutated world")
	}
}

func TestEditorObjectsSharePoolsAndClearOnlySelectedParcel(t *testing.T) {
	w := testFlatWorld()
	if err := w.EditorPlaceFollower(0, 20, 20, 100); err != nil {
		t.Fatal(err)
	}
	if err := w.EditorPlaceFollower(1, 21, 20, 200); err != nil {
		t.Fatal(err)
	}
	if err := w.EditorPlaceScenery(SceneryTree, 20, 20); err != nil {
		t.Fatal(err)
	}
	if err := w.EditorPlaceScenery(SceneryBoulder, 21, 20); err != nil {
		t.Fatal(err)
	}
	geometry := w.Heights
	if err := w.EditorClearCell(20, 20); err != nil {
		t.Fatal(err)
	}
	if w.Occupants[20+20*MapSize] != 0 || w.Nature.sceneryAt(20, 20) >= 0 || w.Occupants[21+20*MapSize] == 0 || w.Nature.sceneryAt(21, 20) < 0 || w.Heights != geometry {
		t.Fatal("editor cleared unrelated objects or terrain")
	}
}

func TestEditorClearsSharedEffectReservationAndWallLeaderChain(t *testing.T) {
	w := testFlatWorld()
	w.Earth.Walls[0] = WallActor{Active: true, Owner: 0, X: 20, Y: 20, Next: 2}
	w.Earth.Walls[1] = WallActor{Active: true, Owner: 0, X: 21, Y: 20}
	w.Earth.WallHeads[0] = 1
	w.Level.Players[0].Powers[Lightning] = true
	w.Players[0].Mana = 1000
	if err := w.PlaceLightning(0, 20, 20); err != nil {
		t.Fatal(err)
	}
	slot := int(w.Air.MarkerSlots[0]) - 1
	if err := w.EditorClearCell(20, 20); err != nil {
		t.Fatal(err)
	}
	if w.effects.Slots[slot].Kind != EffectNone || w.Air.MarkerSlots[0] != 0 || w.Earth.WallHeads[0] != 2 || w.Earth.Walls[0].Active || !w.Earth.Walls[1].Active {
		t.Fatalf("editor left stale effects: kind%d marker%d head%d wallActive%v/%v", w.effects.Slots[slot].Kind, w.Air.MarkerSlots[0], w.Earth.WallHeads[0], w.Earth.Walls[0].Active, w.Earth.Walls[1].Active)
	}
}

func TestBulkEditorRejectsInvalidHeightsAndDiagonalSlopesAtomically(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Nature.Ground[20+20*MapSize] = GroundParcel{Mark: GroundFlowers}
	w.Pressure[20+20*MapSize] = 64
	before := w.Snapshot()
	for _, invalid := range []struct {
		at     int
		height uint8
	}{{64 + 64*CornerSize, 9}, {21 + 21*CornerSize, 3}} {
		heights := w.Heights
		heights[invalid.at] = invalid.height
		if err := w.EditorSetTerrain(heights); err == nil {
			t.Fatal("invalid terrain was accepted")
		}
		if !reflect.DeepEqual(w.Snapshot(), before) || w.Followers[id].State == Inactive {
			t.Fatal("rejected bulk edit changed simulation state")
		}
	}
}

func TestBulkEditorRebuildsGeometryOnceAndKeepsActorCleanupDeferred(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Nature.Ground[20+20*MapSize] = GroundParcel{Mark: GroundFlowers}
	w.Pressure[20+20*MapSize] = 64
	w.Nature.Ground[10+10*MapSize] = GroundParcel{Mark: GroundSwamp}
	w.Pressure[10+10*MapSize] = 80
	w.Water.Painted[21+20*MapSize] = true
	w.Water.Tiles[21+20*MapSize] = 224
	before := w.Heights
	heights := w.Heights
	for y := 19; y <= 22; y++ {
		for x := 19; x <= 22; x++ {
			heights[x+y*CornerSize] = 0
		}
	}
	if err := w.EditorSetTerrain(heights); err != nil {
		t.Fatal(err)
	}
	if w.Heights != heights || !w.Cell(20, 20).IsWater() || w.Cell(20, 20).TileIndex(1, 0, 0) != 32 {
		t.Fatal("bulk geometry or water atlas bank")
	}
	if w.Nature.Ground[20+20*MapSize].Mark != GroundNone || w.Pressure[20+20*MapSize] != 0 || w.Nature.Ground[10+10*MapSize].Mark != GroundSwamp || w.Pressure[10+10*MapSize] != 80 {
		t.Fatal("bulk edit did not invalidate only changed parcels")
	}
	if !w.Water.Painted[21+20*MapSize] || w.Water.Tiles[21+20*MapSize] != 224 {
		t.Fatal("bulk edit discarded persistent basalt")
	}
	if w.Followers[id].State != Walking || w.Occupants[20+20*MapSize] != uint16(id) {
		t.Fatal("bulk edit prematurely removed actors")
	}
	w.Level.Players[0].Scenario.FatalWater = true
	w.stepFollower(id)
	if w.Followers[id].State != Ruin || !w.Followers[id].TerrainDeath.Active {
		t.Fatal("edited terrain bypassed normal actor hazard cleanup")
	}
	_ = before
}
