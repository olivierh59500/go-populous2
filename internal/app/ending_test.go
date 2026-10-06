package app

import (
	"image"
	"image/color"
	"os"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestEndingClockAndLoopDoNotDependOnDrawing(t *testing.T) {
	palette := make(color.Palette, 16)
	for i := range palette {
		palette[i] = color.RGBA{R: uint8(i * 17), A: 255}
	}
	frames := []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 320, 200), palette), image.NewPaletted(image.Rect(0, 0, 320, 200), palette), image.NewPaletted(image.Rect(0, 0, 320, 200), palette)}
	p, err := NewEndingPlayback(&visualassets.EndingSequence{Frames: frames, LoopStart: 1, Text: "ABC", IntroWait: 1, FrameWait: 4, TextStepFrames: 2})
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		p.Update()
	}
	if p.Frame != 0 || p.TextOffset != 0 {
		t.Fatal("ending advanced before its initial wait")
	}
	p.Update()
	if p.Frame != 1 || p.TextOffset != 0 {
		t.Fatal("first ending frame or text phase differs")
	}
	for range 4 {
		p.Update()
	}
	if p.Frame != 2 || p.TextOffset != 1 {
		t.Fatal("ending did not advance one character per two frame updates")
	}
	for range 4 {
		p.Update()
	}
	if p.Frame != 1 || p.TextOffset != 1 {
		t.Fatal("ending did not return to its semantic loop start")
	}
	before := *p
	font := &visualassets.Font{FirstCode: 'A', Width: 8, Height: 8, Glyphs: [][]uint8{make([]uint8, 64), make([]uint8, 64), make([]uint8, 64)}}
	dst := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for range 50 {
		p.Draw(dst, font)
	}
	if p.Frame != before.Frame || p.TextOffset != before.TextOffset || p.updates != before.updates {
		t.Fatal("Draw advanced ending playback")
	}
}

func TestPrivateEndingArtworkEnglishTextAndPALClock(t *testing.T) {
	path := os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if path == "" {
		t.Skip("set POPULOUS2_ENDING_TEST_DIR for original ending playback comparison")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	originalPath := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if originalPath == "" {
		t.Skip("set POPULOUS2_EXPORT_TEST_DIR for private original ending data")
	}
	source, err := populous2.LoadFS(os.DirFS(originalPath))
	if err != nil {
		t.Fatal(err)
	}
	presentation, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	// Presentation language is deliberately independent of the source disk.
	// The source renderer still validates the translated glyphs and PAL clock.
	presentation.EndingText = []byte(visualassets.EnglishEndingText)
	reference, err := populous2.NewNativeEndingPlayback(source.Raw["end.pak"], presentation, 0)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := NewEndingPlayback(assets.Visual.EndingSequence)
	if err != nil {
		t.Fatal(err)
	}
	dst := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for blank := 0; blank < 150; blank++ {
		actual.Draw(dst, assets.Visual.Font)
		want, err := reference.Ending.Image()
		if err != nil {
			t.Fatal(err)
		}
		for y := range 200 {
			for x := range 320 {
				if dst.RGBAAt(x, y) != (color.RGBAModel.Convert(want.At(x, y)).(color.RGBA)) {
					t.Fatalf("ending blank%d pixel%d,%d differs", blank, x, y)
				}
			}
		}
		actual.Update()
		if _, err := reference.VBlank(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEndingRejectsIncompleteAssetSequence(t *testing.T) {
	if _, err := NewEndingPlayback(nil); err == nil {
		t.Fatal("nil ending accepted")
	}
	if _, err := NewEndingPlayback(&visualassets.EndingSequence{}); err == nil {
		t.Fatal("empty ending accepted")
	}
}
