package engine

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func testLandscape() Landscape {
	var l Landscape
	for stage := 1; stage < TownStages; stage++ {
		l.WorkTicks[stage] = 8
		l.EmigrationDivisor[stage] = 3
		l.PopulationLimit[stage] = 100 + stage*100
		l.PopulationAdd[stage] = 1 + stage/2
		l.ManaAdd[stage] = stage
	}
	l.PopulationLimit[18] = 4000
	l.ManaAdd[18] = 100
	return l
}
func testFlatWorld() *World {
	w := &World{Landscape: testLandscape(), Editor: true}
	for i := range w.Heights {
		w.Heights[i] = 1
	}
	w.rebuildCells()
	w.Level.Players[0].Scenario.BuildAnywhere = true
	w.Level.Players[1].Scenario.BuildAnywhere = true
	w.Players[0].Mode = Settle
	w.Players[1].Mode = Settle
	return w
}
func addFollower(w *World, x, y, owner, population int, state FollowerState) int {
	id := w.allocate(Follower{Owner: uint8(owner), X: uint8(x), Y: uint8(y), PreviousX: uint8(x), PreviousY: uint8(y), State: state, Population: population, MovementSpeed: 20})
	w.Occupants[x+y*MapSize] = uint16(id)
	return id
}

func TestSettlementNineteenStagesAndEconomy(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 900, Town)
	if stage := w.TownStage(0, 32, 32, id); stage != 18 {
		t.Fatalf("full support: %d", stage)
	}
	w.Followers[id].Stage = 18
	for range 7 {
		w.Step()
	}
	if w.Followers[id].Population != 900 || w.Players[0].Mana != 0 {
		t.Fatal("town worked early")
	}
	w.Step()
	if w.Followers[id].Population != 910 || w.Players[0].Mana != 100 {
		t.Fatalf("stage eighteen economy: %+v", w.Followers[id])
	}
	// The inner nine cells produce stage nine; the outer sixteen support
	// stage seventeen, and the final clearance ring alone unlocks eighteen.
	for _, d := range townFootprint[9:] {
		at := 32 + d[0] + (32+d[1])*MapSize
		w.Tiles[at] = Cell{}
	}
	if stage := w.TownStage(0, 32, 32, id); stage != 9 {
		t.Fatalf("nine cells: %d", stage)
	}
}

func TestEmigrationPopulationAccountingAndPoolExhaustion(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 4000, Town)
	w.Followers[id].Work = 7
	w.Step()
	if w.Followers[id].Population != 2664 {
		t.Fatalf("parent population: %d", w.Followers[id].Population)
	}
	var children int
	for n, f := range w.Followers {
		if n != id && f.State != Inactive {
			children++
			if f.Population != 1336 || f.Weapons != 18 || f.Search != 36 {
				t.Fatalf("child: %+v", f)
			}
		}
	}
	if children != 1 {
		t.Fatal("expected one emigrant")
	}
	w = testFlatWorld()
	id = addFollower(w, 32, 32, 0, 4000, Town)
	w.Followers[id].Work = 7
	for n := 2; n < FollowerCapacity; n++ {
		w.Followers[n] = Follower{Owner: 0, State: Walking, Population: 1, X: 1, Y: 1}
	}
	w.stepFollower(id)
	if w.Followers[id].Population != 4000 {
		t.Fatal("full pool changed parent population")
	}
}

func TestWalkingFoundingAndRallyOrders(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 8, 8, 0, 100, Walking)
	addFollower(w, 50, 50, 1, 100, Town)
	for range 20 {
		if w.Followers[id].State != Town {
			w.Step()
		}
	}
	if w.Followers[id].State != Town {
		t.Fatal("walking group did not found a town")
	}
	if !w.SetRally(0, 20, 20) {
		t.Fatal("rally rejected")
	}
	for range 180 {
		w.Step()
	}
	f := w.Followers[id]
	if f.State == Town || int(f.X) <= 8 || int(f.Y) <= 8 {
		t.Fatalf("rally did not release and move founder: %+v", f)
	}
}

