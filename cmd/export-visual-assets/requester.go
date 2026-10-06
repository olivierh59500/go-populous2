package main

import (
	"fmt"
	"image/color"
	"strings"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

type NamedRequesterField struct{ Name, Marker string }

// decodeNamedRequester flattens the original compiler result during import.
// Field markers must occupy their original fixed-width cells; they are replaced
// with named data fields, never retained as source template instructions.
func decodeNamedRequester(name string, rules populous2.NativeRequesterRules, requester *populous2.NativeRequester, palette [16]color.RGBA, actionNames map[int]string, fields []NamedRequesterField) (*visualassets.RequesterLayout, error) {
	if requester == nil {
		return nil, fmt.Errorf("missing requester compile result")
	}
	rows := strings.Split(strings.TrimRight(string(requester.Text), "\x00\n"), "\n")
	layout := &visualassets.RequesterLayout{Version: 1, Name: name, OriginX: requester.Column * 8, OriginY: requester.Row, Columns: requester.Width, Rows: len(rows), Palette: palette}
	for row, text := range rows {
		for column, glyph := range []byte(text) {
			layout.Cells = append(layout.Cells, visualassets.GlyphCell{Column: column, Row: row, Glyph: glyph})
		}
	}
	for _, field := range fields {
		found := false
		for row, text := range rows {
			if column := strings.Index(text, field.Marker); column >= 0 {
				layout.Fields = append(layout.Fields, visualassets.RequesterField{Name: field.Name, Column: column, Row: row, Width: len(field.Marker)})
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("requester field %s marker is absent", field.Name)
		}
	}
	for row := range rows {
		for column := 0; column < requester.Width; {
			clone := *requester
			clone.Text = append([]byte(nil), requester.Text...)
			action := rules.Click(&clone, layout.OriginX+column*8, layout.OriginY+row*8)
			if action == 0 {
				column++
				continue
			}
			start := column
			column++
			for column < requester.Width {
				probe := *requester
				probe.Text = append([]byte(nil), requester.Text...)
				if rules.Click(&probe, layout.OriginX+column*8, layout.OriginY+row*8) != action {
					break
				}
				column++
			}
			label := actionNames[action]
			if label == "" {
				return nil, fmt.Errorf("requester action %d has no semantic name", action)
			}
			layout.Actions = append(layout.Actions, visualassets.RequesterAction{Name: label, X: layout.OriginX + start*8, Y: layout.OriginY + row*8, Width: (column - start) * 8, Height: 8})
		}
	}
	if err := layout.Validate(); err != nil {
		return nil, fmt.Errorf("requester %s grid %dx%d at %d,%d: %w", name, layout.Columns, layout.Rows, layout.OriginX, layout.OriginY, err)
	}
	return layout, nil
}
