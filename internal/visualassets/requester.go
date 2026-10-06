package visualassets

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/fs"
)

type GlyphCell struct {
	Column, Row int
	Glyph       byte
}
type RequesterField struct {
	Name               string
	Column, Row, Width int
	Padding            byte `json:"padding,omitempty"`
}
type RequesterAction struct {
	Name                string
	X, Y, Width, Height int
}
type RequesterCheckbox struct {
	Name                         string
	Column, Row                  int
	CheckedGlyph, UncheckedGlyph byte
}

// RequesterLayout is a decoded visual grid with named dynamic fields and
// actions. Its runtime never reads template opcodes or source addresses.
type RequesterLayout struct {
	Version                         int    `json:"version"`
	Name                            string `json:"name"`
	OriginX, OriginY, Columns, Rows int
	Palette                         [16]color.RGBA      `json:"palette"`
	Cells                           []GlyphCell         `json:"cells"`
	Fields                          []RequesterField    `json:"fields"`
	Actions                         []RequesterAction   `json:"actions"`
	Checkboxes                      []RequesterCheckbox `json:"checkboxes,omitempty"`
}

func (l *RequesterLayout) ActionAt(x, y int) string {
	if l == nil {
		return ""
	}
	for _, a := range l.Actions {
		if image.Pt(x, y).In(image.Rect(a.X, a.Y, a.X+a.Width, a.Y+a.Height)) {
			return a.Name
		}
	}
	return ""
}
func (l *RequesterLayout) Draw(dst *image.RGBA, font *Font, values map[string]string, flags map[string]bool) {
	if l == nil || dst == nil || font == nil {
		return
	}
	for _, cell := range l.Cells {
		font.Draw(dst, string([]byte{cell.Glyph}), l.OriginX+cell.Column*8, l.OriginY+cell.Row*8, l.Palette)
	}
	for _, field := range l.Fields {
		text := []byte(values[field.Name])
		if len(text) > field.Width {
			text = text[len(text)-field.Width:]
		}
		padded := make([]byte, field.Width)
		padding := field.Padding
		if padding == 0 {
			padding = 'k'
		}
		for i := range padded {
			padded[i] = padding
		}
		copy(padded, text)
		font.Draw(dst, string(padded), l.OriginX+field.Column*8, l.OriginY+field.Row*8, l.Palette)
	}
	for _, box := range l.Checkboxes {
		glyph := box.UncheckedGlyph
		if flags[box.Name] {
			glyph = box.CheckedGlyph
		}
		font.Draw(dst, string([]byte{glyph}), l.OriginX+box.Column*8, l.OriginY+box.Row*8, l.Palette)
	}
}
func LoadRequesterLayout(files fs.FS, name string) (*RequesterLayout, error) {
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid requester asset path")
	}
	f, err := files.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var layout RequesterLayout
	if err := decoder.Decode(&layout); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("requester has trailing data")
	}
	if err := layout.Validate(); err != nil {
		return nil, err
	}
	return &layout, nil
}
func (l *RequesterLayout) Validate() error {
	if l == nil || l.Version != 1 || l.Name == "" || l.OriginX < 0 || l.OriginY < 0 || l.Columns < 1 || l.Columns > 40 || l.Rows < 1 || l.Rows > 25 || l.OriginX+l.Columns*8 > 320 || l.OriginY+l.Rows*8 > 200 || len(l.Cells) > 1000 || len(l.Fields) > 64 || len(l.Actions) > 128 || len(l.Checkboxes) > 32 {
		return fmt.Errorf("invalid requester layout")
	}
	inside := func(column, row, width int) bool {
		return column >= 0 && row >= 0 && width > 0 && column+width <= l.Columns && row < l.Rows
	}
	for _, cell := range l.Cells {
		if !inside(cell.Column, cell.Row, 1) || cell.Glyph == 0 || cell.Glyph == '\n' {
			return fmt.Errorf("invalid requester glyph cell")
		}
	}
	for _, field := range l.Fields {
		if field.Name == "" || !inside(field.Column, field.Row, field.Width) {
			return fmt.Errorf("invalid requester field")
		}
	}
	for _, box := range l.Checkboxes {
		if box.Name == "" || !inside(box.Column, box.Row, 1) {
			return fmt.Errorf("invalid requester checkbox")
		}
	}
	for _, action := range l.Actions {
		if action.Name == "" || action.X < 0 || action.Y < 0 || action.Width < 1 || action.Height < 1 || action.X+action.Width > 320 || action.Y+action.Height > 200 {
			return fmt.Errorf("invalid requester action")
		}
	}
	return nil
}
