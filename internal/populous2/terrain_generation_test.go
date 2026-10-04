package populous2

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestNativeHillParametersAndHeight(t *testing.T) {
	b := testBundle(t)
	if b.HillParameters != [4][4]int{{29, 6, 29, 6}, {23, 35, 23, 35}, {29, 6, 23, 35}, {23, 35, 29, 6}} {
		t.Fatal("native four-hill parameter table changed")
	}
	a, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.Core.Alt != c.Core.Alt {
		t.Fatal("native random walks are not deterministic")
	}
	highest := 0
	for y := 0; y <= 64; y++ {
		for x := 0; x <= 64; x++ {
			h := a.Core.Alt[x+y*65]
			highest = max(highest, h)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if x+dx < 0 || x+dx > 64 || y+dy < 0 || y+dy > 64 {
						continue
					}
					if abs(h-a.Core.Alt[x+dx+(y+dy)*65]) > 1 {
						t.Fatal("native generated hills have an unrepresentable slope")
					}
				}
			}
		}
	}
	if highest < 1 || highest > 8 {
		t.Fatalf("native walk produced invalid peak: %d", highest)
	}
}

// These digests were obtained by executing the original CODE:$cd22 routine,
// with its real Hunk relocations and a separate zeroed Amiga memory image.
// They compare every vertex, including the eastern and southern borders.
func TestTerrainAgainstOriginal68000Routine(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range []struct {
		seed   uint32
		digest string
	}{
		{0, "8b938bc1ad57a8f7713f4cb392e9310325e88e684099cf86ef760964e84be687"},
		{1, "dbe42d10cc08fdc3f061aee8811ff1b244807e756e4e0747790f03de92528d3c"},
		{4311, "df55d057731012df348dffa568de7cd1f870fabe35ad7ff6de0fbf71538f3397"},
		{5038, "518b6c2c40cefe7fb856a3839e8002186b7dcd8609e24c1268c67454852c9b24"},
		{65535, "9bdef7c34ec845f5da3813b904d8195dbdaa45dbd220d09af07c564340eaa0f0"},
		{65536, "0e48291f9180b667d7051f3e91ff9f11a27f7d03e72c4b127bd4820f41ed2e4e"},
		{131073, "a4c91381894c95a220a6541307f2640609df5c1ca69aaf29702ed53e28be5627"},
		{262143, "3bb392447fa87177a9bad2bd377901a015cbe00b75d3b0c97d202104feb45bfc"},
	} {
		w.Core.GenerateOlympianTerrain(reference.seed, b.HillParameters)
		var heights [4225]byte
		for i, h := range w.Core.Alt {
			heights[i] = byte(h)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(heights[:])); got != reference.digest {
			t.Fatalf("seed %d differs from native terrain: %s", reference.seed, got)
		}
	}
}
