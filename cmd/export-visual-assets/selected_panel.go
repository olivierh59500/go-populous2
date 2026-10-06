package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

// exportSelectedPanel decodes UI artwork and coordinates once during import.
// Runtime panel metadata contains no source addresses or drawing procedures.
func exportSelectedPanel(source *populous2.Bundle, output string) error {
	panel, err := decodeSelectedPanel(source)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(panel, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.SelectionPanelName), append(data, '\n'), 0644)
}
func decodeSelectedPanel(source *populous2.Bundle) (*visualassets.SelectionPanel, error) {
	if source == nil || source.Executable == nil || len(source.Executable.Hunks) == 0 || len(source.Executable.Hunks[0].Data) < 0x212ba {
		return nil, fmt.Errorf("selected panel source artwork is missing")
	}
	code := source.Executable.Hunks[0].Data
	word := func(at int) int { return int(int16(binary.BigEndian.Uint16(code[at:]))) }
	p := &visualassets.SelectionPanel{Version: 1, ActorX: word(0x2128a), ActorY: word(0x2128c), WeaponX: word(0x2128e), WeaponY: word(0x21290), HitX: word(0x21292), HitY: word(0x21294), HitWidth: word(0x21296), HitHeight: word(0x21298), PopulationSprite: 0x84c / 12, Weapons: make([]visualassets.Frame, 19)}
	for i := range p.PopulationPoints {
		p.PopulationPoints[i] = image.Pt(word(0x2129a+i*4), word(0x2129c+i*4))
	}
	for stage, frame := range source.TownCenterArt.Frames {
		for _, layer := range frame.Layers {
			if layer.Sprite == 89 {
				continue
			}
			at := 0x21626 + layer.Sprite*12
			if at < 0 || at+8 > len(code) {
				return nil, fmt.Errorf("town hit descriptor is unavailable")
			}
			p.TownHitHeights[stage] = int(binary.BigEndian.Uint16(code[at+6:]))
		}
	}
	for weapon := range p.Weapons {
		start := int(binary.BigEndian.Uint16(code[0x20b60+weapon*2:])) + 0x140
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil || len(frames) == 0 {
			return nil, fmt.Errorf("selected weapon%d artwork: %w", weapon, err)
		}
		layers := make([]visualassets.SpriteLayer, len(frames[0].Layers))
		for i, layer := range frames[0].Layers {
			layers[i] = visualassets.SpriteLayer{Sprite: layer.Sprite, X: layer.X, Y: layer.Y}
		}
		p.Weapons[weapon] = visualassets.Frame{Layers: layers}
	}
	return p, nil
}
