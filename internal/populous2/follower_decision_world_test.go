package populous2

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldOrdinaryDecisionsAgainstNativeTargets(t *testing.T) {
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
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Fixtures) != 51 {
		t.Fatalf("invalid native decision catalog: %v", err)
	}
	checked := 0
	for _, fixture := range catalog.Fixtures {
		if fixture.Input.Flags&2 != 0 || fixture.Input.Mode == 16 {
			continue // Separate native hero/magnet handlers are not this selector.
		}
		checked++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			w := oceanWorld(t, input.Seed)
			for pos := range w.Occupancy.Grid.Cells {
				w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: input.DefaultHeader, Tile: input.DefaultTile}
			}
			for _, cell := range input.Cells {
				w.Occupancy.Grid.Cells[cell.X+cell.Y*64] = NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile}
			}
			w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: 32 + 32*64, Flags: legacy.OnMove, MovementSpeed: input.Speed, IQ: int(input.SearchIndex), Weapons: 1}}
			for _, node := range input.Actors {
				ref := NativeRecordReference(node.Reference)
				location, ok := LocateNativeRecord(ref)
				if !ok {
					t.Fatal("native fixture references unknown pool")
				}
				record := NativeOccupancyRecord{Next: NativeRecordReference(node.Next), X: uint16(node.X * 256), Y: uint16(node.Y * 256)}
				switch location.Pool {
				case NativeSceneryPool:
					w.Scenery[location.Index] = SceneryActor{Kind: SceneryKind(node.Kind), Active: node.Owner != 0, X: node.X, Y: node.Y}
					w.Occupancy.Scenery[location.Index].Record = record
				case NativeFollowerPool:
					index := location.Index - 1
					for len(w.Core.Peeps) <= index {
						w.Core.Peeps = append(w.Core.Peeps, legacy.Peep{})
					}
					flags := byte(legacy.OnMove)
					if node.Kind == 4 {
						flags = legacy.InTown
					}
					w.Core.Peeps[index] = legacy.Peep{Player: node.Owner - 1, Population: 1000, AtPos: node.X + node.Y*64, Flags: flags}
					w.Occupancy.Followers[index].Record = record
				default:
					t.Fatal("unexpected pool in native decision fixture")
				}
				cell := &w.Occupancy.Grid.Cells[node.X+node.Y*64]
				if cell.Head == 0 {
					cell.Head = ref
				}
			}
			w.Occupancy.Grid.Cells[32+32*64].Head = 52
			w.Core.Magnets[0].Flags = legacy.SettleMode
			if input.Mode == 18 {
				w.Core.Magnets[0].Flags = legacy.JoinMode
			}
			if input.Mode == 20 {
				w.Core.Magnets[0].Flags = legacy.FightMode
			}
			w.Level.Players[0].Parameters[5] = 0
			w.Core.FollowerAttrition = func(int, bool) int { return 0 }
			actor := FollowerMotionActor{Kind: 2, Player: 0, Flags: input.Flags, X: 8320, Y: 8320, VX: input.VX, VY: input.VY, Speed: input.Speed, State: 2, ReturnState: 2, Population: 1000}
			fallsThrough := w.planNativeWalker(0, &actor)
			want := fixture.Actor
			if actor.X != int16(want.X) || actor.Y != int16(want.Y) || actor.VX != want.VX || actor.VY != want.VY || actor.Speed != want.Speed || actor.Timer != want.Timer || actor.State != want.State || actor.ReturnState != want.ReturnState || fallsThrough != (want.State == 4) || w.Core.Snapshot().RNG != fixture.RNG {
				t.Fatalf("World native target/leg/RNG differs: got%+v; native%+v", actor, want)
			}
		})
	}
	if checked != 49 {
		t.Fatalf("native world decision coverage %d; expected49", checked)
	}
}

func TestNativeRoadDecisionSavedLegRetainsVelocity(t *testing.T) {
	for _, speed := range []uint8{20, 240, 255} {
		w := oceanWorld(t, 1)
		pos := 32 + 32*64
		w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: speed, IQ: 2}}
		w.initializeNativeFollower(0)
		w.Core.FollowerAttrition = func(int, bool) int { return 0 }
		w.Level.Players[0].Parameters[5] = 0
		w.Core.Magnets[0].Flags = legacy.SettleMode
		w.Marks[pos+1] = Mark{Spell: Road, Life: 1, Persistent: true, NativeTile: 197}
		w.Occupancy.Grid.Cells[pos+1].Tile = 197
		actor := &w.NativeFollowers[0].Actor
		actor.VX = int16(speed)
		if !w.planNativeWalker(0, actor) {
			t.Fatal("native forward road leg rejected")
		}
		if actor.VX != int16(min(255, int(speed)+20)) {
			t.Fatal("road lost its native accelerated velocity")
		}
		if speed >= 240 && actor.Speed != 235 {
			t.Fatal("saturated native road speed restore was normalized")
		}
		restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
		if err != nil {
			t.Fatal(err)
		}
		restored.Level.Players[0].Parameters[5] = 0
		for range 5 {
			w.updateNativeFollower(0)
			restored.updateNativeFollower(0)
		}
		if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
			t.Fatal("loading changed native accelerated road leg or RNG")
		}
	}
}
