package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportOptions(source *populous2.Bundle, output string) error {
	source, err := interfaceSource(source)
	if err != nil {
		return err
	}
	art, err := decodeOptionsArt(source)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.OptionsArtName), append(data, '\n'), 0644)
}

func decodeOptionsArt(source *populous2.Bundle) (*visualassets.OptionsArt, error) {
	p, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		return nil, err
	}
	palette, err := populous2.NativeWorldPalette(source)
	if err != nil {
		return nil, err
	}
	fields := []NamedRequesterField{{"side", strings.Repeat("S", 31)}, {"special-codes", strings.Repeat("C", 17)}}
	r, err := p.Compile(populous2.NativeMenuOptions, [][]byte{[]byte(fields[0].Marker), []byte(fields[1].Marker)})
	if err != nil {
		return nil, err
	}
	rules := p.Requesters
	rules.EnableOptionMarkers(true)
	// The movable reaction thumb is itself an action marker. It must be
	// present while counting the later special-code and confirmation actions.
	for i, glyph := range r.Text {
		if glyph == 'h' && i+1 < len(r.Text) {
			r.Text[i+1] = 'g'
			break
		}
	}
	actions := map[int]string{2: "side", 24: "reaction-decrease", 26: "reaction-increase", 28: "special-codes", 30: "proceed"}
	for i := 0; i < 10; i++ {
		actions[4+i*2] = fmt.Sprintf("rule-%d", i)
	}
	layout, err := decodeNamedRequester("options", rules, r, palette, actions, fields)
	if err != nil {
		return nil, err
	}
	layout.Fields[0].Padding = ' '
	art := &visualassets.OptionsArt{Version: 1, Layout: *layout, ReactionGlyph: 'g'}
	for i, at := range []int{0x9670, 0x9675} {
		data := source.Executable.Hunks[0].Data[at : at+31]
		end := bytes.IndexByte(data, 0)
		if end < 0 {
			return nil, fmt.Errorf("options side label lacks terminator")
		}
		art.SideNames[i] = string(data[:end])
	}
	for _, cell := range art.Layout.Cells {
		if cell.Glyph == 'y' || cell.Glyph == 'z' {
			art.Layout.Checkboxes = append(art.Layout.Checkboxes, visualassets.RequesterCheckbox{Name: fmt.Sprintf("rule-%d", len(art.Layout.Checkboxes)), Column: cell.Column, Row: cell.Row, CheckedGlyph: 'y', UncheckedGlyph: 'z'})
		}
		if cell.Glyph == 'h' {
			art.ReactionPosition = image.Pt(layout.OriginX+(cell.Column+1)*8, layout.OriginY+cell.Row*8)
		}
	}
	if len(art.Layout.Checkboxes) != 10 || art.ReactionPosition == (image.Point{}) {
		return nil, fmt.Errorf("options checkbox or reaction geometry differs")
	}
	for i := range art.Layout.Cells {
		cell := &art.Layout.Cells[i]
		if cell.Glyph == 'g' {
			cell.Glyph = 'i'
		}
	}
	return art, nil
}
