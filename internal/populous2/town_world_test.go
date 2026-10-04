package populous2

import (
	"bytes"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestNativeTownWorldPaintsFortyNineCellsAndPreservesWork(t *testing.T) {
	w := lightningWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.InTown, MovementSpeed: 20, TownWork: 23}}
	w.initializeNativeFollower(0)
	ref := nativeActorReference(NativeFollowerPool, 0)
	stage, err := w.evaluateNativeTown(ref)
	if err != nil {
		t.Fatal(err)
	}
	if stage != 18 || w.Core.Peeps[0].TownStage != 18 || w.Core.Peeps[0].TownWork != 23 {
		t.Fatal("native stage/support evaluation changed unrelated work fields")
	}
	w.refreshNativeRecordImage()
	work, err := w.RecordImage.Read16(ref, 20)
	if err != nil || work != 23 {
		t.Fatalf("retained native town work differs: got %d, error %v", work, err)
	}
	farms := 0
	for _, cell := range w.Occupancy.Grid.Cells {
		if cell.Tile == 47 {
			farms++
		}
	}
	if farms != 49 {
		t.Fatalf("native farm compositor painted %d cells, expected 49", farms)
	}
	if err := w.clearNativeFarms(ref, 15); err != nil {
		t.Fatal(err)
	}
	for pos, cell := range w.Occupancy.Grid.Cells {
		if cell.Tile == 47 || w.NativeOverlays[pos] != 0 {
			t.Fatal("native full-stage farm/overlay cleanup incomplete")
		}
	}
}

func TestNativeTownWorldKeepsTreeAndRejectsBoulder(t *testing.T) {
	for _, kind := range []SceneryKind{SceneryTree, SceneryBoulder} {
		w := lightningWorld(t, 4311)
		pos := 32 + 32*64
		w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.InTown, TownWork: 23}}
		w.initializeNativeFollower(0)
		w.Scenery[0] = SceneryActor{Active: true, Kind: kind, X: 32, Y: 32, Age: 20, Animation: w.SceneryBank.Trees.Animations[0]}
		w.placeActor(NativeSceneryPool, 0, 8320, 8320)
		stage, err := w.evaluateNativeTown(nativeActorReference(NativeFollowerPool, 0))
		if err != nil {
			t.Fatal(err)
		}
		if kind == SceneryTree && stage != 18 || kind == SceneryBoulder && stage != 0 {
			t.Fatalf("native support with scenery %x gives stage %d", kind, stage)
		}
	}
}

func TestNativeTownWorkCounterWrapsBeforeProduction(t *testing.T) {
	w := lightningWorld(t, 4311)
	var err error
	w.Core.OlympianTowns, err = DecodeTownRules(testBundle(t).Executable, w.Landscape)
	if err != nil {
		t.Fatal(err)
	}
	w.bindNativeTownEvaluator()
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.InTown, MovementSpeed: 20, TownStage: 18, TownWork: 65535}}
	w.initializeNativeFollower(0)
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].TownWork != 0 || w.Core.Peeps[0].Population != 1000 {
		t.Fatalf("native word wrap produced work %d and population %d", w.Core.Peeps[0].TownWork, w.Core.Peeps[0].Population)
	}
	w.refreshNativeRecordImage()
	work, err := w.RecordImage.Read16(nativeActorReference(NativeFollowerPool, 0), 20)
	if err != nil || work != 0 {
		t.Fatalf("retained wrapped native work differs: got %d, error %v", work, err)
	}
}

func TestNativeTownOverlaysSurviveSave(t *testing.T) {
	w := lightningWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.InTown, TownWork: 23}}
	w.initializeNativeFollower(0)
	if _, err := w.evaluateNativeTown(nativeActorReference(NativeFollowerPool, 0)); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("native structure overlays/entry metadata changed after save")
	}
}

func TestNativeTownSaveRejectsUnknownOverlay(t *testing.T) {
	w := lightningWorld(t, 4311)
	snapshot := w.Snapshot()
	snapshot.NativeOverlays[32+32*64] = 35
	if _, err := Restore(testBundle(t), snapshot); err == nil {
		t.Fatal("save accepted an overlay outside the native artwork table")
	}
}

func TestNativeTownTileZeroSurvivesRaisedLandAndSave(t *testing.T) {
	w := lightningWorld(t, 4311)
	const x, y = 32, 32
	if w.nativeTileAt(x, y) == 0 {
		t.Fatal("test requires raised flat land before the native write")
	}
	w.writeNativeTownTile(x, y, 0)
	w.reconcileActorGraph()
	if w.nativeTileAt(x, y) != 0 || w.Occupancy.Grid.Cells[x+y*64].Tile != 0 {
		t.Fatal("explicit native tile zero was replaced by geometric terrain")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if restored.nativeTileAt(x, y) != 0 || restored.Occupancy.Grid.Cells[x+y*64].Tile != 0 {
		t.Fatal("saved explicit native tile zero was replaced by geometric terrain")
	}
}

func TestLightningTownRecoveryUsesNativeFortyNineFarmCompositor(t *testing.T) {
	w := lightningWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.InTown, MovementSpeed: 20, TownStage: 18, TownWork: 23}}
	w.initializeNativeFollower(0)
	ref := nativeActorReference(NativeFollowerPool, 0)
	v, ok := w.lightningRecord(ref)
	if !ok {
		t.Fatal("town record missing")
	}
	v.Kind, v.State, v.Animation, v.EffectReference = 4, 0x1e, 0x744, nativeActorReference(NativeEffectPool, 0)
	w.setLightningRecord(ref, v)
	w.Core.GameTurn = 122
	w.Core.TickWithComputer([2]bool{})
	if w.LightningVictims[0].Active || w.Core.Peeps[0].Population != 934 || w.Core.Peeps[0].TownWork != 23 || w.Core.Peeps[0].TownStage != 18 {
		t.Fatal("native living-town recovery/stage/work differs")
	}
	farms := 0
	for _, cell := range w.Occupancy.Grid.Cells {
		if cell.Tile == 47 {
			farms++
		}
	}
	if farms != 49 || w.NativeEntries[0].Actor.Founded46 != 123 {
		t.Fatalf("native recovery farms/tick differs: farms %d tick %d", farms, w.NativeEntries[0].Actor.Founded46)
	}
}
