package engine

import "testing"

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
