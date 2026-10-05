package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeTileBitmapsAgainstOriginalPreparationAndDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/native_tile_bitmap_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Land, Tile, Pair, DestinationOffset int
			D                                   [8]uint32
			ReturnedOffset                      int
			Hash                                string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 3060 {
		t.Fatal("original tile pair coverage incomplete")
	}
	var banks [4]*NativeTileBitmapBank
	for land := range banks {
		banks[land], err = DecodeNativeTileBitmapBank(testBundle(t).Raw[fmt.Sprintf("block%d.pak", land)])
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range corpus.Cases {
		bitmap := make([]byte, 32000)
		for i := range bitmap {
			bitmap[i] = byte(i*7 + 13 + f.Land*19)
		}
		bank := banks[f.Land]
		first, second := bank.Descriptors[f.Tile][f.Pair*2], bank.Descriptors[f.Tile][f.Pair*2+1]
		if err := bank.PaintChunk(NativeTileChunkRequest{SourceOffset: first, DestinationOffset: f.DestinationOffset}, bitmap); err != nil {
			t.Fatal(f.Land, f.Tile, f.Pair, err)
		}
		if err := bank.PaintChunk(NativeTileChunkRequest{SourceOffset: second, DestinationOffset: f.DestinationOffset + 2}, bitmap); err != nil {
			t.Fatal(f.Land, f.Tile, f.Pair, err)
		}
		if f.ReturnedOffset != f.DestinationOffset+320 || uint16(f.D[0]) != second {
			t.Fatal("original BFAC pair pointer/word continuation differs", f.Land, f.Tile, f.Pair)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.Hash {
			t.Fatalf("LAND%d tile%d pair%d native bitmap differs: %s/%s", f.Land, f.Tile, f.Pair, got, f.Hash)
		}
	}
}
