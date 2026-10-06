package engine

import "testing"

func TestOrdinarySearchRetainsPreferredOrderAndTownRadius(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Followers[id].Search = 2
	x, y, ok := w.chooseMove(id)
	if !ok || x != 19 || y != 19 {
		t.Fatalf("preferred order %d,%d", x, y)
	}
	for _, d := range preferredSettlements[:8] {
		w.Tiles[20+d[0]+(20+d[1])*MapSize] = Cell{}
	}
	w.Followers[id].Search = 10
	x, y, ok = w.chooseMove(id)
	if !ok || x != 18 || y != 18 {
		t.Fatalf("stage-five emigrant search %d,%d", x, y)
	}
}

func TestJoinAndFightSelectMatchingActorBeforeFallback(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	friend := addFollower(w, 21, 20, 0, 100, Walking)
	enemy := addFollower(w, 20, 21, 1, 100, Town)
	w.SetMode(0, Join)
	x, y, ok := w.chooseMove(id)
	if !ok || x != int(w.Followers[friend].X) || y != int(w.Followers[friend].Y) {
		t.Fatal("join did not target a friendly walking group")
	}
	w.SetMode(0, Fight)
	x, y, ok = w.chooseMove(id)
	if !ok || x != int(w.Followers[enemy].X) || y != int(w.Followers[enemy].Y) {
		t.Fatal("fight did not target enemy town")
	}
}

func TestPressureFallbackChoosesLowestVisitPressure(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	for _, d := range decisionNeighbours[:8] {
		at := 20 + d[0] + (20+d[1])*MapSize
		w.Tiles[at].Corners = [4]uint8{1, 2, 2, 1}
		w.Pressure[at] = 200
	}
	w.Pressure[21+21*MapSize] = 0
	x, y, ok := w.chooseMove(id)
	if !ok || x != 21 || y != 21 {
		t.Fatalf("pressure fallback %d,%d", x, y)
	}
}

func TestMagnetPlacementDoesNotChangeTacticalMode(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Players[0].Leader = id
	w.Players[0].Mana = 1000
	w.Level.Players[0].Powers[PapalMagnet] = true
	if err := w.Cast(0, PapalMagnet, PowerTarget{X: 30, Y: 30}); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mode != Settle || w.Players[0].RallyX != 30 || w.Players[0].Mana != 900 {
		t.Fatal("magnet changed tactic or price")
	}
}
