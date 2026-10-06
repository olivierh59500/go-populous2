package main

import (
	"bytes"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateInGameRequesterMatchesOriginalNormalModes(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("provide private original resources")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	ui, err := interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeInGameRequesterRules(ui.Executable)
	if err != nil {
		t.Fatal(err)
	}
	pal, err := populous2.NativeWorldPalette(ui)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := exportInGameLayout(source, dir); err != nil {
		t.Fatal(err)
	}
	l, err := visualassets.LoadRequesterLayout(os.DirFS(dir), "in-game-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	font, err := portableFont(rules.Presentation.Font, pal)
	for _, mode := range []uint16{2, 4} {
		r, err := rules.Plan(populous2.NativeInGameState{Profile: 1, GameMode: mode, ControlMode: 2, PaintFlag: 1})
		if err != nil {
			t.Fatal(err)
		}
		expected, err := rules.Presentation.Compose(make([]byte, 320*200), r, pal)
		if err != nil {
			t.Fatal(err)
		}
		got := image.NewRGBA(expected.Bounds())
		draw.Draw(got, got.Bounds(), image.NewUniform(pal[0]), image.Point{}, draw.Src)
		l.Draw(got, font, map[string]string{"side": "GOOD", "assist": "OFF", "opponent": "COMPUTER V HUMAN   "}, map[string]bool{"conquest": mode == 2, "custom": mode == 4})
		want := image.NewRGBA(expected.Bounds())
		draw.Draw(want, want.Bounds(), expected, image.Point{}, draw.Src)
		if !bytes.Equal(got.Pix, want.Pix) {
			for y := 0; y < 200; y++ {
				for x := 0; x < 320; x++ {
					if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
						t.Fatalf("in-game requester differs at%d,%d got%v want%v", x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
					}
				}
			}
		}
	}
}
