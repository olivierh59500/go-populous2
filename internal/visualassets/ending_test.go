package visualassets

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"
)

func TestEndingAssetsRemainIndexedAndRejectPaletteChanges(t *testing.T) {
	files := fstest.MapFS{}
	palette := make(color.Palette, 16)
	for i := range palette {
		palette[i] = color.RGBA{R: uint8(i * 17), A: 255}
	}
	add := func(name string, palette color.Palette) {
		img := image.NewPaletted(image.Rect(0, 0, 320, 200), palette)
		img.Pix[27] = 5
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			t.Fatal(err)
		}
		files[name] = &fstest.MapFile{Data: data.Bytes()}
	}
	add("frame-a.png", palette)
	add("frame-b.png", palette)
	loader := &imageLoader{files: files, images: map[string]image.Image{}}
	desc := &EndingDescriptor{Frames: []string{"frame-a.png", "frame-b.png"}, LoopStart: 1, Text: "THE END", IntroWait: 1, FrameWait: 4, TextStepFrames: 2}
	sequence, err := loader.ending(desc)
	if err != nil {
		t.Fatal(err)
	}
	if sequence.Frames[0].Pix[27] != 5 || sequence.Palette[5].R != 85 {
		t.Fatal("indexed ending pixel or palette changed")
	}
	bad := append(color.Palette(nil), palette...)
	bad[0] = color.RGBA{G: 255, A: 255}
	add("different.png", bad)
	desc.Frames[1] = "different.png"
	if _, err := loader.ending(desc); err == nil {
		t.Fatal("ending palette drift accepted")
	}
	desc.Frames[1] = "../frame.png"
	if _, err := loader.ending(desc); err == nil {
		t.Fatal("ending path traversal accepted")
	}
	desc.Frames[1] = "frame-b.png"
	desc.LoopStart = 2
	if _, err := loader.ending(desc); err == nil {
		t.Fatal("ending loop outside frames accepted")
	}
}
