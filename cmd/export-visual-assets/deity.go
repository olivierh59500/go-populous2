package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportDeityWidgets(output string, source *populous2.Bundle) error {
	uiSource, err := interfaceSource(source)
	if err != nil {
		return err
	}
	presentation, err := populous2.DecodeNativePresentation(uiSource.Executable)
	if err != nil {
		return err
	}
	palette, err := populous2.NativeWorldPalette(uiSource)
	if err != nil {
		return err
	}
	widgets := presentation.Widgets
	if err := exportDeityLayout(output, presentation, palette); err != nil {
		return err
	}
	images := make([]*image.RGBA, 0, 25)
	for _, pixels := range widgets.Bars {
		img := image.NewRGBA(image.Rect(0, 0, 16, 32))
		for y := 0; y < 32; y++ {
			for x := 0; x < 16; x++ {
				img.SetRGBA(x, y, palette[pixels[x+y*16]])
			}
		}
		images = append(images, img)
	}
	face := image.NewRGBA(image.Rect(0, 0, 48, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 48; x++ {
			face.SetRGBA(x, y, palette[widgets.FaceBackdrop[x+y*48]])
		}
	}
	images = append(images, face)
	atlas, regions, err := pack("deity-widgets.png", images, nil)
	if err != nil {
		return err
	}
	if err = writePNG(output, "deity-widgets.png", atlas); err != nil {
		return err
	}
	desc := visualassets.DeityWidgetsDescriptor{FaceBackdrop: regions[24]}
	copy(desc.Bars[:], regions[:24])
	for i, p := range widgets.FacePositions {
		desc.FacePositions[i] = image.Pt(int(p[0]), int(p[1]))
	}
	portraits, err := populous2.DecodeDeityArt(uiSource.Executable, source.Raw["faces.pak"], palette)
	if err != nil {
		return err
	}
	var portraitImages []*image.RGBA
	for _, parts := range portraits.Parts {
		portraitImages = append(portraitImages, parts[:]...)
	}
	portraitAtlas, portraitRegions, err := pack("deity-portraits.png", portraitImages, nil)
	if err != nil {
		return err
	}
	if err = writePNG(output, "deity-portraits.png", portraitAtlas); err != nil {
		return err
	}
	for part := range desc.PortraitParts {
		copy(desc.PortraitParts[part][:], portraitRegions[part*8:(part+1)*8])
	}
	data, err := json.MarshalIndent(desc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.DeityWidgetsName), append(data, '\n'), 0644)
}

func exportDeityLayout(output string, presentation *populous2.NativePresentation, palette [16]color.RGBA) error {
	requester, err := presentation.Compile(populous2.NativeMenuDeity, [][]byte{[]byte("ABCDEFGHIJKLMNOP"), []byte("NOTHING"), []byte("ABCDEFGHIJKLMNOP")})
	if err != nil {
		return err
	}
	rows := strings.Split(string(requester.Text), "\n")
	layout := visualassets.RequesterLayout{Version: 1, Name: "deity", OriginX: requester.Column * 8, OriginY: requester.Row, Columns: 40, Rows: 25, Palette: palette}
	for row, line := range rows {
		for column, glyph := range []byte(line) {
			if glyph == '\r' || glyph == 0 {
				continue
			}
			layout.Cells = append(layout.Cells, visualassets.GlyphCell{Column: column, Row: row, Glyph: glyph})
		}
	}
	for _, field := range []struct {
		name  string
		row   int
		text  string
		width int
	}{{"name", 2, "ABCDEFGHIJKLMNOP", 16}, {"bolts", 16, "NOTHING", 16}, {"password", 18, "ABCDEFGHIJKLMNOP", 19}} {
		if field.row >= len(rows) {
			return fmt.Errorf("deity field row unavailable")
		}
		column := strings.Index(rows[field.row], field.text)
		if column < 0 {
			return fmt.Errorf("deity field %s missing", field.name)
		}
		layout.Fields = append(layout.Fields, visualassets.RequesterField{Name: field.name, Column: column, Row: field.row, Width: field.width})
		if field.name == "bolts" {
			layout.Fields[len(layout.Fields)-1].Padding = ' '
		}
	}
	actionNames := map[int]string{2: "name", 22: "face-0-prev", 24: "face-0-next", 32: "face-1-prev", 34: "face-1-next", 48: "face-2-prev", 50: "face-2-next", 64: "password", 66: "proceed"}
	for element, actions := range [6][]int{{4, 10, 16, 26}, {6, 12, 18, 28}, {8, 14, 20, 30}, {36, 42, 52, 58}, {38, 44, 54, 60}, {40, 46, 56, 62}} {
		for _, a := range actions {
			actionNames[a] = fmt.Sprintf("experience-%d", element)
		}
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; {
			a := presentation.Requesters.Click(requester, x, y)
			if a == 0 {
				x++
				continue
			}
			name := actionNames[a]
			if name == "" {
				return fmt.Errorf("unknown deity action%d", a)
			}
			start := x
			for x < 320 && presentation.Requesters.Click(requester, x, y) == a {
				x++
			}
			width := x - start
			merged := false
			for i := range layout.Actions {
				prev := &layout.Actions[i]
				if prev.Name == name && prev.X == start && prev.Width == width && prev.Y+prev.Height == y {
					prev.Height++
					merged = true
					break
				}
			}
			if !merged {
				layout.Actions = append(layout.Actions, visualassets.RequesterAction{Name: name, X: start, Y: y, Width: width, Height: 1})
			}
		}
	}
	if err := layout.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "deity-layout.json"), append(data, '\n'), 0644)
}
