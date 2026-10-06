package main

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestAtlasPackingPreservesPixelsAndBlankSlots(t *testing.T) {
	first := image.NewRGBA(image.Rect(3, 4, 35, 6))
	first.SetRGBA(3, 4, color.RGBA{R: 255, A: 255})
	last := image.NewRGBA(image.Rect(0, 0, 32, 20))
	last.SetRGBA(31, 19, color.RGBA{B: 255, A: 255})
	images := make([]*image.RGBA, 18)
	for i := range images {
		images[i] = first
	}
	images[1] = nil
	images[17] = last
	anchors := make([]image.Point, len(images))
	anchors[17] = image.Pt(16, 20)
	atlas, regions, err := pack("atlas.png", images, anchors)
	if err != nil {
		t.Fatal(err)
	}
	if regions[1].Atlas != "" || regions[1].Width != 0 {
		t.Fatal("blank sprite slot acquired artwork")
	}
	region := regions[17]
	if region.Y != 2 || region.AnchorX != 16 || region.AnchorY != 20 {
		t.Fatalf("row packing or anchors changed: %+v", region)
	}
	if atlas.RGBAAt(region.X+31, region.Y+19).B != 255 || atlas.RGBAAt(0, 0).R != 255 {
		t.Fatal("atlas lost pixels or mishandled nonzero image origin")
	}
}

// Original data are optional and excluded from the source repository.
func TestPrivateArtworkExportRoundTrip(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set POPULOUS2_EXPORT_TEST_DIR for private original-art comparison")
	}
	files := os.DirFS(input)
	source, err := populous2.LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := export(files, output); err != nil {
		t.Fatal(err)
	}
	portable, err := visualassets.LoadFS(os.DirFS(output))
	if err != nil {
		t.Fatal(err)
	}
	for bank := range source.Tiles {
		for index, want := range source.Tiles[bank] {
			got := portable.Tiles[bank][index]
			if got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("tile %d/%d changed", bank, index)
			}
		}
		for index, want := range source.Sprites[bank] {
			got := portable.Sprites[bank][index]
			if want.Image == nil {
				if got.Image != nil {
					t.Fatalf("empty sprite %d/%d changed", bank, index)
				}
				continue
			}
			if got.AnchorX != want.AnchorX || got.AnchorY != want.AnchorY || got.Image.Bounds() != want.Image.Bounds() || !bytes.Equal(got.Image.Pix, want.Image.Pix) {
				t.Fatalf("sprite %d/%d changed", bank, index)
			}
		}
	}
	font, err := populous2.DecodeNativeMenuFont(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for index, glyph := range font.Glyphs {
		if !bytes.Equal(portable.Font.Glyphs[index], glyph[:]) {
			t.Fatalf("font glyph %d changed", index)
		}
	}
	for part, variants := range source.DeityArt.Parts {
		for variant, want := range variants {
			if got := portable.PortraitParts[part][variant]; got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("portrait strip %d/%d changed", part, variant)
			}
		}
	}
	for _, path := range []string{"populous.ii", "POPULOUS.II", "code.bin"} {
		if _, err := os.Stat(filepath.Join(output, path)); !os.IsNotExist(err) {
			t.Fatalf("unexpected game program in portable installation: %s", path)
		}
	}
}
