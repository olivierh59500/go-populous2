package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"testing"
)

func TestNativeWorldIconsAgainstOriginalMaskedBlits(t *testing.T) {
	data, err := os.ReadFile("testdata/world_icons_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Slot, Background, X, Y int
			RGBAHash               string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 216 {
		t.Fatal("native world-icon framebuffer catalog incomplete")
	}
	r, err := DecodeNativeInGameRequesterRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	var palette [16]color.RGBA
	for i := range palette {
		palette[i] = color.RGBA{uint8(i * 17), uint8((15 - i) * 17), uint8((i * 7 & 15) * 17), 255}
	}
	for _, fixture := range catalog.Cases {
		img := image.NewRGBA(image.Rect(0, 0, 320, 200))
		for y := range 200 {
			for x := range 320 {
				index := (x*3 + y*5 + 7) & 15
				if fixture.Background == 0 {
					index = 0
				}
				img.SetRGBA(x, y, palette[index])
			}
		}
		icon, err := r.WorldIcon(fixture.Slot, palette)
		if err != nil {
			t.Fatal(err)
		}
		draw.Draw(img, icon.Bounds().Add(image.Pt(fixture.X, fixture.Y)), icon, image.Point{}, draw.Over)
		if got := fmt.Sprintf("%x", sha256.Sum256(img.Pix)); got != fixture.RGBAHash {
			t.Fatalf("slot%d background%d position%d,%d differs: %s / %s", fixture.Slot, fixture.Background, fixture.X, fixture.Y, got, fixture.RGBAHash)
		}
	}
}
