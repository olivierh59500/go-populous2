package engine

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFireWorldRetainsVictimsOccupancyUntilDeathAnimationFinishes(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 32, 32, 0, 100, Walking)
	b := addFollower(w, 32, 32, 1, 100, Walking)
	outside := addFollower(w, 33, 32, 1, 100, Walking)
	w.Players[0].Leader = a
	w.Players[0].Statistics.Metric = 100
	if hits := w.BurnFireCell(32, 32); hits != 2 {
		t.Fatalf("fire damage hit %d actors, want both allegiances", hits)
	}
	if w.Players[0].Leader != 0 || w.Players[0].Statistics.LeaderLosses != 1 || w.Players[0].Statistics.Metric != 88 || !w.Followers[a].CleanupPrepared {
		t.Fatal("fire hit did not prepare retained cleanup exactly once")
	}
	if w.Followers[outside].Population != 100 || w.Cell(32, 32).Code != 95 || w.NatureTownAllowed(0, 32, 32) {
		t.Fatal("fire crossed a parcel boundary or scorched land still supported a town")
	}
	for range 8 {
		w.stepFollower(a)
		w.stepFollower(b)
	}
	if w.Followers[a].State == Inactive || w.Followers[b].State == Inactive || w.Occupants[32+32*MapSize] == 0 {
		t.Fatal("dying actors released their allocation or occupancy too early")
	}
	w.stepFollower(a)
	w.stepFollower(b)
	if w.Followers[a].State != Inactive || w.Followers[b].State != Inactive || w.Occupants[32+32*MapSize] != 0 || w.Players[0].Leader != 0 {
		t.Fatal("completed fire death did not perform full follower cleanup")
	}
}

func TestFireWorldAllHeroImmunitiesAndTreeSpread(t *testing.T) {
	for _, kind := range []HeroKind{HeroPerseus, HeroAdonis, HeroHeracles, HeroOdysseus, HeroAchilles, HeroHelen} {
		w := testFlatWorld()
		id := addFollower(w, 32, 32, 0, 100, Walking)
		w.Followers[id].Hero.Kind = kind
		hits := w.BurnFireCell(32, 32)
		if kind == HeroAchilles {
			if hits != 0 || w.Followers[id].Population != 100 || w.FireDamage.Deaths[id].Mode != FireVictimAlive {
				t.Fatal("Achilles lost his fire immunity")
			}
		} else if hits != 1 || w.FireDamage.Deaths[id].Frames != 9 {
			t.Fatalf("hero %d did not retain the direct-fire death lifecycle", kind)
		}
		w = testFlatWorld()
		id = addFollower(w, 32, 32, 0, 100, Walking)
		w.Followers[id].Hero.Kind = kind
		if w.SpreadTreeFireCell(32, 32) != 0 || w.Followers[id].Population != 100 {
			t.Fatalf("neighboring tree fire incorrectly damaged hero %d", kind)
		}
	}
}

func TestFireWorldTownDestructionClearsOnlyItsOwnedFootprint(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 1000, Town)
	w.Followers[id].Stage = 18
	for _, d := range townFootprint {
		w.Farms[32+d[0]+(32+d[1])*MapSize] = 1
	}
	w.Farms[30+30*MapSize] = 2
	w.Farms[40+40*MapSize] = 1
	if w.BurnFireCell(32, 32) != 1 || w.Followers[id].CombatAftermath.Kind != CombatTownCollapse || w.Followers[id].CombatAftermath.Frames != 13 {
		t.Fatal("stage eighteen town did not enter its original ruin animation")
	}
	for _, d := range townFootprint {
		at := 32 + d[0] + (32+d[1])*MapSize
		if at == 30+30*MapSize {
			if w.Farms[at] != 2 || w.Nature.Ground[at].Mark == GroundScorched {
				t.Fatal("neighboring enemy farm was cleared with the destroyed town")
			}
			continue
		}
		if w.Farms[at] != 0 || w.Nature.Ground[at].Mark != GroundScorched {
			t.Fatal("destroyed town retained owned farmland")
		}
	}
	if w.Farms[40+40*MapSize] != 1 {
		t.Fatal("town destruction crossed its exact support footprint")
	}
}

