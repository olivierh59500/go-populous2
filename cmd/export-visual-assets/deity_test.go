package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateDeityWidgetsAndRequesterMatchOriginalPixels(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("provide private original data")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	ui, err := interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	p, err := populous2.DecodeNativePresentation(ui.Executable)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := populous2.NativeWorldPalette(ui)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := exportDeityWidgets(dir, source); err != nil {
		t.Fatal(err)
	}
	widgets, err := visualassets.LoadDeityWidgets(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	layout, err := visualassets.LoadRequesterLayout(os.DirFS(dir), "deity-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	font, err := portableFont(p.Font, palette)
	if err != nil {
		t.Fatal(err)
	}
	for _, xp := range [][6]uint8{{}, {1, 17, 34, 127, 254, 255}} {
		name, password, bolts := "PLAYER", "ABCDEFGHIJKLMNOP", strings.TrimSuffix(strings.Repeat("} ", 5), " ")
		r, err := p.Compile(populous2.NativeMenuDeity, [][]byte{[]byte(name), []byte(bolts), []byte(password)})
		if err != nil {
			t.Fatal(err)
		}
		want, err := p.Compose(make([]byte, 320*200), r, palette)
		if err != nil {
			t.Fatal(err)
		}
		pixels := append([]byte(nil), want.Pix...)
		if err := p.Widgets.PaintDeity(pixels, xp); err != nil {
			t.Fatal(err)
		}
		wantRGBA := image.NewRGBA(image.Rect(0, 0, 320, 200))
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				wantRGBA.SetRGBA(x, y, palette[pixels[x+y*320]])
			}
		}
		portraits, err := populous2.DecodeDeityArt(ui.Executable, source.Raw["faces.pak"], palette)
		if err != nil {
			t.Fatal(err)
		}
		layers, err := p.Widgets.FaceLayers([3]uint8{})
		if err != nil {
			t.Fatal(err)
		}
		for _, layer := range layers {
			img := portraits.Parts[layer.Part][layer.Variant]
			draw.Draw(wantRGBA, img.Bounds().Add(image.Pt(layer.X, layer.Y)), img, image.Point{}, draw.Over)
		}
		got := image.NewRGBA(wantRGBA.Bounds())
		draw.Draw(got, got.Bounds(), image.NewUniform(palette[0]), image.Point{}, draw.Src)
		layout.Draw(got, font, map[string]string{"name": name, "password": password, "bolts": bolts}, nil)
		widgets.Draw(got, xp, [3]uint8{}, [3][8]*image.RGBA{})
		if !bytes.Equal(got.Pix, wantRGBA.Pix) {
			for y := 0; y < 200; y++ {
				for x := 0; x < 320; x++ {
					if got.RGBAAt(x, y) != wantRGBA.RGBAAt(x, y) {
						t.Fatalf("deity original pixel differs at%d,%d got%v want%v", x, y, got.RGBAAt(x, y), wantRGBA.RGBAAt(x, y))
					}
				}
			}
		}
	}
}

func portableFont(source *populous2.NativeMenuFont, palette [16]color.RGBA) (*visualassets.Font, error) {
	// Read the original font's atlas pixels as indices in its declared palette.
	atlas := source.Atlas(palette)
	f := &visualassets.Font{FirstCode: populous2.NativeGlyphFirst, Width: 8, Height: 8, Glyphs: make([][]uint8, populous2.NativeGlyphCount)}
	for i := range f.Glyphs {
		f.Glyphs[i] = make([]uint8, 64)
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				c := color.RGBAModel.Convert(atlas.At((i%16)*8+x, (i/16)*8+y)).(color.RGBA)
				for index, p := range palette {
					if c == p {
						f.Glyphs[i][x+y*8] = uint8(index)
						break
					}
				}
			}
		}
	}
	return f, nil
}
