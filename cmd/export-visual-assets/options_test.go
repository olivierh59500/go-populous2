package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateOptionsPixelsAndAllHitRegionsMatchEnglishSource(t *testing.T) {
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
	art, err := decodeOptionsArt(source)
	if err != nil {
		t.Fatal(err)
	}
	p, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	font := &visualassets.Font{FirstCode: populous2.NativeGlyphFirst, Width: 8, Height: 8, Glyphs: make([][]uint8, populous2.NativeGlyphCount)}
	for i := range font.Glyphs {
		font.Glyphs[i] = append([]uint8(nil), p.Font.Glyphs[i][:]...)
	}
	actions := map[int]string{2: "side", 24: "reaction-decrease", 26: "reaction-increase", 28: "special-codes", 30: "proceed"}
	for i := 0; i < 10; i++ {
		actions[4+i*2] = fmt.Sprintf("rule-%d", i)
	}
	for side := 1; side <= 2; side++ {
		for _, flags := range []uint16{0, 0x155, 0x3ff} {
			for _, speed := range []uint16{0, 7, 15, 16} {
				t.Run(fmt.Sprintf("side%d-flags%x-speed%d", side, flags, speed), func(t *testing.T) {
					codes := []byte("MUSIC")
					if flags == 0 {
						codes = nil
					}
					original, err := p.Options(uint16(side), flags, speed, codes)
					if err != nil {
						t.Fatal(err)
					}
					want, err := p.Compose(make([]byte, 64000), original.Requester, art.Layout.Palette)
					if err != nil {
						t.Fatal(err)
					}
					wantRGBA := image.NewRGBA(want.Bounds())
					draw.Draw(wantRGBA, wantRGBA.Bounds(), want, image.Point{}, draw.Src)
					got := image.NewRGBA(image.Rect(0, 0, 320, 200))
					draw.Draw(got, got.Bounds(), image.NewUniform(art.Layout.Palette[0]), image.Point{}, draw.Src)
					marks := map[string]bool{}
					for i := 0; i < 10; i++ {
						marks[fmt.Sprintf("rule-%d", i)] = flags&(1<<i) != 0
					}
					art.Layout.Draw(got, font, map[string]string{"side": art.SideNames[side-1], "special-codes": string(codes)}, marks)
					art.DrawReaction(got, font, int(speed))
					if !bytes.Equal(got.Pix, wantRGBA.Pix) {
						reportFirstPixelDifference(t, got, wantRGBA)
					}
					if flags == 0 {
						for y := 0; y < 200; y++ {
							for x := 0; x < 320; x++ {
								clone := *original.Requester
								clone.Text = append([]byte(nil), original.Requester.Text...)
								want := actions[original.Rules.Click(&clone, x, y)]
								if got := art.ActionAt(x, y, int(speed)); got != want {
									t.Fatalf("options action at %d,%d =%s want%s", x, y, got, want)
								}
							}
						}
					}
				})
			}
		}
	}
	output := t.TempDir()
	if err := exportOptions(source, output); err != nil {
		t.Fatal(err)
	}
	if _, err := visualassets.LoadOptionsArt(os.DirFS(output)); err != nil {
		t.Fatal(err)
	}
}
