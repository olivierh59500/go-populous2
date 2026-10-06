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

func TestPrivateStartupLayoutMatchesOriginalPixelsAndHitRegions(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set private original data directory")
	}
	source, err := populous2.LoadFS(os.DirFS(input))
	if err != nil {
		t.Fatal(err)
	}
	presentation, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := export(os.DirFS(input), directory); err != nil {
		t.Fatal(err)
	}
	visual, err := visualassets.LoadFS(os.DirFS(directory))
	if err != nil {
		t.Fatal(err)
	}
	menu, err := visualassets.LoadStartupMenu(os.DirFS(directory))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := presentation.StartupImage()
	if err != nil {
		t.Fatal(err)
	}
	got := image.NewRGBA(expected.Bounds())
	draw.Draw(got, got.Bounds(), visual.Startup, image.Point{}, draw.Src)
	menu.Draw(got, visual.Font)
	want := image.NewRGBA(expected.Bounds())
	draw.Draw(want, want.Bounds(), expected, image.Point{}, draw.Src)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("pure Go startup composition differs from original glyph cells")
	}
	actions := map[int]visualassets.StartupAction{2: visualassets.StartupProfile, 4: visualassets.StartupConquest, 6: visualassets.StartupCustom, 8: visualassets.StartupLoad, 10: visualassets.StartupQuit}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			expected := actions[presentation.Requesters.Click(presentation.StartupRequester, x, y)]
			if got := menu.ActionAt(x, y); got != expected {
				t.Fatalf("original startup hit differs at%d,%d: %s/%s", x, y, got, expected)
			}
		}
	}
}