func TestFireTownRetainsRuinsAndSpreadsOnlyCardinalCollateral(t *testing.T) {
	w := testFlatWorld()
	town := addFollower(w, 32, 32, 0, 1000, Town)
	w.Followers[town].Stage = 18
	cardinal := addFollower(w, 32, 31, 1, 100, Walking)
	diagonal := addFollower(w, 33, 33, 1, 100, Walking)
	hero := addFollower(w, 33, 32, 1, 100, Walking)
	w.Followers[hero].Hero.Kind = HeroPerseus
	neighborTown := addFollower(w, 31, 32, 1, 1000, Town)
	w.Followers[neighborTown].Stage = 9
	w.Nature.Scenery[0] = SceneryActor{Kind: SceneryTree, X: 32, Y: 33, Age: 0}
	w.Actors.Link(ActorRef{Kind: ActorScenery, Index: 0}, 32*256+128, 33*256+128)
	w.Players[0].Leader = town
	if w.BurnFireCell(32, 32) != 1 {
		t.Fatal("town did not receive direct fire")
	}
	if w.Players[0].Leader != 0 {
		t.Fatal("retained town hit did not clear its leader at hit time")
	}
	w.stepFollower(town)
	if w.Followers[cardinal].CombatAftermath.Kind != CombatCollateralDeath || w.Followers[neighborTown].CombatAftermath.Kind != CombatTownCollapse || w.Followers[hero].Population != 100 || w.Followers[diagonal].Population != 100 || w.Nature.Scenery[0].Kind != SceneryBurningTree {
		t.Fatal("fire town cardinal collateral differs")
	}
	for range 12 {
		w.stepFollower(town)
	}
	if f := w.Followers[town]; f.State == Inactive || f.CombatAftermath.Kind != CombatTownRuin || f.CombatAftermath.RuinTime != 399 {
		t.Fatal("town destruction animation discarded retained ruins", f.CombatAftermath)
	}
	var ids [FollowerCapacity]int
	if w.FollowersAt(32, 32, ids[:]) == 0 {
		t.Fatal("retained ruins lost occupancy")
	}
	for range 398 {
		w.stepFollower(town)
	}
	if w.Followers[town].State == Inactive {
		t.Fatal("ruin expired before its last countdown pass")
	}
	w.stepFollower(town)
	if w.Followers[town].State != Inactive || w.FollowersAt(32, 32, ids[:]) != 0 {
		t.Fatal("expired ruin retained allocation or occupancy")
	}
}

func TestRepeatedFireOnRetainedTownChargesCleanupOnlyOnce(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 1000, Town)
	w.Followers[id].Stage = 9
	w.Players[0].Leader = id
	w.Players[0].Statistics.Metric = 100
	if w.BurnFireCell(32, 32) != 1 {
		t.Fatal("initial town hit rejected")
	}
	stats := w.Players[0].Statistics
	for range 20 {
		w.BurnFireCell(32, 32)
	}
	if w.Players[0].Statistics != stats {
		t.Fatal("repeated fire charged a retained town's cleanup again")
	}
	for range 430 {
		w.stepFollower(id)
	}
	if w.Players[0].Statistics != stats {
		t.Fatal("final unlink repeated the town's hit-time cleanup")
	}
}

