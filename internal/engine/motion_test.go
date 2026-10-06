package engine

import (
	"fmt"
	"testing"
)

func TestWalkingLegUsesSourceSubcellUnitsAndCrossingAdmission(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Players[0].Mode = Rally
	if !w.beginLeg(id, 21, 20) {
		t.Fatal("leg rejected")
	}
	f := &w.Followers[id]
	if f.legRemaining != 12 || f.velocityX != 20 || f.velocityY != 0 {
		t.Fatal("speed/timer do not use 256 cell units")
	}
	for pass := 1; pass <= 6; pass++ {
		w.advanceLeg(id)
		if f.X != 20 {
			t.Fatalf("crossed cell early at pass %d", pass)
		}
	}
	x, y := f.Position()
	if x != 20.96875 || y != 20.5 {
		t.Fatalf("subcell position %.5f,%.5f", x, y)
	}
	w.advanceLeg(id)
	if f.X != 21 || w.Occupants[20+20*MapSize] != 0 || w.Occupants[21+20*MapSize] != uint16(id) {
		t.Fatal("crossing did not move occupancy")
	}
	for range 5 {
		w.advanceLeg(id)
	}
	x, y = f.Position()
	if x != 21.4375 || y != 20.5 {
		t.Fatalf("twelfth source movement %.5f,%.5f", x, y)
	}
	w.advanceLeg(id)
	if f.moving {
		t.Fatal("timer did not expire on thirteenth pass")
	}
}

func TestBlockedLegDoesNotCommitWaterCrossing(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Tiles[21+20*MapSize] = Cell{}
	w.beginLeg(id, 21, 20)
	for range 8 {
		w.advanceLeg(id)
	}
	f := w.Followers[id]
	if f.X != 20 || w.Occupants[20+20*MapSize] != uint16(id) || w.Occupants[21+20*MapSize] != 0 {
		t.Fatal("blocked crossing changed position or occupancy")
	}
}

func TestZeroSpeedDoesNotFabricateMovement(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Followers[id].MovementSpeed = 0
	if w.beginLeg(id, 21, 20) {
		t.Fatal("zero-speed leg was silently accelerated")
	}
}

func TestCellEntryWrapsPressureWithoutLoweringSourcePressure(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Pressure[20+20*MapSize] = 16
	w.Pressure[21+20*MapSize] = 248
	w.beginLeg(id, 21, 20)
	for range 7 {
		w.advanceLeg(id)
	}
	if w.Pressure[20+20*MapSize] != 16 || w.Pressure[21+20*MapSize] != 0 {
		t.Fatal("pressure did not retain source and wrap destination byte")
	}
}

func TestHostileWallCrossingUsesStrengthAndBreaksBeforeMoving(t *testing.T) {
	for _, test := range []struct {
		population      int
		crosses, broken bool
	}{{2999, false, false}, {3000, true, false}, {20000, true, false}, {20001, false, true}} {
		t.Run(fmt.Sprint(test.population), func(t *testing.T) {
			w := testFlatWorld()
			id := addFollower(w, 20, 20, 0, test.population, Walking)
			w.Earth.Walls[0] = WallActor{Active: true, Owner: 1, X: 21, Y: 20}
			w.beginLeg(id, 21, 20)
			for range 7 {
				w.advanceLeg(id)
			}
			if (w.Followers[id].X == 21) != test.crosses || w.Earth.Walls[0].Broken != test.broken {
				t.Fatal("wall admission or break order")
			}
			if test.broken && w.Followers[id].positionX != 20*256+248 {
				t.Fatal("breaking update committed crossing")
			}
		})
	}
}

func TestNewbornSearchMovesBeforeClaimingSettlementAndKeepsParentTown(t *testing.T) {
	w := testFlatWorld()
	parent := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Town, Population: 4000, Stage: 18, Work: 7, MovementSpeed: 20})
	w.linkFollower(parent)
	w.Tick = 1
	w.stepFollower(parent)
	child := parent + 1
	if w.Followers[child].State != Walking || w.Followers[child].positionX != w.Followers[parent].positionX {
		t.Fatal("newborn position")
	}
	w.stepFollower(child)
	if w.Followers[child].State != Walking || w.Followers[parent].State != Town || !w.Followers[child].moving || w.Followers[child].positionX == w.Followers[parent].positionX {
		t.Fatal("newborn claimed the parent parcel instead of moving")
	}
}

func TestOrdinarySearchUsesCellSignsRatherThanTargetCentreFractions(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	f := &w.Followers[id]
	f.positionX = 20*256 + 7
	f.positionY = 20*256 + 247
	f.positionSet = true
	if !w.beginSearchLeg(id, 21, 20) || f.velocityX != 20 || f.velocityY != 0 || f.legRemaining != 12 {
		t.Fatal("cross-cell search changed fractional axis velocity")
	}
	f.moving = false
	if !w.beginSearchLeg(id, 20, 20) || f.velocityX != -20 || f.velocityY != -20 || f.legRemaining != 12 {
		t.Fatal("same-cell search did not use maximum fractional distance")
	}
}
