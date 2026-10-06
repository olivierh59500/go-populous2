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

func exportHUD(source *populous2.Bundle, output string) error {
	descriptor, images, err := decodeHUD(source)
	if err != nil {
		return err
	}
	atlas, regions, err := pack("hud-icons.png", images, nil)
	if err != nil {
		return err
	}
	if err := writePNG(output, "hud-icons.png", atlas); err != nil {
		return err
	}
	descriptor.DisabledIcon = regions[0]
	for slot := range descriptor.Icons {
		descriptor.Icons[slot] = regions[slot+1]
	}
	data, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.HUDFile), append(data, '\n'), 0644)
}
func decodeHUD(source *populous2.Bundle) (*visualassets.HUDDescriptor, []*image.RGBA, error) {
	r, err := populous2.DecodeNativeHUDRules(source.Executable)
	if err != nil {
		return nil, nil, err
	}
	panel, err := populous2.DecodeNativeProfilePanelFrameRules(source.Executable)
	if err != nil {
		return nil, nil, err
	}
	code := source.Executable.Hunks[0].Data
	word := func(at int) int { return int(int16(binary.BigEndian.Uint16(code[at:]))) }
	descriptor := &visualassets.HUDDescriptor{Version: 1, IconPalette: source.Landscapes[0].Palettes[0]}
	images := make([]*image.RGBA, 37)
	decodeIcon := func(offset int) (*image.RGBA, error) {
		icon, ok := panel.Icons[0x214b2+offset]
		if !ok {
			return nil, fmt.Errorf("HUD icon descriptor unavailable")
		}
		return populous2.DecodeNativeMaskedPlanes(icon.Planes, icon.Width, icon.Height, source.Landscapes[0].Palettes[0])
	}
	images[0], err = decodeIcon(0)
	if err != nil {
		return nil, nil, err
	}
	for slot := range descriptor.Icons {
		images[slot+1], err = decodeIcon(word(0x21102 + slot*2))
		if err != nil {
			return nil, nil, err
		}
	}
	for row := range descriptor.IconPositions {
		descriptor.IconPositions[row] = image.Pt(word(0x2114a+row*4), word(0x2114c+row*4))
	}
	for slot, bar := range r.Bars {
		descriptor.Bars[slot] = visualassets.HUDBar{X: int(bar[0]), Y: int(bar[1]), Color: uint8(bar[2])}
	}
	for _, layer := range r.Indicator {
		descriptor.Indicator.Layers = append(descriptor.Indicator.Layers, visualassets.SpriteLayer{Sprite: layer.Sprite, X: int(layer.X), Y: int(layer.Y)})
	}
	for owner, points := range r.Population {
		for index, point := range points {
			descriptor.Population[owner][index] = visualassets.HUDPopulationPoint{XByte: int(point[0]>>3) + 31, Y: int(point[1] / 40), Variant: int(point[0]&7) / 2}
		}
	}
	for owner := range r.PopulationInk {
		for variant := range r.PopulationInk[owner] {
			for phase := range r.PopulationInk[owner][variant] {
				for row, ink := range r.PopulationInk[owner][variant][phase] {
					descriptor.Stencils[owner][variant][phase][row] = visualassets.HUDStencilRow{Preserve: ink[0], Ink: [4]uint8{ink[1], ink[2], ink[3], ink[4]}}
				}
			}
		}
	}
	highlight := func(at, pattern int) visualassets.HUDHighlight {
		h := visualassets.HUDHighlight{X: word(at) * 8, Y: word(at + 2)}
		for row := range h.Mask {
			copy(h.Mask[row][:], code[pattern+row*4:pattern+row*4+4])
		}
		return h
	}
	descriptor.InspectHighlight = highlight(0x33294+8, 0x3dd68)
	for index, offset := range [4]int{20, 24, 12, 16} {
		descriptor.ModeHighlights[index] = highlight(0x33294+offset, 0x3dd68)
	}
	for category := range descriptor.CategoryHighlights {
		descriptor.CategoryHighlights[category] = highlight(0x33294+0x1c+category*4, 0x3dd98)
	}
	for index := range descriptor.Controls {
		target := 0x16a2 + word(0x16a2+index*2)
		switch target {
		case 0x16d4:
			descriptor.Controls[index] = "inspect"
		case 0x16fc:
			descriptor.Controls[index] = "settle"
		case 0x1716:
			descriptor.Controls[index] = "join"
		case 0x1730:
			descriptor.Controls[index] = "fight"
		case 0x174a:
			descriptor.Controls[index] = "rally"
		case 0x17c0:
			descriptor.Controls[index] = "menu"
		case 0x17c2:
			descriptor.Controls[index] = "menu"
		case 0x17e4:
		default:
			return nil, nil, fmt.Errorf("HUD control source body%d unsupported", index)
		}
	}
	return descriptor, images, nil
}
