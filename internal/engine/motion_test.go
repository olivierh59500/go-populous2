package engine

import "testing"

func TestWalkingLegUsesSourceSubcellUnitsAndCrossingAdmission(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
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