func TestPrivateFireTownCollapseMatchesOriginalAftermathTimers(t *testing.T) {
	path := os.Getenv("POPULOUS2_TOWN_AFTERMATH_TRACE")
	if path == "" {
		t.Skip("set POPULOUS2_TOWN_AFTERMATH_TRACE for private destruction lifecycle comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name               string
				Owner, Stage, Tile uint8
				X, Y               int
				Neighbors          []any
			}
			Trace []struct {
				Tick    int
				Records []struct {
					Reference uint16
					Raw       [52]uint8
				}
			}
		}
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range catalog.Cases {
		i := c.Input
		if !strings.HasPrefix(i.Name, "town-owner") || i.Owner < 1 || i.Owner > 2 || len(i.Neighbors) != 0 {
			continue
		}
		t.Run(i.Name, func(t *testing.T) {
			w := testFlatWorld()
			id := addFollower(w, i.X, i.Y, int(i.Owner-1), 1000, Town)
			w.Followers[id].Stage = i.Stage
			if hits := w.BurnFireCell(i.X, i.Y); hits != 1 {
				t.Fatal("source-equivalent town hit was rejected")
			}
			for _, frame := range c.Trace {
				w.stepFollower(id)
				var expected [52]uint8
				found := false
				for _, record := range frame.Records {
					if record.Reference == 52 {
						expected = record.Raw
						found = true
					}
				}
				if !found {
					t.Fatal("source fixture omitted destroyed town")
				}
				f := w.Followers[id]
				if (f.State != Inactive) != (expected[12] != 0) {
					t.Fatal("town retained allocation differs", frame.Tick)
				}
				if expected[12] == 0 {
					continue
				}
				if expected[22] == 48 {
					if f.CombatAftermath.Kind != CombatTownRuin || f.CombatAftermath.RuinTime != int(int16(uint16(expected[20])<<8|uint16(expected[21]))) {
						t.Fatal("source town ruin timer differs", frame.Tick, f.CombatAftermath)
					}
				} else if expected[22] == 40 {
					if f.CombatAftermath.Kind != CombatTownCollapse {
						t.Fatal("source collapse phase differs", frame.Tick)
					}
				}
			}
		})
		checked++
	}
	if checked != 38 {
		t.Fatal("source town stage/owner coverage incomplete", checked)
	}
}

func TestFireWorldSharedPoolReusesVelocityAndKeepsAdmissionCostSeparate(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	w.Players[0].Mana = 123456
	w.effects.Slots[0].LastVelocityX, w.effects.Slots[0].LastVelocityY = 17, -19
	if err := w.CastFireColumn(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	e := w.Fire.Columns[0]
	if e.VX != 17 || e.VY != -19 || w.Players[0].Mana != 123456 {
		t.Fatal("fire creation lost shared recycled velocities or charged outside Cast")
	}
	for range EffectCapacity - 1 {
		w.allocateEffect(EffectStorm, 1)
	}
	before := w.Fire
	if err := w.CastFireColumn(0, 32, 32); err == nil || w.Fire != before || w.Players[0].Mana != 123456 {
		t.Fatal("full-pool admission changed existing effects or the mana ledger")
	}
}

func TestFireWorldLavaBurningDamagesPopulationGradually(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 1000, Walking)
	w.pushFireParcel(32, 32, 20, 0)
	if w.Followers[id].Population != 1000 || w.FireDamage.Deaths[id].Mode != FireVictimBurning {
		t.Fatal("lava entry killed a follower instead of beginning gradual burning")
	}
	w.stepFollower(id)
	if w.Followers[id].Population != 934 || w.Followers[id].State == Inactive {
		t.Fatal("lava burning did not apply population>>4 plus four")
	}
	for pass := 0; pass < 200 && w.Followers[id].State != Inactive; pass++ {
		w.stepFollower(id)
	}
	if w.Followers[id].State != Inactive || w.Occupants[32+32*MapSize] != 0 {
		t.Fatal("depleted burning follower retained occupancy")
	}
}

func TestLavaTownIsCleanedBeforeItsBurningTransition(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 1000, Town)
	w.Followers[id].Stage = 9
	w.Players[0].Leader = id
	w.Players[0].Statistics.Metric = 100
	w.pushFireParcel(32, 32, 20, 0)
	if f := w.Followers[id]; f.Population != 0 || !f.CleanupPrepared || f.CombatAftermath.Kind != CombatAftermathNone || w.FireDamage.Deaths[id].Mode != FireVictimBurning || w.Players[0].Leader != 0 {
		t.Fatal("lava town did not complete source destruction before burning")
	}
	stats := w.Players[0].Statistics
	w.stepFollower(id)
	if w.Followers[id].State != Inactive || w.Players[0].Statistics != stats {
		t.Fatal("lava town terminal cleanup charged death twice")
	}
}

