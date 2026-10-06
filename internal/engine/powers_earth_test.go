package engine

import (
	"crypto/sha256"
	"fmt"
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

func TestWallCrossingRetainsStrictSideDependentThresholds(t *testing.T) {
	for owner := range 2 {
		for _, xp := range []uint8{0, 32, 255} {
			w := testFlatWorld()
			wall := 0
			w.Earth.Walls[wall] = WallActor{Active: true, Owner: uint8(owner ^ 1), X: 32, Y: 32}
			id := addFollower(w, 31, 32, owner, 1, Walking)
			w.Players[owner].Experience[Earth] = xp
			bonus := ([2]int{256, 512}[owner] + int(xp)) * 128
			for _, c := range []struct {
				population int
				want       WallCrossing
			}{{bonus + 3000, WallBlocked}, {bonus + 3001, WallClimb}, {bonus + 20000, WallClimb}, {bonus + 20001, WallBreak}} {
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
