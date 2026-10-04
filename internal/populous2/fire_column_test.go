package populous2

import (
	"bytes"
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestFireColumnNativePhasesAndExperience(t *testing.T) {
	w := flatGroundWorld(t)
	w.Experience[0][Fire] = 255
	if !w.Cast(0, FireColumn, Target{X: 32, Y: 32}) {
		t.Fatal("native fire column rejected")
	}
	a := &w.NativeEffects[0]
	if a.State != 2 || a.Animation != 0x1a0 || a.Life != 455 || a.Speed != 16 || a.Timer != 1 {
		t.Fatalf("wrong original creation: %+v", a)
	}
	w.tickNativeEffects()
	if a.Animation != 0x1a4 || a.State != 2 || a.Life != 455 {
		t.Fatal("first native intro tick changed lifetime or frame")
	}
	for range 8 {
		w.tickNativeEffects()
	}
	if a.State != 4 || a.Animation != 0x4bc || a.Life != 454 || a.Timer != 30 {
		t.Fatalf("intro transition did not fall through: %+v", a)
	}
	a.Life = 1
	w.tickNativeEffects()
	if a.State != 6 || a.Animation != 0x664 {
		t.Fatal("expiry did not enter and advance outro in one call")
	}
	for range 9 {
		w.tickNativeEffects()
	}
	if a.Active {
		t.Fatal("native effect slot did not expire after outro")
	}
}

func TestFireColumnPoolWaterAndSavedContinuation(t *testing.T) {
	w := flatGroundWorld(t)
	for i := range w.NativeEffects {
		w.NativeEffects[i] = NativeEffectActor{Active: true, Kind: 0x22, Player: 0, State: 2, Animation: 0x1a0, Speed: 16, Life: 200, X: 32*256 + 128, Y: 32*256 + 128}
	}
	actors := w.NativeEffects
	mana := w.Core.Magnets[0].Mana
	random := w.Core.Snapshot().RNG
	if w.Cast(0, FireColumn, Target{X: 32, Y: 32}) || w.NativeEffects != actors || w.Core.Magnets[0].Mana != mana || w.Core.Snapshot().RNG == random {
		t.Fatal("full-pool native cast did not retain actors/mana and consume jitter draw")
	}
	w.NativeEffects[0].Active = false
	w.NativeEffects[0].VX = 9
	w.NativeEffects[0].VY = -9
	if !w.Cast(0, FireColumn, Target{X: 32, Y: 32}) || w.NativeEffects[0].VX != 9 || w.NativeEffects[0].VY != -9 {
		t.Fatal("native first free slot or retained velocity bytes lost")
	}
	w.NativeEffects = [NativeEffectCapacity]NativeEffectActor{}
	if !w.Cast(0, FireColumn, Target{X: 32, Y: 32}) {
		t.Fatal("funded effect rejected")
	}
	for range 12 {
		w.tickNativeEffects()
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 75 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("save changed fixed-point column/routing continuation")
	}
}

func TestFireColumnHitsBothSidesOnCurrentCellOnly(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove}, {Player: 1, Population: 100, AtPos: 2000, Flags: legacy.OnMove}, {Player: 1, Population: 100, AtPos: 2001, Flags: legacy.OnMove}, {Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove, Status: legacy.KnightStatus}}
	w.Heroes[3] = Hero{Spell: Achilles, Active: true, Player: 0, Population: 100}
	w.burnFireCell(2000%64, 2000/64)
	if w.Core.Peeps[0].Population != 0 || w.Core.Peeps[1].Population != 0 || w.Core.Peeps[2].Population != 100 || w.Core.Peeps[3].Population != 100 {
		t.Fatal("fire retained radius, owner filter, or lost Achilles immunity")
	}
	if len(w.FlameDeaths) != 2 || !w.Core.FollowerReserved(0) || !w.Core.FollowerReserved(1) {
		t.Fatal("native death actors vanished or slots reused immediately")
	}
	if w.nativeTileAt(2000%64, 2000/64) != 95 || w.Core.HabitatTerrainAllowed(0, 2000) {
		t.Fatal("burned ground still supports a settlement")
	}
	for range 50 {
		w.tickFlameDeaths()
	}
	if w.Core.FollowerReserved(0) || w.Core.FollowerReserved(1) {
		t.Fatal("completed death animation retained follower slot")
	}
}

func TestTreeFireSpreadsOnlyToFourAdjacentCells(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 1, Population: 100, AtPos: 32 + 31*64, Flags: legacy.OnMove}, {Player: 1, Population: 100, AtPos: 33 + 31*64, Flags: legacy.OnMove}, {Player: 0, Population: 100, AtPos: 33 + 32*64, Flags: legacy.OnMove, Status: legacy.KnightStatus}}
	w.Heroes[2] = Hero{Spell: Perseus, Active: true, Player: 0, Population: 100}
	w.spreadTreeFire(32, 32)
	if w.Core.Peeps[0].Population != 0 || w.Core.Peeps[1].Population != 100 || w.Core.Peeps[2].Population != 100 {
		t.Fatal("tree fire used a square area or ignored heroic protection")
	}
}

func TestOlderColumnSaveMigratesToNativePool(t *testing.T) {
	w := flatGroundWorld(t)
	s := w.Snapshot()
	s.Version = 6
	s.Effects = []Effect{{Spell: FireColumn, Player: 1, X: 32, Y: 32, DX: -1, DY: 1, Life: 15}}
	restored, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Effects) != 0 || !restored.NativeEffects[0].Active || restored.NativeEffects[0].Player != 1 || restored.NativeEffects[0].Life != 15 || restored.NativeEffects[0].VX != -16 {
		t.Fatal("old generic column retained or lost during migration")
	}
	if len(s.Effects) != 1 {
		t.Fatal("save migration mutated caller-owned effects")
	}
}
