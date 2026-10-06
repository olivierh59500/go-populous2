package engine

import (
	"encoding/json"
	"testing"
)

func mobileRightsView() Viewport {
	mask := make([]uint64, MapSize)
	for y := 20; y < 35; y++ {
		for x := 18; x < 39; x++ {
			if x+y >= 43 && x+y < 65 {
				mask[y] |= uint64(1) << uint(x)
			}
		}
	}
	return Viewport{X: 18, Y: 20, Width: 21, Height: 15, Visible: mask}
}

func TestMobileCursorRightsExcludeHiddenCullingParcels(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario = ScenarioOptions{}
	view := mobileRightsView()
	if !view.Valid() || !view.ContainsCell(38, 25) || view.ContainsCell(18, 20) {
		t.Fatal("projected visible mask differs from its parcel membership")
	}
	town := addFollower(w, 18, 20, 0, 100, Town)
	if rights := w.CursorTerrainRights(0, view); rights.BuildAnywhere || rights.SeaLevelOnly {
		t.Fatal("town in the culling rectangle but outside the playfield granted rights")
	}
	w.Followers[town].X, w.Followers[town].Y = 38, 25
	w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(town)}, 38*256+128, 25*256+128)
	if rights := w.CursorTerrainRights(0, view); !rights.BuildAnywhere {
		t.Fatal("visible town beyond the classic eight-parcel view did not grant rights")
	}
	if view.ContainsCorner(18, 20) || !view.ContainsCorner(39, 26) {
		t.Fatal("vertices did not follow their adjacent visible parcels")
	}
}

func TestMobileViewportRejectsMalformedAndExpandedMasks(t *testing.T) {
	valid := mobileRightsView()
	cases := []Viewport{
		{}, {X: 20, Y: 20, Size: 8, Width: 8},
		{X: 20, Y: 20, Width: 8, Height: 8},
		{X: 20, Y: 20, Width: 8, Height: 8, Visible: make([]uint64, MapSize)},
		{X: 60, Y: 20, Width: 8, Height: 8, Visible: valid.Visible},
		{X: 18, Y: 20, Size: 7},
	}
	for _, view := range cases {
		if view.Valid() || view.ContainsCorner(20, 20) || view.ContainsCell(20, 20) {
			t.Fatalf("malformed viewport accepted: %+v", view)
		}
	}
	for _, point := range [][2]int{{17, 23}, {39, 23}, {24, 19}, {24, 35}} {
		view := valid
		view.Visible = append([]uint64(nil), valid.Visible...)
		view.Visible[point[1]] |= uint64(1) << uint(point[0])
		if view.Valid() {
			t.Fatal("mask bit outside the declared rectangle accepted", point)
		}
	}
	all := make([]uint64, MapSize)
	for y := range all {
		all[y] = ^uint64(0)
	}
	if !(Viewport{Width: MapSize, Height: MapSize, Visible: all}).Valid() {
		t.Fatal("full-width row mask overflowed")
	}
}

func TestMobileViewportTerrainCommandRoundTrip(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario = ScenarioOptions{}
	w.Level.Players[0].Powers[RaiseLower] = true
	w.Players[0].Mana = 1000
	addFollower(w, 38, 25, 0, 100, Town)
	view := mobileRightsView()
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Viewport
	if err := json.Unmarshal(encoded, &decoded); err != nil || !decoded.Valid() {
		t.Fatal("mobile network geometry lost its checked bitset", err)
	}
	if err := w.CastFromViewport(0, RaiseLower, PowerTarget{X: 25, Y: 25}, decoded); err != nil {
		t.Fatal("visible mobile terrain command rejected", err)
	}
	before := w.Players[0].Mana
	if err := w.CastFromViewport(0, RaiseLower, PowerTarget{X: 18, Y: 20}, decoded); err == nil || w.Players[0].Mana != before {
		t.Fatal("hidden mobile target accepted or consumed mana")
	}
}
