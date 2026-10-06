package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestWindMixedActorCrossingsMatchOriginalNumbers(t *testing.T) {
	var corpus struct {
		Cases []struct {
			Input struct {
				Name        string
				Owner, X, Y uint8
				Direction   uint16
				Victims     []struct {
					Reference   uint16
					Kind, Owner uint8
					X, Y        uint16
					Population  int32
				}
			}
			Creation struct {
				Records []struct {
					Reference uint16
					Raw       [52]uint8
				}
			}
			Trace []struct {
				Tick    int
				RNG     uint32
				Records []struct {
					Reference uint16
					Raw       [52]uint8
				}
			}
		}
	}
	raw, err := os.ReadFile("../populous2/testdata/hurricane_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, test := range corpus.Cases {
		if len(test.Input.Victims) != 1 || test.Input.Victims[0].Kind != 2 {
			continue
		}
		t.Run(test.Input.Name, func(t *testing.T) {
			v := test.Input.Victims[0]
			w := testFlatWorld()
			w.random = 4311
			// The original wind fixtures also contain the two divine markers. Their
			// moves affect traversal order when they cross before the follower.
			for _, record := range test.Creation.Records {
				b := record.Raw
				if b[0] == 20 && b[12] >= 1 && b[12] <= 2 {
					index := int(b[12] - 1)
					x, y := int(binary.BigEndian.Uint16(b[6:])), int(binary.BigEndian.Uint16(b[8:]))
					w.Magnets[index] = MagnetActor{X: x, Y: y, Owner: uint8(index)}
					w.Actors.Link(ActorRef{ActorMagnet, uint16(index)}, x, y)
				}
			}
			id := addFollower(w, int(v.X>>8), int(v.Y>>8), int(v.Owner)-1, int(v.Population), Walking)
			w.Followers[id].positionX, w.Followers[id].positionY, w.Followers[id].positionSet = int(v.X), int(v.Y), true
			w.Actors.Link(ActorRef{ActorFollower, uint16(id)}, int(v.X), int(v.Y))
			if err := w.CastWind(int(test.Input.Owner)-1, int(test.Input.X), int(test.Input.Y), int(test.Input.Direction/2)); err != nil {
				t.Fatal(err)
			}
			for _, tick := range test.Trace {
				w.tickWind(0)
				for _, record := range tick.Records {
					if record.Reference != v.Reference {
						continue
					}
					f := w.Followers[id]
					if f.State == Inactive {
						if record.Raw[12] != 0 {
							t.Fatal("wind removed a follower early")
						}
						continue
					}
					x, y := binary.BigEndian.Uint16(record.Raw[6:]), binary.BigEndian.Uint16(record.Raw[8:])
					if uint16(f.positionX) != x || uint16(f.positionY) != y || uint32(w.random) != tick.RNG {
						t.Fatalf("tick%d: position%d,%d expected%d,%d RNG%x/%x", tick.Tick, f.positionX, f.positionY, x, y, uint32(w.random), tick.RNG)
					}
				}
			}
		})
		checked++
	}
	if checked != 12 {
		t.Fatal("incomplete original wind cases", checked)
	}
}
