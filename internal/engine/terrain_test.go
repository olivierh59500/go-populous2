package engine

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// Reference digests came from the original game's four-hill generator. They
// describe heights only and contain neither original assets nor executable data.
func TestTerrainReferenceDigests(t *testing.T) {
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
		w := &World{}
		w.generate(reference.seed)
		if got := fmt.Sprintf("%x", sha256.Sum256(w.Heights[:])); got != reference.digest {
			t.Fatalf("seed %d: %s", reference.seed, got)
		}
	}
}

func TestTerrainEditsRetainRepresentableSlopesAndRefreshFourCells(t *testing.T) {
	w := &World{}
	w.Players[0].Mana = 10000
	for repeat := 0; repeat < 6; repeat++ {
		if !w.RaiseAt(0, 32, 32) {
			t.Fatal("raise rejected")
		}
	}
	checkSlopes := func() {
		t.Helper()
		for y := 0; y < CornerSize; y++ {
			for x := 0; x < CornerSize; x++ {
				for _, d := range directions {
					nx, ny := x+d[0], y+d[1]
					if insideCorner(nx, ny) && abs(int(w.Heights[x+y*CornerSize])-int(w.Heights[nx+ny*CornerSize])) > 1 {
						t.Fatal("unrepresentable slope")
					}
				}
			}
		}
	}
	checkSlopes()
	for _, tile := range [4][2]int{{31, 31}, {32, 31}, {32, 32}, {31, 32}} {
		if w.Cell(tile[0], tile[1]).IsWater() {
			t.Fatal("a neighbouring tile was not updated")
		}
	}
	for repeat := 0; repeat < 6; repeat++ {
		if !w.LowerAt(0, 32, 32) {
			t.Fatal("lower rejected")
		}
	}
	checkSlopes()
	mana := w.Players[0].Mana
	if w.LowerAt(0, 32, 32) || w.Players[0].Mana != mana {
		t.Fatal("unchanged terrain consumed mana")
	}
}

func TestTerrainAtlasBanksAndSlopedPicking(t *testing.T) {
	c := Cell{Corners: [4]uint8{2, 2, 2, 2}, BaseAltitude: 1, Shape: 15, Code: 15}
	if c.TileIndex(0, 0, 0) != 31 {
		t.Fatal("raised flat bank")
	}
	sea := Cell{}
	if sea.TileIndex(1, 0, 0) != 32 || sea.TileIndex(7, 0, 0) != 128 {
		t.Fatal("water animation used raised-land bank")
	}
	if !c.Contains(16, 8) || c.Contains(0, 23) {
		t.Fatal("surface picking ignored height")
	}
}

func TestTerrainEditsClearOnlyChangedGroundAndPressure(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].Mana = 1000
	w.Nature.Ground[31+31*MapSize] = GroundParcel{Mark: GroundFlowers, Owner: 0}
	w.Pressure[31+31*MapSize] = 64
	w.Nature.Ground[10+10*MapSize] = GroundParcel{Mark: GroundSwamp, Owner: 1}
	w.Pressure[10+10*MapSize] = 80
	if !w.RaiseAt(0, 32, 32) {
		t.Fatal("raise rejected")
	}
	if w.Nature.Ground[31+31*MapSize].Mark != GroundNone || w.Pressure[31+31*MapSize] != 0 {
		t.Fatal("changed ground retained its overlay or pressure")
	}
	if w.Nature.Ground[10+10*MapSize].Mark != GroundSwamp || w.Pressure[10+10*MapSize] != 80 {
		t.Fatal("unrelated ground was cleared")
	}
}
