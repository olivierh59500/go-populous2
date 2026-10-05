package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeTileWindowAgainstOriginalAdjacentDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/native_bitmap_window_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Land, Tile, Pair, DestinationOffset int
			Hash                                string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 480 {
		t.Fatal("native adjacent tile window coverage incomplete")
	}
	var banks [4]*NativeTileBitmapBank
	for land := range banks {
		banks[land], err = DecodeNativeTileBitmapBank(testBundle(t).Raw[fmt.Sprintf("block%d.pak", land)])
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range corpus.Cases {
		ram := make([]byte, 32640)
		for i := range ram {
			ram[i] = byte(i*19 + f.Land*17 + f.Tile)
		}
		window := NativeBitmapWindow{Bytes: ram, BitmapOffset: 128}
		bank := banks[f.Land]
		for half := 0; half < 2; half++ {
			request := NativeTileChunkRequest{SourceOffset: bank.Descriptors[f.Tile][f.Pair*2+half], DestinationOffset: f.DestinationOffset + half*2}
			if err := bank.PaintChunkWindow(request, window); err != nil {
				t.Fatal(f, err)
			}
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(ram)); got != f.Hash {
			t.Fatalf("native adjacent DMA differs %+v: %s/%s", f, got, f.Hash)
		}
	}
}

func TestNativeTileWindowReportsUnavailableBackingAfterPrefix(t *testing.T) {
	bank, err := DecodeNativeTileBitmapBank(testBundle(t).Raw["block0.pak"])
	if err != nil {
		t.Fatal(err)
	}
	ram := make([]byte, 32000)
	for i := range ram {
		ram[i] = 0x5a
	}
	before := append([]byte(nil), ram...)
	request := NativeTileChunkRequest{SourceOffset: 3140, DestinationOffset: 0x1e90}
	err = bank.PaintChunkWindow(request, NativeBitmapWindow{Bytes: ram})
	if err == nil {
		t.Fatal("absent native adjacent RAM was silently clamped")
	}
	changed := false
	for i, v := range ram {
		if v != before[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("native DMA prefix did not retain preceding writes")
	}
}
