package main

import (
	"strings"

	"go-populous2/internal/populous2"
)

func exportNetworkLayout(source *populous2.Bundle, output string) error {
	ui, err := interfaceSource(source)
	if err != nil {
		return err
	}
	p, err := populous2.DecodeNativePresentation(ui.Executable)
	if err != nil {
		return err
	}
	pal, err := populous2.NativeWorldPalette(ui)
	if err != nil {
		return err
	}
	fields := []NamedRequesterField{{Name: "mode", Marker: "HOST"}, {Name: "address", Marker: strings.Repeat("A", 18)}, {Name: "status", Marker: strings.Repeat("S", 30)}}
	params := make([][]byte, len(fields))
	for i, f := range fields {
		params[i] = []byte(f.Marker)
	}
	r, err := p.Compile(populous2.NativeMenuSerial, params)
	if err != nil {
		return err
	}
	layout, err := decodeNamedRequester("network", p.Requesters, r, pal, map[int]string{2: "host", 4: "join", 6: "address", 8: "connect", 10: "cancel"}, fields)
	if err != nil {
		return err
	}
	// Serial hardware is a host boundary. Preserve original cells and controls
	// while naming the modern transport choices honestly in those same cells.
	for i := range layout.Fields {
		if layout.Fields[i].Name != "address" {
			layout.Fields[i].Padding = ' '
		}
	}
	for i := range layout.Cells {
		c := &layout.Cells[i]
		if c.Row == 3 && c.Column >= 3 && c.Column < 12 {
			c.Glyph = []byte("LINK MODE")[c.Column-3]
		}
		if c.Row == 4 && c.Column >= 3 && c.Column < 16 {
			c.Glyph = []byte("HOST ADDRESS ")[c.Column-3]
		}
		if c.Row == 3 && c.Column == 21 {
			c.Glyph = ' '
		}
	}
	return writeLayout(output, "network-layout.json", layout)
}
