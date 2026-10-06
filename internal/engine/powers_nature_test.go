package engine

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func countTrees(w *World) int {
	n := 0
	for _, a := range w.Nature.Scenery {
		if a.Kind == SceneryTree {
			n++
		}
	}
	return n
}

func TestTreesUseSharedSceneryBudgetAndAge(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	w.Players[0].Mana = 12345
	before := w.Heights
	if err := w.CastTrees(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if countTrees(w) == 0 {
		t.Fatal("no forest was planted")
	}
	if w.Heights != before || w.Players[0].Mana != 12345 {
		t.Fatal("nature implementation changed geometry or charged outside Cast")
	}
	for _, a := range w.Nature.Scenery {
		if a.Kind == SceneryNone {
			continue
		}
		if a.Age != 24 || a.Variant > 3 || w.NatureTownAllowed(0, int(a.X), int(a.Y)) {
			t.Fatal("tree lifecycle or settlement blocking wrong")
		}
	}
	for tick := uint64(1); tick <= 4; tick++ {
		w.Tick = tick
		w.tickNature()
	}
	for _, a := range w.Nature.Scenery {
		if a.Kind == SceneryTree && a.Age != 23 {
			t.Fatal("tree aged faster than four passes")
		}
	}
	full := testFlatWorld()
	for id := range full.Nature.Scenery {
		full.Nature.Scenery[id] = SceneryActor{Kind: SceneryBoulder, X: 1, Y: 1, Age: 24}
	}
	if err := full.CastTrees(0, 32, 32); !errors.Is(err, ErrNatureNoChange) || countTrees(full) != 0 {
		t.Fatal("forest exceeded shared scenery capacity")
	}
}

func TestForestBurialRecoversWithoutDeletingGeometry(t *testing.T) {
	w := testFlatWorld()
	w.Nature.Scenery[0] = SceneryActor{Kind: SceneryTree, X: 10, Y: 10, Age: 0}
	original := w.Tiles[10+10*MapSize]
	w.Tiles[10+10*MapSize] = Cell{}
	w.Tick = 1
	w.tickNature()
	if a := w.Nature.Scenery[0]; a.Age != -1 || a.Kind != SceneryTree {
		t.Fatal("hazard did not start tree removal")
	}
	w.Tiles[10+10*MapSize] = original
	w.Tick = 2
	w.tickNature()
	if w.Nature.Scenery[0].Age != 2 {
		t.Fatal("dry land did not recover the tree")
	}
	w.Tiles[10+10*MapSize] = Cell{}
	for tick := uint64(3); tick < 30; tick++ {
		w.Tick = tick
		w.tickNature()
	}
	if w.Nature.Scenery[0].Kind != SceneryNone {
		t.Fatal("buried tree did not leave scenery pool")
	}
	if w.Tiles[10+10*MapSize] != (Cell{}) {
		t.Fatal("tree removal altered terrain")
	}
}

func TestTreesCanCoexistWithSwampAndKeepAgingNormally(t *testing.T) {
	w := testFlatWorld()
	for at := range w.Nature.Ground {
		w.Nature.Ground[at] = GroundParcel{Mark: GroundSwamp}
	}
	w.random = 4311
	if err := w.CastTrees(0, 32, 32); err != nil || countTrees(w) == 0 {
		t.Fatal("swamp incorrectly rejected original tree decoration")
	}
	for tick := uint64(1); tick <= 96; tick++ {
		w.Tick = tick
		w.tickNature()
	}
	for _, a := range w.Nature.Scenery {
		if a.Kind == SceneryTree && a.Age != 0 {
			t.Fatal("swamp incorrectly buried its trees")
		}
	}
}

func TestBurningMatureForestSpreadsOnlyAfterItsFourFrames(t *testing.T) {
	w := testFlatWorld()
	w.Nature.Scenery[0] = SceneryActor{Kind: SceneryBurningTree, X: 32, Y: 32, Age: 0}
	w.Nature.Scenery[1] = SceneryActor{Kind: SceneryTree, X: 31, Y: 32, Age: 0}
	ordinary := addFollower(w, 32, 31, 0, 100, Walking)
	hero := addFollower(w, 33, 32, 0, 100, Walking)
	w.Followers[hero].Hero.Kind = HeroPerseus
	for range 3 {
		w.tickNature()
	}
	if w.Nature.Scenery[1].Kind != SceneryTree || w.Followers[ordinary].State != Walking {
		t.Fatal("tree fire spread before ending")
	}
	w.tickNature()
	if w.Nature.Scenery[0].Age != -1 || w.Nature.Scenery[1].Kind != SceneryBurningTree {
		t.Fatal("mature tree did not spread to cardinal neighbour")
	}
	if w.Followers[ordinary].State != Ruin || w.Followers[hero].State != Walking {
		t.Fatal("tree spread failed ordinary damage or hero exemption")
	}
	for range 23 {
		w.tickNature()
	}
	if w.Nature.Scenery[0].Kind != SceneryNone {
		t.Fatal("burned scenery did not complete removal")
	}
}

func TestImmatureBurningForestRemovesWithoutSpreading(t *testing.T) {
	w := testFlatWorld()
	w.Nature.Scenery[0] = SceneryActor{Kind: SceneryBurningTree, X: 32, Y: 32, Age: 10}
	w.Nature.Scenery[1] = SceneryActor{Kind: SceneryTree, X: 31, Y: 32, Age: 0}
	for range 14 {
		w.tickNature()
	}
	if w.Nature.Scenery[0].Kind != SceneryNone || w.Nature.Scenery[1].Kind != SceneryTree {
		t.Fatal("immature burning tree used mature spread sequence")
	}
}

func TestRenewLandKeepsOccupantsAndRestoresSettlementSupport(t *testing.T) {
	w := testFlatWorld()
	w.random = 173
	for y := 25; y <= 39; y++ {
		for x := 25; x <= 39; x++ {
			w.Nature.Ground[x+y*MapSize] = GroundParcel{Mark: GroundSwamp}
		}
	}
	id := addFollower(w, 32, 32, 1, 100, Walking)
	before := w.Heights
	if err := w.CastFlowers(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	restored := 0
	for at, p := range w.Nature.Ground {
		if p.Mark != GroundFlowers {
			continue
		}
		restored++
		if !w.NatureTownAllowed(0, at%MapSize, at/MapSize) {
			t.Fatal("flowers did not restore support")
		}
		if cell := w.Cell(at%MapSize, at/MapSize); cell.Code != 245 || !cell.IsFlat() {
			t.Fatal("flowers changed geometry or lacked original artwork code")
		}
	}
	if restored == 0 || w.Occupants[32+32*MapSize] != uint16(id) || w.Followers[id].Population != 100 || w.Heights != before {
		t.Fatal("renew did not preserve town occupants and terrain")
	}
}

func TestSwampRejectsOccupantsAndDoesNotUseExperienceForAttemptCount(t *testing.T) {
	a, b := testFlatWorld(), testFlatWorld()
	a.random, b.random = 991, 991
	b.Players[0].Experience[Plants] = 255
	for _, w := range []*World{a, b} {
		for y := 28; y <= 36; y++ {
			for x := 28; x <= 36; x++ {
				if (x+y)%3 == 0 {
					addFollower(w, x, y, 0, 100, Walking)
				}
			}
		}
		if err := w.CastSwamp(0, 32, 32); err != nil {
			t.Fatal(err)
		}
	}
	if a.Nature.Ground != b.Nature.Ground || a.random != b.random {
		t.Fatal("plant experience changed swamp sampling")
	}
	count := 0
	for at, p := range a.Nature.Ground {
		if p.Mark != GroundSwamp {
			continue
		}
		count++
		if a.Occupants[at] != 0 || a.NatureTownAllowed(0, at%MapSize, at/MapSize) {
			t.Fatal("swamp planted under occupants or remained settlement land")
		}
		if a.Cell(at%MapSize, at/MapSize).Code != 168 {
			t.Fatal("swamp artwork code absent")
		}
	}
	if count == 0 {
		t.Fatal("no swamp was planted")
	}
}

func TestFungusCollectsSeedsWithoutRestartingItsClock(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].Experience[Plants] = 96
	if err := w.CastFungus(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	id := int(w.Nature.PendingFungus[0]) - 1
	for range 35 {
		w.tickNature()
	}
	if w.Nature.Fungi[id].Wait != 65 {
		t.Fatal("fungus collection timing wrong")
	}
	if err := w.CastFungus(0, 36, 33); err != nil {
		t.Fatal(err)
	}
	controller := w.Nature.Fungi[id]
	if controller.Wait != 65 || controller.Period != 7 || controller.MaxX != 38 || controller.MaxY != 35 {
		t.Fatalf("seed extension restarted collection or lost bounds: %+v", controller)
	}
	for range 64 {
		w.tickNature()
	}
	if !w.Nature.Fungi[id].Collecting {
		t.Fatal("fungus evolved before collection ended")
	}
	w.tickNature()
	if w.Nature.Fungi[id].Collecting || w.Nature.PendingFungus[0] != 0 {
		t.Fatal("collecting slot did not become evolving")
	}
	if err := w.CastFungus(0, 45, 45); err != nil {
		t.Fatal(err)
	}
	if next := int(w.Nature.PendingFungus[0]) - 1; next == id || next < 0 {
		t.Fatal("new seed reused an evolving controller")
	}
}

func TestFungusPlantsBeforeSharedPoolAdmission(t *testing.T) {
	w := testFlatWorld()
	for range EffectCapacity {
		if w.allocateEffect(EffectFireColumn, 0) < 0 {
			t.Fatal("test effect allocation failed")
		}
	}
	before := w.random
	if err := w.CastFungus(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if w.Nature.Ground[32+32*MapSize].Mark != GroundFungusFresh || w.Nature.PendingFungus[0] != 0 || w.random != before {
		t.Fatal("full pool prevented fungus planting or consumed random input")
	}
}

func TestMatureGroundHazardsRetainAndThenCleanUpFollower(t *testing.T) {
	for _, mark := range []GroundMark{GroundSwamp, GroundFungusYoung, GroundFungusDying} {
		w := testFlatWorld()
		id := addFollower(w, 32, 32, 0, 100, Walking)
		w.Players[0].Leader = id
		w.Nature.Ground[32+32*MapSize] = GroundParcel{Mark: mark}
		if !w.EnterNatureHazard(id) || w.Followers[id].State != Ruin || w.Occupants[32+32*MapSize] != uint16(id) {
			t.Fatal("hazard did not retain follower artwork")
		}
		if !w.EnterNatureHazard(id) || w.Followers[id].State == Inactive {
			t.Fatal("death ended before its second frame")
		}
		if !w.EnterNatureHazard(id) || w.Followers[id].State != Inactive || w.Occupants[32+32*MapSize] != 0 || w.Players[0].Leader != 0 {
			t.Fatal("death cleanup retained an actor or leader")
		}
	}
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 100, Walking)
	w.Nature.Ground[32+32*MapSize] = GroundParcel{Mark: GroundFungusFresh}
	if w.EnterNatureHazard(id) {
		t.Fatal("fresh fungus killed a follower")
	}
}

func TestNatureHeroImmunityAndDeathTiming(t *testing.T) {
	for _, mark := range []GroundMark{GroundSwamp, GroundFungusYoung} {
		for kind := HeroPerseus; kind <= HeroHelen; kind++ {
			w := testFlatWorld()
			id := addFollower(w, 32, 32, 0, 100, Walking)
			w.Followers[id].Hero.Kind = kind
			w.Nature.Ground[32+32*MapSize] = GroundParcel{Mark: mark}
			entered := w.EnterNatureHazard(id)
			if kind == HeroAdonis {
				if entered || w.Followers[id].State != Walking {
					t.Fatal("Adonis lost nature immunity")
				}
				continue
			}
			if !entered {
				t.Fatal("vulnerable hero ignored nature hazard")
			}
			hazard := HazardFungus
			if mark == GroundSwamp {
				hazard = HazardSwamp
			}
			duration := w.Followers[id].HazardDeathFrames(hazard)
			for frame := 1; frame < duration; frame++ {
				w.EnterNatureHazard(id)
				if w.Followers[id].State == Inactive {
					t.Fatal("hero death ended too early")
				}
			}
			w.EnterNatureHazard(id)
			if w.Followers[id].State != Inactive {
				t.Fatal("hero did not leave pool after death sequence")
			}
		}
	}
}

func TestShallowSwampUsesVictimScenarioAndRestoresOnlyItsParcel(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[1].Scenario.ShallowSwamps = true
	blue := addFollower(w, 31, 32, 0, 100, Walking)
	red := addFollower(w, 32, 32, 1, 100, Walking)
	for _, x := range []int{31, 32} {
		w.Nature.Ground[x+32*MapSize] = GroundParcel{Mark: GroundSwamp, Owner: 0}
	}
	if !w.EnterNatureHazard(blue) || !w.EnterNatureHazard(red) {
		t.Fatal("swamp did not admit its victims")
	}
	if w.Nature.Ground[31+32*MapSize].Mark != GroundSwamp {
		t.Fatal("swamp used caster or opposite-side scenario")
	}
	if p := w.Nature.Ground[32+32*MapSize]; p.Mark != GroundRestored || w.Cell(32, 32).Code != 15 {
		t.Fatal("shallow swamp did not restore underlying land")
	}
	if w.Followers[red].State != Ruin {
		t.Fatal("shallow swamp incorrectly rescued victim")
	}
}

func TestEmptyGroundCastsSucceedButEmptyForestIsNotAdmitted(t *testing.T) {
	w := &World{}
	if err := w.CastTrees(0, 32, 32); !errors.Is(err, ErrNatureNoChange) {
		t.Fatal("empty forest charge admission differs")
	}
	for _, cast := range []func(int, int, int) error{w.CastFlowers, w.CastSwamp, w.CastFungus} {
		if err := cast(0, 32, 32); err != nil {
			t.Fatal("valid ground cast was not an admitted no-op")
		}
		if err := cast(2, 32, 32); err == nil {
			t.Fatal("invalid player cast accepted")
		}
	}
}

func TestNatureEligibilityUsesActiveArtworkAndClearsPriorEffects(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	for at := range w.Water.Painted {
		w.Water.Painted[at] = true
		w.Water.Tiles[at] = 224
	}
	if err := w.CastFlowers(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if err := w.CastSwamp(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if err := w.CastFungus(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	for _, p := range w.Nature.Ground {
		if p.Mark != GroundNone {
			t.Fatal("nature used hidden flat geometry instead of basalt shape")
		}
	}
	for at := range w.Water.Painted {
		w.Water.Painted[at] = false
		w.FireDamage.Painted[at] = true
		w.FireDamage.Tiles[at] = 220
	}
	if err := w.CastFlowers(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	restored := 0
	for at, p := range w.Nature.Ground {
		if p.Mark == GroundFlowers {
			restored++
			if w.FireDamage.Painted[at] || w.Water.Painted[at] || w.Cell(at%MapSize, at/MapSize).Code != 245 {
				t.Fatal("restored land retained a previous fire/water overlay")
			}
		}
	}
	if restored == 0 {
		t.Fatal("renew did not restore eligible lava terrain")
	}
}

func TestPrivateFungusMapTraces(t *testing.T) {
	path := os.Getenv("POPULOUS2_FUNGUS_TRACE")
	if path == "" {
		t.Skip("set POPULOUS2_FUNGUS_TRACE for private reference map comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Name       string
			Experience uint8
			Initial    []struct {
				X, Y int
				Tile uint8
			}
			ActorInitial []struct {
				Index int
				Raw   string
			}
			Actions   []struct{ Tick, Player, X, Y int }
			Snapshots []struct {
				Tick  int
				Tiles []uint8
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, reference := range catalog.Cases {
		if reference.Name == "row-edge-neighbor-alias" || reference.Name == "bottom-edge-first-generation" {
			continue
		}
		t.Run(reference.Name, func(t *testing.T) {
			w := testFlatWorld()
			w.Players[0].Experience[Plants] = reference.Experience
			for _, point := range reference.Initial {
				w.Nature.Ground[point.X+point.Y*MapSize].Mark = GroundMark(point.Tile-145) + GroundFungusFresh
			}
			for _, initial := range reference.ActorInitial {
				raw, err := hex.DecodeString(initial.Raw)
				if err != nil || len(raw) != 32 {
					t.Fatal("invalid private fungus state")
				}
				owner := raw[12] - 1
				id := w.allocateEffect(EffectFungus, owner)
				if id != initial.Index {
					t.Fatal("unexpected private fungus slot")
				}
				x, y := int(raw[6]), int(raw[8])
				w.Nature.Fungi[id] = FungusController{Active: true, Owner: owner, Period: int(raw[18]), Wait: int(raw[21]), MinX: x, MinY: y, MaxX: x + int(raw[26]), MaxY: y + int(raw[27]), AgeMinX: int(raw[7]), AgeMinY: int(raw[9]), AgeMaxX: int(raw[7]) + int(raw[28]), AgeMaxY: int(raw[9]) + int(raw[29])}
			}
			next := 0
			for tick := 0; next < len(reference.Snapshots); tick++ {
				for _, action := range reference.Actions {
					if action.Tick == tick {
						if err := w.CastFungus(action.Player, action.X, action.Y); err != nil {
							t.Fatal(err)
						}
					}
				}
				if expected := reference.Snapshots[next]; expected.Tick == tick {
					actual := make([]byte, MapSize*MapSize)
					for at := range actual {
						actual[at] = w.Cell(at%MapSize, at/MapSize).Code
					}
					if !bytes.Equal(actual, expected.Tiles) {
						for at, code := range expected.Tiles {
							if actual[at] != code {
								t.Fatalf("tick%d parcel%d,%d=%d expected%d", tick, at%MapSize, at/MapSize, actual[at], code)
							}
						}
					}
					next++
				}
				w.tickNature()
			}
		})
	}
}