func TestLavaPushKeepsMixedAndFollowerChainsConsistent(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 32, 32, 0, 1000, Walking)
	b := addFollower(w, 33, 32, 1, 1000, Walking)
	w.Followers[a].initialisePosition()
	w.Followers[a].positionX = 32*256 + 250
	w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(a)}, w.Followers[a].positionX, w.Followers[a].positionY)
	w.pushFireParcel(32, 32, 20, 0)
	if f := w.Followers[a]; f.X != 33 || f.positionX != 33*256+14 {
		t.Fatal("lava push lost fractional crossing")
	}
	var ids [FollowerCapacity]int
	count := w.FollowersAt(33, 32, ids[:])
	foundA, foundB := false, false
	for _, id := range ids[:count] {
		foundA = foundA || id == a
		foundB = foundB || id == b
	}
	if !foundA || !foundB || w.FollowersAt(32, 32, ids[:]) != 0 {
		t.Fatal("lava push discarded a follower chain member")
	}
	if _, err := w.Snapshot().Restore(); err != nil {
		t.Fatal("lava crossing produced invalid mixed-actor continuation", err)
	}
}

func TestFireWorldConsecratedFollowerResistsFireBeforeHeroChecks(t *testing.T) {
	for _, kind := range []HeroKind{HeroNone, HeroPerseus, HeroAdonis, HeroHeracles, HeroOdysseus, HeroAchilles, HeroHelen} {
		w := testFlatWorld()
		id := addFollower(w, 32, 32, 0, 1000, Walking)
		w.Followers[id].Hero.Kind = kind
		w.Followers[id].Consecrated = true
		before := w.Followers[id]
		if w.BurnFireCell(32, 32) != 0 || w.Followers[id] != before {
			t.Fatalf("direct fire damaged a consecrated follower of hero kind %d", kind)
		}
		w.pushFireParcel(32, 32, 20, 0)
		if w.Followers[id] != before || w.FireDamage.Deaths[id].Mode != FireVictimAlive {
			t.Fatalf("lava pushed or damaged a consecrated follower of hero kind %d", kind)
		}
	}
}

func TestFireWorldLavaCreatesBasaltImmediatelyAtBaseLifetime(t *testing.T) {
	w := testFlatWorld()
	for i := range w.Heights {
		w.Heights[i] = 0
	}
	w.rebuildCells()
	w.Players[0].Experience[Water] = 255
	result := w.Fire.CreateLava(0, 32, 32, 1, worldFireHabitat{w})
	if result != 1 || w.effects.Slots[0].Kind != EffectBasalt || !w.Water.Basalt[0].Active || w.Water.Basalt[0].Life != 100 {
		t.Fatal("lava did not immediately create base-lifetime basalt without water experience")
	}
	if w.Cell(32, 32).Code != 224 {
		t.Fatal("lava's basalt handoff did not paint the parcel immediately")
	}
}

func TestLavaTraversesMixedMarkersSceneryAndFollowerChains(t *testing.T) {
	w := testFlatWorld()
	follower := addFollower(w, 20, 20, 0, 1000, Walking)
	w.Nature.Scenery[0] = SceneryActor{Kind: SceneryTree, X: 20, Y: 20, Age: 0}
	w.Actors.Link(ActorRef{Kind: ActorScenery, Index: 0}, 20*256+128, 20*256+128)
	w.Magnets[0] = MagnetActor{Owner: 0, X: 20*256 + 128, Y: 20*256 + 128}
	w.Actors.Link(ActorRef{Kind: ActorMagnet, Index: 0}, w.Magnets[0].X, w.Magnets[0].Y)
	w.pushFireParcel(20, 20, 20, 0)
	if w.Magnets[0].X != 20*256+148 || w.Nature.Scenery[0].Kind != SceneryBurningTree || w.FireDamage.Deaths[follower].Mode != FireVictimBurning || w.Followers[follower].positionX != 20*256+148 {
		t.Fatal("lava omitted non-follower mixed parcel actors")
	}
}

func TestFireDamageUsesActualMixedMembershipWithoutTouchingUnmappedActors(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 100, Walking)
	b := w.allocate(Follower{Owner: 1, X: 20, Y: 20, State: Walking, Population: 100})
	// The second actor is allocated but has not been inserted into any parcel.
	if hits := w.damageFireParcel(20, 20, false); hits != 1 || w.Followers[a].Population != 0 || w.Followers[b].Population != 100 {
		t.Fatal("fire scanned pool coordinates instead of actual map membership")
	}
}
