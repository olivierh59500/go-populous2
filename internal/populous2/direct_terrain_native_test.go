package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

type nativeDirectAltRun struct {
	Count int
	Value uint8
}

type nativeDirectTerrainCell struct {
	Index        int
	Header, Tile uint8
	Head         uint16
}

type nativeDirectTerrainStep struct {
	Operation struct {
		X, Y  int
		Raise bool
	}
	AltSHA256       string
	ChangedVertices int
	BasaltCells     []nativeDirectTerrainCell
	Mana            [2]int
	RNG             uint32
}

type nativeDirectTerrainCase struct {
	Name             string
	InitialAltRuns   []nativeDirectAltRun
	InitialAltSHA256 string
	Overrides        []nativeDirectTerrainCell
	War              bool
	Steps            []nativeDirectTerrainStep
}

func nativeDirectTerrainFixtures(t *testing.T) []nativeDirectTerrainCase {
	t.Helper()
	data, err := os.ReadFile("testdata/native_direct_terrain.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeDirectTerrainCase }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 66 {
		t.Fatal("native direct-terrain fixture catalog is incomplete")
	}
	return catalog.Cases
}

func nativeDirectHeightHash(alt [legacy.EndWidth * legacy.EndWidth]int) string {
	var bytes [legacy.EndWidth * legacy.EndWidth]byte
	for index, height := range alt {
		bytes[index] = byte(height)
	}
	return fmt.Sprintf("%x", sha256.Sum256(bytes[:]))
}

// These fixtures execute the original direct terrain operations, including
// recursive lowering, 65-by-65 boundary vertices, and persistent basalt codes.
// They do not use the player admission/planning routine at $d464/$d47c.
func TestDirectTerrainAgainstNativeHeightGridsAndBasaltShapes(t *testing.T) {
	operations := 0
	for _, fixture := range nativeDirectTerrainFixtures(t) {
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
				t.Fatal("native initial4225height grid differs")
			}
			core := legacy.WorldFromSnapshot(legacy.WorldSnapshot{Alt: alt, War: fixture.War, RNG: 4311}, legacy.DefaultTerrainRules())
			core.Magnets[0].Mana, core.Magnets[1].Mana = 1000, 2000
			// The hook is defined by the owner of the shared legacy engine.
			// Keep this proof in a separate file from its implementation.
			lowerer, ok := any(core).(interface{ DirectLowerTerrain(int, int) bool })
			if !ok {
				t.Fatal("direct terrain lowering hook is missing")
			}
			world := &World{Core: core}
			for _, cell := range fixture.Overrides {
				if cell.Tile >= 0xe0 && cell.Tile <= 0xef {
					world.Marks[cell.Index] = Mark{Spell: Basalt, Life: 1, Persistent: true, NativeTile: cell.Tile}
				}
			}
			for index, step := range fixture.Steps {
				operations++
				before := core.Alt
				changed := false
				if step.Operation.Raise {
					changed = core.PaintRaiseAt(step.Operation.X, step.Operation.Y)
				} else {
					changed = lowerer.DirectLowerTerrain(step.Operation.X, step.Operation.Y)
				}
				world.clearChangedGround(before)
				if changed != (step.ChangedVertices != 0) || nativeDirectHeightHash(core.Alt) != step.AltSHA256 {
					t.Fatalf("operation%d native4225height grid or return differs", index)
				}
				if core.Magnets[0].Mana != step.Mana[0] || core.Magnets[1].Mana != step.Mana[1] || core.Snapshot().RNG != step.RNG {
					t.Fatal("direct terrain operation changed native mana or RNG")
				}
				if core.War != fixture.War {
					t.Fatal("direct lowering changed the world battle flag")
				}
				for _, cell := range step.BasaltCells {
					if got := world.TerrainCell(cell.Index%64, cell.Index/64).Code; got != cell.Tile {
						t.Fatalf("operation%d basaltcode%x differs fromnative%x", index, got, cell.Tile)
					}
				}
			}
		})
	}
	if operations != 77 {
		t.Fatalf("checked%d native operations, want77", operations)
	}
}
