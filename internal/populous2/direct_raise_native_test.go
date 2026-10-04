package populous2

import (
	"encoding/json"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

// The original unpriced $d81e/$cdca helper executes every operation, including
// recursive propagation and boundary vertices. These cases do not use the
// player sculpt planner, follower admission or inherited GameMode semantics.
func TestDirectRaiseAgainstNativeHeightGridsAndBasalt(t *testing.T) {
	data, err := os.ReadFile("testdata/native_direct_raise.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeDirectTerrainCase }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 666 {
		t.Fatal("native direct-raise catalog incomplete")
	}
	operations := 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			var alt [legacy.EndWidth * legacy.EndWidth]int
			at := 0
			for _, run := range fixture.InitialAltRuns {
				if run.Count < 1 || at+run.Count > len(alt) || run.Value > 8 {
					t.Fatal("invalid native height run")
				}
				for range run.Count {
					alt[at] = int(run.Value)
					at++
				}
			}
			if at != len(alt) || nativeDirectHeightHash(alt) != fixture.InitialAltSHA256 {
				t.Fatal("native initial height grid differs")
			}
			core := legacy.WorldFromSnapshot(legacy.WorldSnapshot{Alt: alt, War: fixture.War, RNG: 4311}, legacy.DefaultTerrainRules())
			core.Magnets[0].Mana, core.Magnets[1].Mana = 1000, 2000
			world := &World{Core: core}
			for _, cell := range fixture.Overrides {
				world.Marks[cell.Index] = Mark{Spell: Basalt, Life: 1, Persistent: true, NativeTile: cell.Tile}
			}
			for index, step := range fixture.Steps {
				operations++
				if !step.Operation.Raise {
					t.Fatal("non-raise operation in native raising catalog")
				}
				before := core.Alt
				changed := core.DirectRaiseTerrain(step.Operation.X, step.Operation.Y)
				world.clearChangedGround(before)
				if changed != (step.ChangedVertices != 0) || nativeDirectHeightHash(core.Alt) != step.AltSHA256 {
					t.Fatalf("operation%d native4225-height grid or return differs", index)
				}
				if core.Magnets[0].Mana != step.Mana[0] || core.Magnets[1].Mana != step.Mana[1] || core.Snapshot().RNG != step.RNG || core.War != fixture.War {
					t.Fatal("unpriced raise changed mana, RNG or War")
				}
				for _, cell := range step.BasaltCells {
					if got := world.TerrainCell(cell.Index%64, cell.Index/64).Code; got != cell.Tile {
						t.Fatalf("operation%d Basalt cell%d got%x want%x", index, cell.Index, got, cell.Tile)
					}
				}
			}
		})
	}
	if operations != 948 {
		t.Fatalf("checked%d native raising operations, want948", operations)
	}
}
