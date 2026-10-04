package populous2

import (
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestNativeTerrainGraphicsBanks(t *testing.T) {
	w := &World{Core: &legacy.World{}}
	for mask := 0; mask < 16; mask++ {
		for base := 0; base < 7; base++ {
			p := 20 + 20*65
			for i, offset := range [4]int{0, 1, 66, 65} {
				w.Core.Alt[p+offset] = base + (mask>>i)&1
			}
			cell := w.TerrainCell(20, 20)
			if mask > 0 && mask < 15 {
				want := mask
				if base > 0 {
					want += 16
				}
				if cell.Tile(0, 0, 0) != want {
					t.Fatalf("mask %d base %d: tile %d want %d", mask, base, cell.Tile(0, 0, 0), want)
				}
			}
		}
	}
	water := TerrainCell{}
	for phase, want := range []int{0, 32, 48, 64, 80, 96, 112, 128} {
		if got := water.Tile(phase, 0, 0); got != want {
			t.Fatalf("water phase %d uses land bank: %d", phase, got)
		}
	}
	if got := (TerrainCell{Code: 47, BaseAltitude: 3}).Tile(5, 2, 3); got != 47 {
		t.Fatal("blue farm tile lost")
	}
	if got := (TerrainCell{Code: 63, BaseAltitude: 3}).Tile(5, 2, 3); got != 63 {
		t.Fatal("red farm tile lost")
	}
}

func TestLowFlatLandIsNotWater(t *testing.T) {
	w := flatGroundWorld(t)
	cell := w.TerrainCell(32, 32)
	if cell.BaseAltitude != 0 || cell.IsWater() {
		t.Fatal("first-height land mistaken for water")
	}
	if w.Cast(0, Whirlpool, Target{X: 32, Y: 32}) || w.Cast(0, Tsunami, Target{X: 32, Y: 32}) {
		t.Fatal("water-only disaster accepted on low flat land")
	}
	if !w.Cast(0, Trees, Target{X: 32, Y: 32}) {
		t.Fatal("vegetation rejected on low flat land")
	}
	if w.TerrainCell(32, 32).Code != 15 {
		t.Fatal("native flat-land minimap code changed")
	}
	w.Marks[32+32*64] = Mark{Spell: Flowers, Player: 0, Life: 1, Persistent: true, NativeTile: 245}
	if w.TerrainCell(32, 32).Code != 245 {
		t.Fatal("minimap cell lost native ground effect code")
	}
}

func TestSculptKeepsSharedCornerGeometryContinuous(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Peeps = []legacy.Peep{{Player: 0, AtPos: 32 + 32*64, Population: 100, Flags: legacy.OnMove}}
	w.Core.MapWho = [4096]uint16{}
	w.Core.MapWho[32+32*64] = 1
	for _, raise := range []bool{true, true, true, true, false, false, true, false, false, false} {
		w.Core.Magnets[0].Mana = 100000
		if !w.Sculpt(0, 32, 32, raise) {
			t.Fatal("valid terrain edit rejected")
		}
		for y := 27; y < 37; y++ {
			for x := 27; x < 37; x++ {
				c, e, s := w.TerrainCell(x, y), w.TerrainCell(x+1, y), w.TerrainCell(x, y+1)
				if c.Corners[1] != e.Corners[0] || c.Corners[2] != e.Corners[3] || c.Corners[2] != s.Corners[1] || c.Corners[3] != s.Corners[0] {
					t.Fatalf("terrain seam after edit at %d,%d", x, y)
				}
				for _, height := range c.Corners {
					if height-c.BaseAltitude < 0 || height-c.BaseAltitude > 1 {
						t.Fatal("native tile cannot represent propagated slope")
					}
				}
				if c.Tile(0, x, y) >= len(testBundle(t).Tiles[0]) {
					t.Fatal("out-of-range terrain graphic")
				}
			}
		}
	}
	flat := TerrainCell{Corners: [4]int{1, 1, 1, 1}, Shape: 15, Code: 15}
	if !flat.Contains(16, 8) || flat.Contains(16, 25) {
		t.Fatal("picker ignores the raised tile surface")
	}
}
