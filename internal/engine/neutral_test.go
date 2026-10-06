package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNeutralMovementOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/neutral_motion_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name         string
			Kind         NeutralKind
			X, Y, VX, VY int
			Seed         uint32
			Tile         uint8
			RNG          uint32
			Calls        []struct {
				Name string
				X, Y int
			}
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 216 {
		t.Fatal("neutral corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := &World{}
			w.random = randomState(test.Seed)
			for at := range w.Tiles {
				w.Tiles[at] = Cell{Code: test.Tile, Shape: test.Tile & 15}
			}
			id := w.allocate(Follower{Owner: 2, X: uint8(test.X >> 8), Y: uint8(test.Y >> 8), State: Walking, Neutral: NeutralState{Kind: test.Kind}, positionX: test.X, positionY: test.Y, positionSet: true, velocityX: test.VX, velocityY: test.VY})
			w.linkFollower(id)
			w.tickNeutral(id)
			expectedX, expectedY := test.X+test.VX, test.Y+test.VY
			inside := expectedX >= 0 && expectedY >= 0 && expectedX < MapSize*256 && expectedY < MapSize*256
			if inside {
				f := w.Followers[id]
				if f.positionX != expectedX || f.positionY != expectedY {
					t.Fatal("neutral movement")
				}
			} else if w.Followers[id].State != Inactive {
				t.Fatal("out-of-map neutral survived")
			}
			// The source traces stop at separate creator callbacks. Integrated child
			// creators draw their own randomness, so only their boundary is excluded.
			childCreator := false
			for _, call := range test.Calls {
				if call.Name == "whirlwind" || call.Name == "fire-column" {
					childCreator = true
				}
			}
			if !childCreator && uint32(w.random) != test.RNG {
				t.Fatalf("rng%x source%x", w.random, test.RNG)
			}
		})
	}
}