func TestFightOrdersResolveCombatAndOccupancy(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 100, Walking)
	d := addFollower(w, 21, 20, 1, 2, Walking)
	w.Players[0].Mode = Fight
	w.Players[1].Mode = Fight
	for range 40 {
		w.Step()
	}
	if w.Followers[d].State != Inactive || w.Followers[a].State == Inactive {
		t.Fatal("stronger group failed to win")
	}
	if w.Occupants[21+20*MapSize] == uint16(d) || w.Players[0].BattlesWon != 1 {
		t.Fatal("defeated group still occupied terrain")
	}
}

func TestCampaignTypedDecodingAndSubworldSeeds(t *testing.T) {
	data := make([]byte, 50000)
	for record := 0; record < 200; record++ {
		p := data[record*250:]
		binary.BigEndian.PutUint16(p, 4)
		binary.BigEndian.PutUint16(p[2:], 250)
		binary.BigEndian.PutUint16(p[4:], 20)
		binary.BigEndian.PutUint16(p[8:], 1000)
		binary.BigEndian.PutUint16(p[10:], 1)
		p[22] = 1
		binary.BigEndian.PutUint32(p[182:], 4311)
		copy(p[186:], "Test opponent")
	}
	levels, err := DecodeCampaign(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 1000 || levels[4].Seed != 4311+4*727 || levels[5].Seed != 4311 || levels[0].Players[0].Mana != 1000 || levels[0].Players[0].MovementSpeed != 20 || !levels[0].Players[0].Powers[0] {
		t.Fatal("campaign fields")
	}
	n, ok := DecodeLevelCode(levels[77].Code)
	if !ok || n != 77 {
		t.Fatal("campaign code round trip")
	}
	data[22] = 2
	if _, err := DecodeCampaign(data); err == nil {
		t.Fatal("malformed campaign accepted")
	}
}

func TestReservedPowersAreRejectedWithoutConsumingMana(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].Mana = 60000
	w.Level.Players[0].Powers[5] = true
	if err := w.Cast(0, PowerID(5), PowerTarget{}); err == nil || w.Players[0].Mana != 60000 {
		t.Fatal("reserved power appeared successful")
	}
	if len(Powers) != 29 {
		t.Fatal("power catalogue must contain all original powers")
	}
}

func TestDeterministicTwoMinuteSimulation(t *testing.T) {
	level := Level{Seed: 4311, Players: [2]PlayerOptions{{Groups: 3, Population: 100, Mana: 1000, MovementSpeed: 20}, {Groups: 3, Population: 100, Mana: 1000, MovementSpeed: 20, ReactionDelay: 30}}}
	a, err := NewWorld(level, testLandscape())
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewWorld(level, testLandscape())
	if err != nil {
		t.Fatal(err)
	}
	for range 1500 {
		a.Step()
		b.Step()
	}
	if !reflect.DeepEqual(*a, *b) {
		t.Fatal("simulation is not deterministic")
	}
	for owner, summary := range a.Summaries() {
		if summary.Towns == 0 || summary.Population <= 300 || summary.Mana <= 1000 {
			t.Fatalf("side %d did not develop: %+v", owner, summary)
		}
	}
	for at, id := range a.Occupants {
		if id != 0 {
			f := a.Followers[id]
			if f.State == Inactive || int(f.X)+int(f.Y)*MapSize != at {
				t.Fatal("stale occupancy")
			}
		}
	}
}

func TestTerrainPriceCountsPropagationAndExperience(t *testing.T) {
	w := &World{}
	w.Level.Players[0].Scenario.BuildAnywhere = true
	w.Players[0].Mana = 10000
	if !w.RaiseAt(0, 32, 32) || w.Players[0].Mana != 9980 {
		t.Fatal("one-corner raise did not cost twenty ledger units")
	}
	if !w.RaiseAt(0, 32, 32) || w.Players[0].Mana != 9800 {
		t.Fatalf("nine-corner propagation price: %d", w.Players[0].Mana)
	}
	w.Players[0].Experience[People] = 64
	if w.PowerCost(0, PapalMagnet) != 92 {
		t.Fatal("experience discount or quarter-mana conversion")
	}
	w.Players[0].Mana = 20
	if !w.RaiseAt(0, 32, 32) || w.Players[0].Mana != 0 {
		t.Fatal("propagated order did not clamp exhausted ledger")
	}
}

