package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestTownSupportOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/town_support_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name                                        string
			X, Y, Owner, Stage, Clock, Default, Support int
			Cells                                       []struct{ Index, Tile, Head int }
			Actors                                      []struct {
				Slot                                           int
				Town                                           bool
				Owner, Stage, X, Y, Population, Next, Previous int
			}
			Result int
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) < 450 {
		t.Fatal("settlement corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := &World{Landscape: testLandscape(), Tick: uint64(test.Clock)}
			for at := range w.Tiles {
				w.Tiles[at] = Cell{Code: uint8(test.Default)}
			}
			w.Followers[1] = Follower{Owner: uint8(test.Owner), X: uint8(test.X), Y: uint8(test.Y), State: Town, Stage: uint8(test.Stage), Population: 934}
			w.Occupants[test.X+test.Y*MapSize] = 1
			if test.Support >= 0 {
				for _, d := range townFootprint[:min(test.Support, 49)] {
					x, y := test.X+d[0], test.Y+d[1]
					if inside(x, y) {
						w.Tiles[x+y*MapSize].Code = 15
					}
				}
			}
			for _, actor := range test.Actors {
				state := Walking
				if actor.Town {
					state = Town
				}
				w.Followers[actor.Slot] = Follower{Owner: uint8(actor.Owner), X: uint8(actor.X), Y: uint8(actor.Y), State: state, Stage: uint8(actor.Stage), Population: actor.Population, NextFollower: actor.Next, PreviousFollower: actor.Previous}
			}
			for _, cell := range test.Cells {
				w.Tiles[cell.Index].Code = uint8(cell.Tile)
				w.Occupants[cell.Index] = uint16(cell.Head)
			}
			if got := w.EvaluateTown(1); got != test.Result {
				t.Fatalf("stage%d; source%d", got, test.Result)
			}
		})
	}
}

func TestTownEvaluationDemotesCompetingTownInsteadOfRejectingProximity(t *testing.T) {
	w := testFlatWorld()
	a := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Town, Population: 100, Stage: 9})
	w.linkFollower(a)
	b := w.allocate(Follower{Owner: 1, X: 21, Y: 20, State: Town, Population: 200, Stage: 9})
	w.linkFollower(b)
	w.Tick = uint64(a) & 3
	if stage := w.EvaluateTown(a); stage != 18 {
		t.Fatalf("ownstage%d", stage)
	}
	if w.Followers[b].State != Walking || w.Followers[b].Population != 200 || w.Followers[b].Work != 0 {
		t.Fatal("competing town was not demoted with population intact")
	}
}

func TestTownSupportUsesParcelPropertiesInsteadOfEqualHeight(t *testing.T) {
	w := testFlatWorld()
	id := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Town, Population: 100})
	w.linkFollower(id)
	// Adjacent flat parcels may lie at different elevations. The original
	// support scan reads the ground class and does not compare vertex heights.
	w.Tiles[19+19*MapSize] = Cell{Corners: [4]uint8{2, 2, 2, 2}, BaseAltitude: 1, Shape: 15, Code: 15}
	w.Tick = 1
	if got := w.EvaluateTown(id); got != 18 {
		t.Fatalf("stage%d", got)
	}
}
