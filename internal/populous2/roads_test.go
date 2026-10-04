package populous2

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestRoadJoinsAgainstOriginal68000Routine(t *testing.T) {
	w := flatGroundWorld(t)
	for _, point := range [][2]int{{32, 32}, {33, 32}, {34, 32}, {34, 33}, {33, 33}} {
		if !w.Cast(0, Road, Target{X: point[0], Y: point[1]}) {
			t.Fatal("road paint rejected")
		}
	}
	var tiles [4096]byte
	for i := range tiles {
		tiles[i] = w.TerrainCell(i%64, i/64).Code
	}
	// All 4096 tile codes from five sequential native $1677a calls.
	if got := fmt.Sprintf("%x", sha256.Sum256(tiles[:])); got != "036d45628688268226ee231c53abcad3b5dbee6f240fa213eaca6cbc6b872d3d" {
		t.Fatalf("native road joins differ: %s", got)
	}
	before := w.Core.Magnets[0].Mana
	if !w.RemoveRoad(33, 32) || w.TerrainCell(33, 32).Code != 15 || w.Core.Magnets[0].Mana != before {
		t.Fatal("road removal failed or charged mana")
	}
}
