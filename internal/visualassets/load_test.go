package visualassets

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"
)

func fixture(t *testing.T) (fstest.MapFS, Catalog) {
	t.Helper()
	files := make(fstest.MapFS)
	addPNG := func(name string, img image.Image) {
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			t.Fatal(err)
		}
		files[name] = &fstest.MapFile{Data: data.Bytes()}
	}
	colorA := color.RGBA{R: 17, G: 34, B: 51, A: 255}
	colorB := color.RGBA{R: 221, G: 68, B: 85, A: 255}
	atlas := image.NewRGBA(image.Rect(0, 0, 32, 32))
	atlas.SetRGBA(8, 9, colorA)
	atlas.SetRGBA(9, 9, colorB)
	addPNG("atlas.png", atlas)
	addPNG("screen.png", image.NewRGBA(image.Rect(0, 0, 320, 200)))
	fontPalette := make(color.Palette, 16)
	for i := range fontPalette {
		fontPalette[i] = color.RGBA{R: uint8(i * 17), A: 255}
	}
	font := image.NewPaletted(image.Rect(0, 0, 8, 8), fontPalette)
	font.SetColorIndex(0, 0, 4)
	font.SetColorIndex(1, 0, 15)
	addPNG("font.png", font)
	catalog := Catalog{Version: SchemaVersion, Background: "screen.png", Startup: "screen.png", Font: FontDescriptor{Atlas: "font.png", FirstCode: 'A', Count: 1, GlyphWidth: 8, GlyphHeight: 8, Columns: 1}}
	for land := range catalog.Landscapes {
		catalog.Landscapes[land].Tiles = []Region{{Atlas: "atlas.png", X: 8, Y: 9, Width: 2, Height: 1}}
		catalog.Landscapes[land].Sprites = []Region{{}, {Atlas: "atlas.png", X: 8, Y: 9, Width: 2, Height: 1, AnchorX: 1, AnchorY: 1}}
		catalog.Landscapes[land].Palette[4] = colorA
		catalog.Landscapes[land].Palette[15] = colorB
		catalog.Landscapes[land].MapColors[6] = colorB
	}
	for part := range catalog.PortraitParts {
		for variant := range catalog.PortraitParts[part] {
			catalog.PortraitParts[part][variant] = Region{Atlas: "atlas.png", Width: 32, Height: 16}
		}
	}
	catalog.Animations = map[string]Animation{"follower/0/0/0": {Loop: true, Frames: []Frame{{Layers: []SpriteLayer{{Sprite: 1, X: -2, Y: 3}}, SoundCue: 2}}}}
	return files, catalog
}

func putCatalog(t *testing.T, files fstest.MapFS, catalog Catalog) {
	t.Helper()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	files[CatalogName] = &fstest.MapFile{Data: data}
}

func TestLoadWithoutAnyOriginalProgramOrResource(t *testing.T) {
	files, catalog := fixture(t)
	putCatalog(t, files, catalog)
	bundle, err := LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	if got := bundle.Tiles[2][0].RGBAAt(0, 0); got != (color.RGBA{R: 17, G: 34, B: 51, A: 255}) {
		t.Fatalf("atlas extraction changed pixel: %v", got)
	}
	if bundle.Sprites[0][0].Image != nil || bundle.Sprites[1][1].AnchorX != 1 || bundle.Sprites[1][1].AnchorY != 1 {
		t.Fatal("blank descriptor or artwork anchors changed")
	}
	if bundle.Tiles[0][0].Bounds() != image.Rect(0, 0, 2, 1) {
		t.Fatal("atlas crop not normalized")
	}
	bundle.Tiles[0][0].SetRGBA(0, 0, color.RGBA{})
	if bundle.Tiles[1][0].RGBAAt(0, 0).A != 255 {
		t.Fatal("landscape crops unexpectedly share mutable pixels")
	}
	if got := bundle.Animations["follower/0/0/0"].Frames[0]; len(got.Layers) != 1 || got.Layers[0].X != -2 || got.SoundCue != 2 {
		t.Fatal("semantic animation metadata changed")
	}
	if got := bundle.MapColors[3][6]; got != catalog.Landscapes[3].MapColors[6] {
		t.Fatal("minimap color lost")
	}
	portrait, err := bundle.Portrait([3]uint8{7, 3, 1})
	if err != nil || portrait.Bounds() != image.Rect(0, 0, 32, 96) {
		t.Fatalf("portrait composition: %v", err)
	}
	if _, err := bundle.Portrait([3]uint8{8, 0, 0}); err == nil {
		t.Fatal("invalid face variant accepted")
	}
}

