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

func TestPrivateFileRequesterMatchesOriginalGlyphsAndActions(t *testing.T) {
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
	p, err := populous2.DecodeNativePresentation(ui.Executable)
	if err != nil {
		t.Fatal(err)
	}
	pal, err := populous2.NativeWorldPalette(ui)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := exportFileLayouts(source, dir); err != nil {
		t.Fatal(err)
	}
	l, err := visualassets.LoadRequesterLayout(os.DirFS(dir), "files-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	font, err := portableFont(p.Font, pal)
	if err != nil {
		t.Fatal(err)
	}
	params := make([][]byte, 16)
	params[0], params[15] = []byte("LOAD"), []byte("LOAD")
	values := map[string]string{"verb": "LOAD", "directory": "SAVES", "name": "TEST.GAM"}
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("TRACK%02d.GAM", i)
		params[i+1] = []byte(name)
		values[fmt.Sprintf("file-%d", i)] = name
	}
	params[13], params[14] = []byte("SAVES"), []byte("TEST.GAM")
	r, err := p.Compile(populous2.NativeMenuFiles, params)
	if err != nil {
		t.Fatal(err)
	}
	wanted, err := p.Compose(make([]byte, 320*200), r, pal)
	if err != nil {
		t.Fatal(err)
	}
	got := image.NewRGBA(wanted.Bounds())
	draw.Draw(got, got.Bounds(), image.NewUniform(pal[0]), image.Point{}, draw.Src)
	l.Draw(got, font, values, nil)
	want := image.NewRGBA(wanted.Bounds())
	draw.Draw(want, want.Bounds(), wanted, image.Point{}, draw.Src)
	if !bytes.Equal(got.Pix, want.Pix) {
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
					t.Fatalf("file requester pixel differs at%d,%d got%v want%v", x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
				}
			}
		}
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			action := p.Requesters.Click(r, x, y)
			name := map[int]string{2: "scroll-up", 28: "scroll-down", 30: "directory", 32: "name", 34: "submit", 36: "cancel"}[action]
			if action >= 4 && action <= 26 {
				name = fmt.Sprintf("file-%d", (action-4)/2)
			}
			if l.ActionAt(x, y) != name {
				t.Fatalf("file hit differs%d,%d", x, y)
			}
		}
	}
}
