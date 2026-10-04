package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldTownEvaluatorAgainstNativeMaps(t *testing.T) {
	data, err := os.ReadFile("testdata/town_evaluator_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeTownFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 530 {
		t.Fatalf("invalid native town catalog: %v", err)
	}
	checked := 0
	for _, fixture := range catalog.Cases {
		if len(fixture.Input.Records) != 0 || !inside(fixture.Input.X, fixture.Input.Y) {
			continue
		}
		checked++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			w := lightningWorld(t, 4311)
			original := nativeTownInitial(input, w.TownEvaluator)
			for pos := range w.Occupancy.Grid.Cells {
				at := pos * 4
				w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: original.grid[at], Tile: original.grid[at+1], Head: NativeRecordReference(original.grid[at+2])<<8 | NativeRecordReference(original.grid[at+3])}
				w.NativeOverlays[pos] = original.overlays[pos]
				if original.grid[at+1] != 15 {
					w.Marks[pos] = Mark{Spell: Flowers, Life: 1, Persistent: true, NativeTile: original.grid[at+1], NativeCodeValid: true}
				}
			}
			pos := input.X + input.Y*64
			w.Core.Peeps = []legacy.Peep{{Player: input.Owner - 1, Population: 934, AtPos: pos, Flags: legacy.InTown, TownStage: int(input.Stage), TownWork: 23}}
			ref := nativeActorReference(NativeFollowerPool, 0)
			w.Occupancy.Followers[0] = NativeWorldOccupancyRecord{Linked: true, Record: NativeOccupancyRecord{X: uint16(input.X*256 + 128), Y: uint16(input.Y*256 + 128)}}
			w.initializeEntryRecord(0)
			w.NativeEntries[0].Actor.Motion.Animation = 0x744
			w.Core.GameTurn = int(input.Clock)
			stage := 0
			if input.Mode == "clear" {
				if err := w.clearNativeFarms(ref, input.Replacement); err != nil {
					t.Fatal(err)
				}
			} else {
				var err error
				stage, err = w.evaluateNativeTown(ref)
				if err != nil {
					t.Fatal(err)
				}
			}
			if input.Mode != "clear" && stage != int(fixture.ResultStage) {
				t.Fatalf("native town stage: got %d, want %d", stage, fixture.ResultStage)
			}
			if worldNativeGridHash(w) != fixture.GridSHA256 || fmt.Sprintf("%x", sha256.Sum256(w.NativeOverlays[:])) != fixture.OverlaySHA256 || w.Core.Peeps[0].TownWork != 23 {
				t.Fatal("world native map/overlays/work differ")
			}
		})
	}
	if checked != 523 {
		t.Fatalf("native world town coverage %d; expected 523", checked)
	}
}
