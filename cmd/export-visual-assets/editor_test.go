package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateEditorRequesterMatchesOriginalPixelsAndAllControls(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original UI directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	source, err = interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := exportEditor(source, directory); err != nil {
		t.Fatal(err)
	}
	layout, err := visualassets.LoadRequesterLayout(os.DirFS(directory), "editor-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	font := &visualassets.Font{FirstCode: 32, Width: 8, Height: 8, Glyphs: make([][]uint8, len(p.Font.Glyphs))}
	for id, g := range p.Font.Glyphs {
		font.Glyphs[id] = append([]uint8(nil), g[:]...)
	}
	actions := map[int]string{2: "blue", 4: "red", 6: "tree", 8: "rock", 10: "local-mana-add", 12: "local-mana-subtract", 14: "opponent-mana-add", 16: "opponent-mana-subtract", 18: "landscape", 20: "new-map", 22: "time", 24: "x", 26: "y", 28: "effect", 30: "next-event", 32: "previous-event"}
	for _, sample := range []struct {
		values   []string
		selected string
	}{{[]string{"0", "0", "0", "0"}, "blue"}, {[]string{"65535", "63", "32", "100"}, "rock"}} {
		parameters := make([][]byte, 4)
		for id, v := range sample.values {
			parameters[id] = []byte(v)
		}
		requester, err := p.Compile(populous2.NativeMenuPaint, parameters)
		if err != nil {
			t.Fatal(err)
		}
		selected := map[string]int{"blue": 0, "red": 1, "tree": 2, "rock": 3}[sample.selected]
		radio := 0
		for id, glyph := range requester.Text {
			if glyph == 'c' {
				if radio == selected {
					requester.Text[id] = 'd'
				}
				radio++
			}
		}
		expected, err := p.Compose(p.StartupPixels, requester, layout.Palette)
		if err != nil {
			t.Fatal(err)
		}
		base, err := p.Compose(p.StartupPixels, &populous2.NativeRequester{}, layout.Palette)
		if err != nil {
			t.Fatal(err)
		}
		got := image.NewRGBA(image.Rect(0, 0, 320, 200))
		draw.Draw(got, got.Bounds(), base, image.Point{}, draw.Src)
		layout.Draw(got, font, map[string]string{"time": sample.values[0], "x": sample.values[1], "y": sample.values[2], "effect": sample.values[3]}, map[string]bool{sample.selected: true})
		want := image.NewRGBA(got.Bounds())
		draw.Draw(want, want.Bounds(), expected, image.Point{}, draw.Src)
		if !bytes.Equal(got.Pix, want.Pix) {
			reportFirstPixelDifference(t, got, want)
		}
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				r := *requester
				r.Text = append([]byte(nil), requester.Text...)
				command := p.Requesters.Click(&r, x, y)
				if layout.ActionAt(x, y) != actions[command] {
					t.Fatalf("original editor actionregion differs at%d,%d", x, y)
				}
			}
		}
	}
}

func TestPrivateEditorBrushPreviewsMatchOriginalFrameAndAnchor(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set private original editor directory")
	}
	source, err := populous2.LoadFS(os.DirFS(input))
	if err != nil {
		t.Fatal(err)
	}
	source, err = interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := exportEditor(source, directory); err != nil {
		t.Fatal(err)
	}
	previews, err := visualassets.LoadEditorPreview(os.DirFS(directory))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeEditorCursorRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for index, name := range []string{"blue", "red", "tree", "rock"} {
		raw := make([]byte, 0x11280)
		put := func(at int, value uint16) { binary.BigEndian.PutUint16(raw[at:], value) }
		put(0xf0e, 1)
		put(0xf10, uint16((index+1)*2))
		put(0x138, 190)
		put(0x13a, 95)
		memory := populous2.FollowerCleanupMemory{Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }}
		state := rules.NewImageState()
		frame := [8]uint32{}
		sprites, err := rules.Preview(memory, &state, &frame)
		if err != nil {
			t.Fatal(err)
		}
		layers := previews[name].Layers
		if len(layers) != len(sprites) {
			t.Fatal("editor preview layer count differs", name)
		}
		for id, layer := range layers {
			sprite := source.Sprites[0][layer.Sprite]
			request := sprites[id]
			if layer.Sprite != request.Sprite || 190+layer.X-sprite.AnchorX != int(request.X) || 95+layer.Y-sprite.AnchorY != int(request.Y) {
				t.Fatal("original editor brush frame/anchor differs", name, layer, request)
			}
		}
	}
}

func TestPrivateEditorFiftiethEventTimeMatchesOriginalNumericEdit(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original editor directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	source, err = interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := exportEditor(source, directory); err != nil {
		t.Fatal(err)
	}
	layout, err := visualassets.LoadRequesterLayout(os.DirFS(directory), "editor-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativePresentationInputRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	state := rules.NewPaintingState()
	state.Index = 49 * 6
	raw := make([]byte, 0x11280)
	put := func(at int, value uint16) { binary.BigEndian.PutUint16(raw[at:], value) }
	put(0xeb44, 8)
	put(0xeb42, 1)
	put(0x140, 1)
	for _, a := range layout.Actions {
		if a.Name == "time" {
			put(0x134, uint16(a.X+1))
			put(0x136, uint16(a.Y+1))
			break
		}
	}
	const record = 0xdde + 49*6
	put(record, 10)
	put(record+2, 40)
	raw[record+4], raw[record+5] = 31, 32
	memory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil }, Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { put(at, v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil }}
	_, err = rules.Painting(&state, populous2.NativePaintingCallbacks{Memory: memory, EditNumber: func(field int, _ []byte, _ *populous2.NativeHUDRegisters) ([]byte, error) {
		if field != 0 {
			t.Fatal("wrong source numeric field")
		}
		return []byte("200"), nil
	}}, &populous2.NativeHUDRegisters{})
	if err != nil {
		t.Fatal(err)
	}
	world := &engine.World{Editor: true}
	event := engine.ScenarioEvent{Time: 200, Kind: engine.ScenarioEarthquake, X: 31, Y: 32}
	if err := world.EditorSetScenarioEvent(49, event); err != nil {
		t.Fatal(err)
	}
	encoded, err := engine.EncodeScenarioEvent(world.Scenario.Events[49])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded[:], raw[record:record+6]) {
		t.Fatal("fiftieth typed event differs from original numeric editor")
	}
}
