package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportEditor(source *populous2.Bundle, output string) error {
	source, err := interfaceSource(source)
	if err != nil {
		return err
	}
	p, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		return err
	}
	fields := []NamedRequesterField{{"time", "TIMEXXXXXXX"}, {"x", "POSX"}, {"y", "POSY"}, {"effect", "EFFECTXXXXX"}}
	parameters := make([][]byte, len(fields))
	for i, f := range fields {
		parameters[i] = []byte(f.Marker)
	}
	r, err := p.Compile(populous2.NativeMenuPaint, parameters)
	if err != nil {
		return err
	}
	actions := map[int]string{2: "blue", 4: "red", 6: "tree", 8: "rock", 10: "local-mana-add", 12: "local-mana-subtract", 14: "opponent-mana-add", 16: "opponent-mana-subtract", 18: "landscape", 20: "new-map", 22: "time", 24: "x", 26: "y", 28: "effect", 30: "next-event", 32: "previous-event"}
	layout, err := decodeNamedRequester("editor", p.Requesters, r, source.Landscapes[0].Palettes[0], actions, fields)
	if err != nil {
		return err
	}
	for _, cell := range layout.Cells {
		if cell.Glyph == 'c' {
			index := len(layout.Checkboxes)
			names := []string{"blue", "red", "tree", "rock", "new-map"}
			if index < len(names) {
				layout.Checkboxes = append(layout.Checkboxes, visualassets.RequesterCheckbox{Name: names[index], Column: cell.Column, Row: cell.Row, CheckedGlyph: 'd', UncheckedGlyph: 'c'})
			}
		}
	}
	previews := make(map[string]visualassets.Frame)
	for index, name := range []string{"blue", "red", "tree", "rock"} {
		start := int(binary.BigEndian.Uint16(source.Executable.Hunks[0].Data[0x20ac2+index*2:]))
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil || len(frames) == 0 {
			return fmt.Errorf("editor brush animation missing: %w", err)
		}
		previews[name] = animation(frames, false).Frames[0]
	}
	previewData, err := json.MarshalIndent(previews, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, visualassets.EditorPreviewFile), append(previewData, '\n'), 0644); err != nil {
		return err
	}
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "editor-layout.json"), append(data, '\n'), 0644)
}
