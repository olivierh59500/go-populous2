package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestRoadJoinsUpdateBothSidesWithoutChangingOwners(t *testing.T) {
	w := testFlatWorld()
	if err := w.CastRoad(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if w.Cell(32, 32).Code != 201 {
		t.Fatal("isolated road art changed")
	}
	if err := w.CastRoad(1, 33, 32); err != nil {
		t.Fatal(err)
	}
	if w.Cell(33, 32).Code != 209 || w.Cell(32, 32).Code != 203 {
		t.Fatalf("reciprocal east/west connections wrong %d/%d", w.Cell(32, 32).Code, w.Cell(33, 32).Code)
	}
	if w.Earth.Roads[32+32*MapSize].Owner != 0 || w.Earth.Roads[33+32*MapSize].Owner != 1 {
		t.Fatal("road joining transferred ownership")
	}
	if !w.RemoveRoad(32, 32) || w.Cell(32, 32).Code != 15 {
		t.Fatal("uncharged road removal did not restore ground")
	}
	if w.Cell(33, 32).Code != 209 {
		t.Fatal("road removal unexpectedly recomputed neighboring road")
	}
}

func TestRoadFullMapMatchesOriginalFivePlacementFingerprint(t *testing.T) {
	w := testFlatWorld()
	for _, p := range [][2]int{{32, 32}, {33, 32}, {34, 32}, {34, 33}, {33, 33}} {
		if err := w.CastRoad(0, p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	var tiles [MapSize * MapSize]byte
	for at := range tiles {
		tiles[at] = w.Cell(at%MapSize, at/MapSize).Code
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(tiles[:])); got != "036d45628688268226ee231c53abcad3b5dbee6f240fa213eaca6cbc6b872d3d" {
		t.Fatalf("original road fingerprint differs: %s", got)
	}
}

func TestRoadsUseFourOriginalRampShapesAndRejectEnemyFarms(t *testing.T) {
	for index, shape := range [4]uint8{3, 6, 9, 12} {
		w := testFlatWorld()
		w.Tiles[32+32*MapSize].Code = shape
		if err := w.CastRoad(0, 32, 32); err != nil || w.Cell(32, 32).Code != 197+uint8(index) {
			t.Fatal("ramp road art differs", shape)
		}
	}
	w := testFlatWorld()
	w.Tiles[32+32*MapSize].Code = 63
	if err := w.CastRoad(0, 32, 32); err == nil {
		t.Fatal("road claimed enemy farm")
	}
	w.Nature.Ground[32+32*MapSize] = GroundParcel{Mark: GroundScorched}
	if err := w.CastRoad(0, 32, 32); err != nil || w.Nature.Ground[32+32*MapSize].Mark != GroundNone {
		t.Fatal("road did not replace eligible scorched overlay")
	}
}

func TestWallRequiresAConnectedChainAfterFirstPlacement(t *testing.T) {
	w := testFlatWorld()
	if err := w.CastWall(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if w.WallAt(32, 32) != 0 || !w.WallBlocksLightning(32, 32) {
		t.Fatal("first wall collision absent")
	}
	if err := w.CastWall(0, 40, 40); err == nil {
		t.Fatal("disconnected second wall accepted")
	}
	if w.Earth.WallHeads[0] != 2 || w.Earth.Walls[1].Active {
		t.Fatal("original failed-head update not retained")
	}
	if err := w.CastWall(0, 33, 32); err != nil {
		t.Fatal("neighboring wall rejected", err)
	}
	if a := w.Earth.Walls[1]; !a.Active || a.Variant != 4 || a.Next != 2 {
		t.Fatal("adjacent wall variant/head state differs", a)
	}
	for range 4 {
		w.tickWalls()
	}
	if w.Earth.Walls[0].Frame != 2 || w.Earth.Walls[1].Frame != 2 {
		t.Fatal("constructed walls did not hold final frame")
	}
}

func TestWallsOnRoadsCreateGatesButStillBlockLightning(t *testing.T) {
	w := testFlatWorld()
	if err := w.CastRoad(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if err := w.CastWall(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if a := w.Earth.Walls[0]; !a.Gate || a.Variant != 8 || !w.WallBlocksLightning(32, 32) {
		t.Fatal("gate animation or lightning blocker missing", a)
	}
	w.Earth.Walls[0].Broken = true
	if w.WallBlocksLightning(32, 32) {
		t.Fatal("broken wall still blocked lightning")
	}
}

func TestWallPoolAndUnsuitableGroundRemoval(t *testing.T) {
	w := testFlatWorld()
	for id := range w.Earth.Walls {
		w.Earth.Walls[id] = WallActor{Active: true, Owner: 0, X: 1, Y: 1}
	}
	if err := w.CastWall(0, 32, 32); err == nil {
		t.Fatal("wall exceeded its own 200 actor budget")
	}
	w = testFlatWorld()
	if err := w.CastWall(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	w.Nature.Ground[32+32*MapSize] = GroundParcel{Mark: GroundSwamp}
	w.tickWalls()
	if w.Earth.Walls[0].Active || w.Earth.WallHeads[0] != 0 {
		t.Fatal("swamp did not remove wall and construction head")
	}
}

func TestWallCrossingUsesOriginalMovementThresholds(t *testing.T) {
	for owner := range 2 {
		for _, xp := range []uint8{0, 32, 255} {
			w := testFlatWorld()
			wall := 0
			w.Earth.Walls[wall] = WallActor{Active: true, Owner: uint8(owner ^ 1), X: 32, Y: 32}
			id := addFollower(w, 31, 32, owner, 1, Walking)
			w.Players[owner].Experience[Earth] = xp
			bonus := int(xp) * 128
			for _, c := range []struct {
				population int
				want       WallCrossing
			}{{bonus + 2999, WallBlocked}, {bonus + 3000, WallClimb}, {bonus + 20000, WallClimb}, {bonus + 20001, WallBreak}} {
				w.Followers[id].Population = c.population
				if got := w.DecideWallCrossing(id, wall); got != c.want {
					t.Fatal("wall threshold differs", owner, xp, c.population, got, c.want)
				}
				w.Followers[id].Hero.Kind = HeroPerseus
				if got := w.DecideWallCrossing(id, wall); got != c.want {
					t.Fatal("hero bypassed wall comparison")
				}
			}
		}
	}
}

func TestBatholithMatchesOriginalRaisedTerrainAndBoulderChoices(t *testing.T) {
	for _, reference := range []struct {
		seed, rng uint32
		hash      string
		x, y      int
		variant   int
	}{
		{0, 0x7714b9fe, "d705eb09403358892d6eb41d50ec79ee50eb84f138fb79021fba252140673dcc", 0, 0, -1},
		{1, 0xa374e3e9, "f725e681fa541e31b43ca00a34f89092267859c5a74cdcec1e7f9a5a61357ecb", 0, 0, -1},
		{4311, 0x11428447, "85eea23a3bc05659cedf6f34e7ab1f0f22eab7533f1ac82092727b9f1b08631a", 35, 32, 0},
		{5038, 0x4ff81a8e, "85eea23a3bc05659cedf6f34e7ab1f0f22eab7533f1ac82092727b9f1b08631a", 29, 29, 1},
	} {
		w := testFlatWorld()
		w.random = randomState(reference.seed)
		if err := w.CastBatholith(0, 32, 32); err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(w.Heights[:])); got != reference.hash || uint32(w.random) != reference.rng {
			t.Fatal("original batholith geometry or random state differs", reference.seed, got, w.random)
		}
		if reference.variant >= 0 {
			a := w.Nature.Scenery[0]
			if a.Kind != SceneryBoulder || int(a.X) != reference.x || int(a.Y) != reference.y || int(a.Variant) != reference.variant {
				t.Fatal("batholith boulder differs", a)
			}
		} else if w.Nature.Scenery[0].Kind != SceneryNone {
			t.Fatal("raise branch created an additional boulder")
		}
	}
}

func TestEarthquakeBranchesThenFadesWithoutManaOrMemoryAliases(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	w.Players[0].Mana = 1000
	if err := w.CastEarthquake(0, 32, 32, 0); err != nil {
		t.Fatal(err)
	}
	if c := w.Cell(32, 32); c.Code != 172 {
		t.Fatal("initial earthquake crack absent")
	}
	for pass := 0; pass < 2; pass++ {
		for id := range w.effects.Slots {
			w.tickEarthEffect(id)
		}
	}
	if w.Earth.Quakes[0].Phase != QuakeWaiting {
		t.Fatal("quake did not leave growing branch")
	}
	if !w.Earth.Cracks[32+31*MapSize].Active {
		t.Fatal("quake did not extend in its cardinal direction")
	}
	for range 150 {
		for id := range w.effects.Slots {
			w.tickEarthEffect(id)
		}
	}
	for _, e := range w.Earth.Quakes {
		if e.Active {
			t.Fatal("quake remained after fade life")
		}
	}
	if w.Players[0].Mana != 1000 {
		t.Fatal("natural earthquake progress spent mana")
	}
	if !w.Earth.Cracks[32+32*MapSize].Active {
		t.Fatal("controller expiry removed persistent crack art")
	}
}

func TestEarthquakePoolExhaustionAndMapEdgeAreBounded(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	for range EffectCapacity {
		w.allocateEffect(EffectFungus, 0)
	}
	before := *w
	if err := w.CastEarthquake(0, 32, 32, 0); err != nil {
		t.Fatal("no-op quake cast was not admitted")
	}
	if *w != before {
		t.Fatal("exhausted quake wrote outside its effect pool")
	}
	w = testFlatWorld()
	w.random = 4311
	if err := w.CastEarthquake(0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	for range 120 {
		for id := range w.effects.Slots {
			w.tickEarthEffect(id)
		}
	}
	for _, e := range w.Earth.Quakes {
		if e.Active {
			t.Fatal("edge quake did not finish")
		}
	}
}

func TestPrivateEarthquakeGeometryAndRandomTraces(t *testing.T) {
	path := os.Getenv("POPULOUS2_QUAKE_TRACE")
	if path == "" {
		t.Skip("set POPULOUS2_QUAKE_TRACE for private quake comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type frame struct {
		Tick        int
		AltSHA256   string
		RNG         uint32
		ActiveCount int
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name, Mode                   string
				X, Y                         int
				Direction, Owner             uint8
				Strength                     uint16
				Seed                         uint32
				Ticks                        int
				Full, ParentOnly, EdgeParent bool
			}
			Creation frame
			Trace    []frame
		}
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range catalog.Cases {
		i := c.Input
		if i.Owner < 1 || i.Owner > 2 || i.Full || i.ParentOnly || i.EdgeParent {
			continue
		}
		t.Run(i.Name, func(t *testing.T) {
			w := &World{random: randomState(i.Seed)}
			for y := 0; y < CornerSize; y++ {
				for x := 0; x < CornerSize; x++ {
					h := 3
					switch i.Mode {
					case "water":
						h = 0
					case "slope":
						h = 1 + x/8
					case "checker":
						h = 3 + (x+y)%2
					case "hill":
						h = max(0, 8-max(abs(x-32), abs(y-32))/4)
					}
					w.Heights[x+y*CornerSize] = uint8(min(8, h))
				}
			}
			w.rebuildCells()
			w.createEarthquake(uint8(i.Owner-1), i.X, i.Y, int(i.Direction), int(i.Strength))
			compare := func(f frame) {
				if got := fmt.Sprintf("%x", sha256.Sum256(w.Heights[:])); got != f.AltSHA256 || uint32(w.random) != f.RNG {
					t.Fatalf("quake tick%d geometry/RNG differs: %s/%d expected%s/%d", f.Tick, got, w.random, f.AltSHA256, f.RNG)
				}
				active := 0
				for _, e := range w.Earth.Quakes {
					if e.Active {
						active++
					}
				}
				if active != f.ActiveCount {
					t.Fatalf("quake active count%d expected%d", active, f.ActiveCount)
				}
			}
			compare(c.Creation)
			for _, f := range c.Trace {
				for id := range w.effects.Slots {
					w.tickEarthEffect(id)
				}
				compare(f)
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("private quake catalog supplied no normal cases")
	}
}
