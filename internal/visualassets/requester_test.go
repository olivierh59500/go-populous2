package visualassets

import (
	"encoding/json"
	"image"
	"image/color"
	"testing"
	"testing/fstest"
)

func TestRequesterNamedFieldsPreserveTheirBackgroundAndHitEdges(t *testing.T) {
	font := &Font{FirstCode: 32, Width: 8, Height: 8, Glyphs: make([][]uint8, 96)}
	for index := range font.Glyphs {
		font.Glyphs[index] = make([]uint8, 64)
		for p := range font.Glyphs[index] {
			font.Glyphs[index][p] = uint8((index + 32) % 16)
		}
	}
	layout := &RequesterLayout{Version: 1, Name: "test", Columns: 4, Rows: 1, Fields: []RequesterField{{Name: "code", Column: 0, Row: 0, Width: 2}, {Name: "label", Column: 2, Row: 0, Width: 2, Padding: ' '}}, Actions: []RequesterAction{{Name: "proceed", X: 8, Y: 0, Width: 16, Height: 8}}}
	for i := range layout.Palette {
		layout.Palette[i] = color.RGBA{uint8(i), 0, 0, 255}
	}
	dst := image.NewRGBA(image.Rect(0, 0, 32, 8))
	layout.Draw(dst, font, map[string]string{"code": "A", "label": "B"}, nil)
	for _, sample := range []struct{ x, index int }{{0, int('A') % 16}, {8, int('k') % 16}, {16, int('B') % 16}, {24, int(' ') % 16}} {
		if got := dst.RGBAAt(sample.x, 0); got != layout.Palette[sample.index] {
			t.Fatal("field background padding changed", sample, got)
		}
	}
	for _, sample := range []struct {
		x, y   int
		action string
	}{{7, 0, ""}, {8, 0, "proceed"}, {23, 7, "proceed"}, {24, 0, ""}, {8, 8, ""}} {
		if got := layout.ActionAt(sample.x, sample.y); got != sample.action {
			t.Fatal("action boundary differs", sample, got)
		}
	}
}

func TestRequesterLoaderRejectsMalformedAndUnexpectedMetadata(t *testing.T) {
	layout := RequesterLayout{Version: 1, Name: "test", Columns: 40, Rows: 25, Cells: []GlyphCell{{Column: 1, Row: 1, Glyph: 'A'}}}
	data, err := json.Marshal(layout)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{"requester.json": &fstest.MapFile{Data: data}}
	if _, err := LoadRequesterLayout(files, "requester.json"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append(data, []byte(" {}")...), []byte(`{"version":1,"name":"test","Columns":40,"Rows":25,"program":"unused"}`), []byte(`{"version":1,"name":"test","Columns":40,"Rows":25,"fields":[{"Name":"overflow","Column":39,"Row":0,"Width":2}]}`)} {
		files["requester.json"].Data = bad
		if _, err := LoadRequesterLayout(files, "requester.json"); err == nil {
			t.Fatal("malformed requester metadata was accepted")
		}
	}
	if _, err := LoadRequesterLayout(files, "../requester.json"); err == nil {
		t.Fatal("requester path escaped its asset root")
	}
}

func TestConquestIconLatticeSelectsEveryAvailableColumn(t *testing.T) {
	art := &ConquestArt{}
	for row := 0; row < 6; row++ {
		for column := 0; column < 5; column++ {
			x, y := 46+row*32+column*16, 38+column*8
			if slot, ok := art.PowerAt(x, y); !ok || slot != row*6+column {
				t.Fatal("power lattice selected a different icon", row, column, slot, ok)
			}
		}
	}
	for _, p := range []image.Point{image.Pt(-1, 38), image.Pt(320, 38), image.Pt(46, -1), image.Pt(46, 200), image.Pt(126, 78)} {
		if _, ok := art.PowerAt(p.X, p.Y); ok {
			t.Fatal("point outside the power lattice was accepted", p)
		}
	}
}
