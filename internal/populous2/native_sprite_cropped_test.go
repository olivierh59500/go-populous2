package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeCroppedSpritePixelsAgainstOriginalDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/native_sprite_cropped_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name    string
				Sprite  int
				Visible uint16
				X, Y    int16
			}
			SourceHeight uint16
			Routine      uint32
			D            [8]uint32
			Hash, Error  string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 1260 {
		t.Fatalf("native cropped DMA corpus incomplete: %v", err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(testBundle(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	routines := map[uint32]int{}
	for _, f := range catalog.Cases {
		routines[f.Routine]++
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original crop DMA failed", f.Error)
			}
			bitmap := make([]byte, 32000)
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
			}
			r := NativeCroppedSpriteRequest{Sprite: NativePresentationSprite{Sprite: f.Input.Sprite, X: f.Input.X, Y: f.Input.Y, Height: int16(f.Input.Visible), Routine: f.Routine}, SourceHeight: int16(f.SourceHeight)}
			if err := bank.PaintCropped(r, bitmap); err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.Hash {
				t.Errorf("original cropped sprite pixels differ: got%s want%s", got, f.Hash)
			}
		})
	}
	if routines[0xf0e8] != 630 || routines[0xf398] != 630 {
		t.Fatal("native cropped width coverage incomplete")
	}
}

func TestNativeCroppedSpriteRejectsUnprovedDisplacement(t *testing.T) {
	bank, err := DecodeNativeSpriteBitmapBank(testBundle(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	r := NativeCroppedSpriteRequest{Sprite: NativePresentationSprite{Sprite: 79, Height: 4, Routine: 0xf0e8}, SourceHeight: 8, SourceRow: 1}
	bitmap := make([]byte, 32000)
	if err := bank.PaintCropped(r, bitmap); err == nil {
		t.Fatal("unproved native source-row offset silently accepted")
	}
	for _, v := range bitmap {
		if v != 0 {
			t.Fatal("rejected crop mutated bitmap")
		}
	}
}
