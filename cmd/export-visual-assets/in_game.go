package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportInGameLayout(source *populous2.Bundle, output string) error {
	ui, err := interfaceSource(source)
	if err != nil {
		return err
	}
	p, err := populous2.DecodeNativePresentation(ui.Executable)
	if err != nil {
		return err
	}
	palette, err := populous2.NativeWorldPalette(ui)
	if err != nil {
		return err
	}
	r, err := p.Compile(populous2.NativeMenuInGame, [][]byte{[]byte("AAAA"), []byte("HHH"), []byte("EEEEEEEEEEEEEEEEEEE")})
	if err != nil {
		return err
	}
	actions := map[int]string{2: "profile", 4: "assist", 6: "opponent-control", 8: "load", 10: "save", 12: "restart", 14: "quit-map", 16: "network", 18: "editor", 20: "options", 22: "about", 24: "resume"}
	layout, err := decodeNamedRequester("in-game", p.Requesters, r, palette, actions, []NamedRequesterField{{Name: "side", Marker: "AAAA"}, {Name: "assist", Marker: "HHH"}, {Name: "opponent", Marker: "EEEEEEEEEEEEEEEEEEE"}})
	if err != nil {
		return err
	}
	for i := range layout.Fields {
		layout.Fields[i].Padding = ' '
	}
	mode, radio := 0, 0
	for _, cell := range layout.Cells {
		if cell.Glyph == 'y' {
			name := "conquest"
			if mode != 0 {
				name = "custom"
			}
			layout.Checkboxes = append(layout.Checkboxes, visualassets.RequesterCheckbox{Name: name, Column: cell.Column, Row: cell.Row, CheckedGlyph: 'y', UncheckedGlyph: 'z'})
			mode++
		}
		if cell.Glyph == 'c' {
			name := "network"
			if radio != 0 {
				name = "paint"
			}
			layout.Checkboxes = append(layout.Checkboxes, visualassets.RequesterCheckbox{Name: name, Column: cell.Column, Row: cell.Row, CheckedGlyph: 'd', UncheckedGlyph: 'c'})
			radio++
		}
	}
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "in-game-layout.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	rules, err := populous2.DecodeNativeInGameRequesterRules(ui.Executable)
	if err != nil {
		return err
	}
	about, err := rules.AboutPlan()
	if err != nil {
		return err
	}
	aboutLayout, err := decodeNamedRequester("about", p.Requesters, about, palette, map[int]string{2: "resume"}, nil)
	if err != nil {
		return err
	}
	return writeLayout(output, "about-layout.json", aboutLayout)
}
