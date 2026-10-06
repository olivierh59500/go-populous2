package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateSpellHelpEnglishSheetAndAnimationPixelsMatchOriginal(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set the private imported artwork directory")
	}
	source, err := populous2.LoadFS(os.DirFS(input))
	if err != nil {
		t.Fatal(err)
	}
	source, err = interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := exportSpellHelp(source, output); err != nil {
		t.Fatal(err)
	}
	art, err := visualassets.LoadSpellHelpArt(os.DirFS(output))
	if err != nil {
		t.Fatal(err)
	}
	images := map[visualassets.Region]bool{}
	frames, pixels := 0, 0
	for _, icon := range art.Descriptor.Icons {
		images[icon] = true
	}
	for _, land := range art.Descriptor.Sequences {
		for _, sequence := range land {
			for _, frame := range sequence.Frames {
				frames++
				images[frame.Image] = true
				pixels += frame.Image.Width * frame.Image.Height
			}
		}
	}
	pngBytes, _ := os.ReadFile(output + "/spell-help.png")
	metadataBytes, _ := os.ReadFile(output + "/spell-help.json")
	t.Logf("%d shared graphic regions; %d frame references; %.2f MiB decoded frame copies; PNG %d bytes; metadata %d bytes", len(images), frames, float64(pixels*4)/(1<<20), len(pngBytes), len(metadataBytes))
	rules, err := populous2.DecodeNativeInGameRequesterRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	animationRules, err := populous2.DecodeNativeSpellHelpAnimationRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	p := rules.Presentation
	font := &visualassets.Font{FirstCode: populous2.NativeGlyphFirst, Width: 8, Height: 8, Glyphs: make([][]uint8, populous2.NativeGlyphCount)}
	for i := range font.Glyphs {
		font.Glyphs[i] = append([]uint8(nil), p.Font.Glyphs[i][:]...)
	}
	palette := art.Descriptor.Layout.Palette
	state := populous2.NativeWorldRequesterState{}
	for i := range state.PowerFlags {
		state.PowerFlags[i] = 1
	}
	for land := 0; land < 4; land++ {
		sprites, err := populous2.DecodeNativeSpriteBitmapBank(source, land)
		if err != nil {
			t.Fatal(err)
		}
		block := source.Raw[fmt.Sprintf("block%d.pak", land)]
		tiles, err := populous2.DecodeNativeTileBitmapBank(block)
		if err != nil {
			t.Fatal(err)
		}
		for slot := 0; slot < 36; slot++ {
			base, ok, err := rules.SpellHelpBase(source, state, uint16(slot*2), make([]byte, 64000))
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				continue
			}
			t.Run(fmt.Sprintf("land%d-power%d", land, slot), func(t *testing.T) {
				got := image.NewRGBA(image.Rect(0, 0, 320, 200))
				draw.Draw(got, got.Bounds(), image.NewUniform(palette[0]), image.Point{}, draw.Src)
				art.DrawBase(got, font, slot)
				if !bytes.Equal(got.Pix, base.Image.Pix) {
					reportFirstPixelDifference(t, got, base.Image)
				}
				state := animationRules.NewState()
				if err := animationRules.Begin(&state, uint16(slot*2)); err != nil {
					t.Fatal(err)
				}
				registers := [8]uint32{}
				for age := 0; age < 64; age++ {
					plan, err := animationRules.Advance(&state, block, &registers)
					if err != nil {
						t.Fatal(err)
					}
					if age != 0 && age != 1 && age != 2 && age != 31 && age != 63 {
						continue
					}
					bitmap := encodeHelpBitmap(base.Image, palette)
					for _, sprite := range plan.Sprites {
						if err := sprites.Paint(sprite, bitmap); err != nil {
							t.Fatal(err)
						}
					}
					for _, tile := range plan.Tiles {
						for chunk, offset := range tiles.Descriptors[tile.Tile] {
							destination := int(tile.ByteOffset) + (chunk%2)*2 + (chunk/2)*320
							if err := tiles.PaintChunk(populous2.NativeTileChunkRequest{SourceOffset: offset, DestinationOffset: destination}, bitmap); err != nil {
								t.Fatal(err)
							}
						}
					}
					want, err := populous2.DecodeScreen(bitmap, palette)
					if err != nil {
						t.Fatal(err)
					}
					draw.Draw(got, got.Bounds(), base.Image, image.Point{}, draw.Src)
					art.DrawFrame(got, land, slot, age)
					if !bytes.Equal(got.Pix, want.Pix) {
						t.Logf("animation age %d", age)
						reportFirstPixelDifference(t, got, want)
					}
				}
			})
		}
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			requester, err := p.Compile(populous2.NativeMenuSpellHelp, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if p.Requesters.Click(requester, x, y) == 2 {
				want = "return"
			}
			if got := art.Descriptor.Layout.ActionAt(x, y); got != want {
				t.Fatalf("help action at %d,%d=%s want%s", x, y, got, want)
			}
		}
	}
}

func encodeHelpBitmap(img *image.RGBA, palette [16]color.RGBA) []byte {
	lookup := map[color.RGBA]int{}
	for i, c := range palette {
		lookup[c] = i
	}
	bitmap := make([]byte, 32000)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			index := lookup[img.RGBAAt(x, y)]
			for plane := 0; plane < 4; plane++ {
				if index&(1<<plane) != 0 {
					bitmap[plane*8000+y*40+x/8] |= 1 << uint(7-x%8)
				}
			}
		}
	}
	return bitmap
}
