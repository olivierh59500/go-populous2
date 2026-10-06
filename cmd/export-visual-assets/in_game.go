package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go-populous2/internal/populous2"
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
	r, err := p.Compile(populous2.NativeMenuInGame, [][]byte{[]byte("AAAABBBB"), []byte("CCCCDDDD"), []byte("EEEEFFFF")})
	if err != nil {
		return err
	}
	actions := map[int]string{2: "profile", 4: "assist", 6: "opponent-control", 8: "load", 10: "save", 12: "restart", 14: "quit-map", 16: "network", 18: "editor", 20: "options", 22: "about", 24: "resume"}
	layout, err := decodeNamedRequester("in-game", p.Requesters, r, palette, actions, []NamedRequesterField{{Name: "side", Marker: "AAAABBBB"}, {Name: "assist", Marker: "CCCCDDDD"}, {Name: "opponent", Marker: "EEEEFFFF"}})
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "in-game-layout.json"), append(data, '\n'), 0644)
}
