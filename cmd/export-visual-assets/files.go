package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportFileLayouts(source *populous2.Bundle, output string) error {
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
	params := make([][]byte, 16)
	var fields []NamedRequesterField
	params[0], params[15] = []byte("LOAD"), []byte("LOAD")
	for i := 0; i < 12; i++ {
		marker := fmt.Sprintf("FILE%02d", i)
		params[i+1] = []byte(marker)
		fields = append(fields, NamedRequesterField{Name: fmt.Sprintf("file-%d", i), Marker: marker})
	}
	params[13], params[14] = []byte(strings.Repeat("D", 13)), []byte(strings.Repeat("N", 13))
	fields = append(fields, NamedRequesterField{Name: "directory", Marker: string(params[13])}, NamedRequesterField{Name: "name", Marker: string(params[14])})
	r, err := p.Compile(populous2.NativeMenuFiles, params)
	if err != nil {
		return err
	}
	actions := map[int]string{2: "scroll-up", 28: "scroll-down", 30: "directory", 32: "name", 34: "submit", 36: "cancel"}
	for i := 0; i < 12; i++ {
		actions[4+i*2] = fmt.Sprintf("file-%d", i)
	}
	layout, err := decodeNamedRequester("files", p.Requesters, r, palette, actions, fields)
	if err != nil {
		return err
	}
	for i := range layout.Fields {
		if strings.HasPrefix(layout.Fields[i].Name, "file-") {
			layout.Fields[i].Padding = ' '
			layout.Fields[i].Width = 23
		}
	}
	// Both verb locations occupy four original cells and use named text.
	rows := strings.Split(string(r.Text), "\n")
	for row, line := range rows {
		start := 0
		for {
			col := strings.Index(line[start:], "LOAD")
			if col < 0 {
				break
			}
			col += start
			layout.Fields = append(layout.Fields, visualassets.RequesterField{Name: "verb", Column: col, Row: row, Width: 4, Padding: ' '})
			start = col + 4
		}
	}
	if err := writeLayout(output, "files-layout.json", layout); err != nil {
		return err
	}
	for _, spec := range []struct {
		name     string
		template populous2.NativeMenuTemplate
		marker   string
		actions  map[int]string
	}{{"overwrite", populous2.NativeMenuOverwrite, strings.Repeat("N", 22), map[int]string{2: "replace", 4: "cancel"}}, {"file-error", populous2.NativeMenuMessage, strings.Repeat("E", 22), map[int]string{2: "dismiss"}}} {
		r, err := p.Compile(spec.template, [][]byte{[]byte(spec.marker)})
		if err != nil {
			return err
		}
		l, err := decodeNamedRequester(spec.name, p.Requesters, r, palette, spec.actions, []NamedRequesterField{{Name: "message", Marker: spec.marker}})
		if err != nil {
			return err
		}
		if err := writeLayout(output, spec.name+"-layout.json", l); err != nil {
			return err
		}
	}
	return nil
}

func writeLayout(output, name string, layout *visualassets.RequesterLayout) error {
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, name), append(data, '\n'), 0644)
}