func BenchmarkWorldStep(b *testing.B) {
	level := Level{Seed: 4311, Players: [2]PlayerOptions{{Groups: 10, Population: 100, Mana: 1792, MovementSpeed: 20}, {Groups: 3, Population: 50, Mana: 112, MovementSpeed: 20}}}
	w, err := NewWorld(level, testLandscape())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Step()
	}
}

func TestEvacuateReleasesTownWithoutDuplicatingPopulation(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Town)
	if !w.Evacuate(id) || w.Followers[id].State != Walking || w.Followers[id].Population != 100 {
		t.Fatal("evacuation failed")
	}
	w.Step()
	if w.Followers[id].State != Walking || w.Summaries()[0].Population != 100 || w.Summaries()[0].Groups != 1 {
		t.Fatal("evacuation immediately recreated town or duplicated people")
	}
}

func TestTownEmigrantStartsAtParentPositionAndRightClickIsDeferred(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Town)
	w.Followers[id].Work = 7
	w.Followers[id].positionX = 20*256 + 135
	w.Followers[id].positionY = 20*256 + 147
	w.Followers[id].positionSet = true
	if !w.Sprog(0, 20, 20) || !w.Followers[id].ForceEmigration {
		t.Fatal("right-click sprog")
	}
	beforeMana := w.Players[0].Mana
	w.stepFollower(id)
	child := id + 1
	if w.Followers[child].State != Walking || w.Followers[child].positionX != 20*256+135 || w.Followers[child].positionY != 20*256+147 || w.Followers[id].ForceEmigration {
		t.Fatal("newborn did not retain parent position or clear request")
	}
	if w.Players[0].Mana != beforeMana+w.Landscape.ManaAdd[18] {
		t.Fatal("sprog spent extra mana")
	}
}

func TestRareTownBirthCreatesNeutralInventionAtSourceSlot(t *testing.T) {
	w := testFlatWorld()
	parent := 1
	w.Followers[parent] = Follower{Owner: 0, X: 32, Y: 32, State: Town, Stage: 18, Population: 4000, MovementSpeed: 20, Work: 7, positionSet: true, positionX: 32*256 + 128, positionY: 32*256 + 128}
	w.Occupants[32+32*MapSize] = uint16(parent)
	for id := 2; id < 250; id++ {
		w.Followers[id] = Follower{Owner: 0, State: Ruin, Population: 1}
	}
	w.Tick = 1
	w.random = randomState(4311)
	expected := w.random
	expected.next()
	bits := expected.next()
	w.stepFollower(parent)
	if w.Followers[250].State != Walking || w.Followers[251].Owner != 2 || w.Followers[251].Neutral.Kind != NeutralKind(((bits%12)&^1)/2+1) || w.NeutralBirthDeadline != 51 || w.random != expected {
		t.Fatal("rare neutral birth slot, selector, deadline or RNG")
	}
}

func TestFreshFollowerAppearanceUsesOriginalSlotVariant(t *testing.T) {
	w := testFlatWorld()
	for slot := 1; slot <= 17; slot++ {
		id := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Walking, Population: 100, MovementSpeed: 20})
		if id != slot || w.Followers[id].AppearanceVariant != uint8((slot&14)/2) {
			t.Fatalf("slot%d variant%d", slot, w.Followers[id].AppearanceVariant)
		}
	}
	if err := w.EditorPlaceFollower(1, 30, 30, 100); err != nil {
		t.Fatal(err)
	}
	if w.Followers[18].AppearanceVariant != 1 {
		t.Fatal("editor follower did not use fresh source variant")
	}
}

func TestHeroConversionPreservesOrdinaryAppearanceData(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Followers[id].AppearanceVariant = 7
	if err := w.ConvertHero(id, HeroPerseus); err != nil {
		t.Fatal(err)
	}
	if w.Followers[id].AppearanceVariant != 7 {
		t.Fatal("hero conversion destroyed retained appearance metadata")
	}
}