func TestFontKeepsColorIndicesAndClipsNegativeOrigin(t *testing.T) {
	files, catalog := fixture(t)
	putCatalog(t, files, catalog)
	bundle, err := LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Font.Glyphs[0][0] != 4 || bundle.Font.Glyphs[0][1] != 15 {
		t.Fatal("PNG palette indices changed")
	}
	dest := image.NewRGBA(image.Rect(0, 0, 8, 8))
	bundle.Font.Draw(dest, "A", -1, 0, bundle.Palettes[0])
	if dest.RGBAAt(0, 0) != bundle.Palettes[0][15] {
		t.Fatal("font clipping or dynamic palette failed")
	}
	bundle.Palettes[0][4] = color.RGBA{B: 255, A: 255}
	bundle.Font.Draw(dest, "A", 0, 0, bundle.Palettes[0])
	if dest.RGBAAt(0, 0).B != 255 {
		t.Fatal("font was tied to an import-time palette")
	}
}

func TestRejectInvalidCatalogAndUnboundedImages(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Catalog)
	}{
		{"version", func(c *Catalog) { c.Version++ }},
		{"path traversal", func(c *Catalog) { c.Background = "../screen.png" }},
		{"missing art", func(c *Catalog) { c.Startup = "absent.png" }},
		{"outside crop", func(c *Catalog) { c.Landscapes[0].Tiles[0].X = 32 }},
		{"negative crop", func(c *Catalog) { c.Landscapes[0].Tiles[0].Width = -1 }},
		{"overflow crop", func(c *Catalog) { c.Landscapes[0].Tiles[0].X = int(^uint(0) >> 1) }},
		{"empty bank", func(c *Catalog) { c.Landscapes[1].Sprites = nil }},
		{"too many sprites", func(c *Catalog) { c.Landscapes[1].Sprites = make([]Region, maxSprites+1) }},
		{"font geometry", func(c *Catalog) { c.Font.GlyphWidth = 32 }},
		{"font code range", func(c *Catalog) { c.Font.FirstCode = 255; c.Font.Count = 2 }},
		{"invalid frame sprite", func(c *Catalog) {
			c.Animations["bad"] = Animation{Frames: []Frame{{Layers: []SpriteLayer{{Sprite: 1000}}}}}
		}},
		{"empty animation", func(c *Catalog) { c.Animations["bad"] = Animation{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files, catalog := fixture(t)
			test.mutate(&catalog)
			putCatalog(t, files, catalog)
			if _, err := LoadFS(files); err == nil {
				t.Fatal("invalid visual catalog accepted")
			}
		})
	}
	files, catalog := fixture(t)
	putCatalog(t, files, catalog)
	files[CatalogName].Data = append(files[CatalogName].Data, []byte(" {}")...)
	if _, err := LoadFS(files); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	putCatalog(t, files, catalog)
	var fields map[string]any
	if err := json.Unmarshal(files[CatalogName].Data, &fields); err != nil {
		t.Fatal(err)
	}
	fields["executable"] = "original.ii"
	files[CatalogName].Data, _ = json.Marshal(fields)
	if _, err := LoadFS(files); err == nil {
		t.Fatal("unknown executable field accepted")
	}
	if _, err := LoadFS(nil); err == nil {
		t.Fatal("nil filesystem accepted")
	}
}
