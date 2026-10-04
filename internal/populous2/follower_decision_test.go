package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeDecisionInput struct {
	Name                       string
	Mode                       uint16
	SearchIndex, Speed, Flags  uint8
	Seed                       uint32
	VX, VY                     int16
	DefaultHeader, DefaultTile uint8
	Cells                      []struct {
		X, Y         int
		Header, Tile uint8
	}
	Actors []struct {
		Reference   uint16
		Kind, Owner uint8
		Next        uint16
		X, Y        int
	}
}

type nativeDecisionActor struct {
	Kind, Owner, Flags, Speed, State, ReturnState, SearchIndex, Weapon uint8
	X, Y, Variant                                                      uint16
	VX, VY, Timer                                                      int16
	Population                                                         uint32
}

// These observations execute the original state2 instructions, stopping just
// before motion or delegated handlers. They preserve raw modes, mixed list
// ordering, inactive boulders, pressure ties and saturated road speed restore.
func TestFollowerDecisionAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_decision_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Input  nativeDecisionInput
			Route  string
			Target [2]uint16
			Actor  nativeDecisionActor
			RNG    uint32
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 51 {
		t.Fatal("native decision catalog incomplete")
	}
	rules, err := DecodeFollowerDecisionRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			var cells NativeOccupancyState
			for index := range cells.Cells {
				cells.Cells[index] = NativeOccupancyCell{Header: input.DefaultHeader, Tile: input.DefaultTile}
			}
			for _, cell := range input.Cells {
				cells.Cells[cell.X+cell.Y*64] = NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile}
			}
			records := make(map[NativeRecordReference]FollowerDecisionRecord)
			for _, a := range input.Actors {
				reference := NativeRecordReference(a.Reference)
				records[reference] = FollowerDecisionRecord{Kind: a.Kind, Owner: a.Owner, Next: NativeRecordReference(a.Next)}
				cell := &cells.Cells[a.X+a.Y*64]
				if cell.Head == 0 {
					cell.Head = reference
				}
			}
			cells.Cells[32+32*64].Head = 52
			records[52] = FollowerDecisionRecord{Kind: 2, Owner: 1}
			actor := FollowerMotionActor{Kind: 2, Player: 0, Flags: input.Flags, X: 8320, Y: 8320, VX: input.VX, VY: input.VY, Speed: input.Speed, State: 2, ReturnState: 2, Population: 1000}
			seed := input.Seed
			result, err := rules.Select(&actor, input.SearchIndex, NativeFollowerMode(input.Mode), &cells, FollowerDecisionCallbacks{
				Record: func(reference NativeRecordReference) (FollowerDecisionRecord, bool) {
					record, ok := records[reference]
					return record, ok
				},
				Random: func() int {
					if seed == 0 {
						seed = 0x00bc614e
					}
					seed *= 0xbb40e62d
					return int(seed >> 8 & 0x7fff)
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			kind := map[string]FollowerDecisionKind{"none": FollowerNoTarget, "preferred": FollowerPreferredTarget, "pressure": FollowerPressureTarget, "road": FollowerRoadTarget, "magnet": FollowerMagnetHandler, "hero": FollowerHeroHandler}[fixture.Route]
			if result.Kind != kind || [2]uint16{uint16(result.TargetX), uint16(result.TargetY)} != fixture.Target || result.Fallthrough != (fixture.Actor.State == 4) {
				t.Fatalf("decision %+v differs from native route %s target%v", result, fixture.Route, fixture.Target)
			}
			got := nativeDecisionActor{Kind: actor.Kind, Owner: actor.Player + 1, Flags: actor.Flags, X: uint16(actor.X), Y: uint16(actor.Y), VX: actor.VX, VY: actor.VY, Speed: actor.Speed, Timer: actor.Timer, State: actor.State, ReturnState: actor.ReturnState, SearchIndex: input.SearchIndex, Weapon: 1, Population: uint32(actor.Population), Variant: actor.Variant}
			if got != fixture.Actor {
				t.Fatalf("Go actor %+v; native %+v", got, fixture.Actor)
			}
			if seed != fixture.RNG {
				t.Fatalf("Go RNG %08x; native %08x", seed, fixture.RNG)
			}
		})
	}
}

func TestFollowerDecisionPrepassAndMalformedChains(t *testing.T) {
	rules, err := DecodeFollowerDecisionRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	var cells NativeOccupancyState
	actor := FollowerMotionActor{Kind: 2, Player: 0, X: 8320, Y: 8320, Speed: 20, State: 2}
	before := actor
	result, err := rules.Select(&actor, 2, NativeFollowerSettle, &cells, FollowerDecisionCallbacks{Prepass: func(*FollowerMotionActor) bool { return false }, Random: func() int { t.Fatal("rejected prepass reached RNG"); return 0 }})
	if err != nil || result.Kind != FollowerNoTarget || actor != before {
		t.Fatal("prepass did not stop selection")
	}
	cells.Cells[31+31*64].Head = 104
	_, err = rules.Select(&actor, 2, NativeFollowerSettle, &cells, FollowerDecisionCallbacks{Record: func(reference NativeRecordReference) (FollowerDecisionRecord, bool) {
		return FollowerDecisionRecord{Kind: 2, Owner: 1, Next: reference}, true
	}})
	if err == nil {
		t.Fatal("cyclic native metadata chain accepted")
	}
	for _, index := range []uint8{1, 37, 255} {
		if _, err := rules.Select(&actor, index, NativeFollowerSettle, &cells, FollowerDecisionCallbacks{}); err == nil {
			t.Fatal(fmt.Sprintf("invalid search index%d accepted", index))
		}
	}
}
