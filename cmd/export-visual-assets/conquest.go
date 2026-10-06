package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportConquest(source *populous2.Bundle, output string) error {
	source, err := interfaceSource(source)
	if err != nil {
		return err
	}
	d, images, err := decodeConquest(source)
	if err != nil {
		return err
	}
	atlas, regions, err := pack("conquest-icons.png", images, nil)
	if err != nil {
		return err
	}
	copy(d.Icons[:], regions[:36])
	d.FaceBackdrop = regions[36]
	if err := writePNG(output, "conquest-icons.png", atlas); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.ConquestArtName), append(data, '\n'), 0644)
}

func decodeConquest(source *populous2.Bundle) (*visualassets.ConquestDescriptor, []*image.RGBA, error) {
	rules, err := populous2.DecodeNativeInGameRequesterRules(source.Executable)
	if err != nil {
		return nil, nil, err
	}
	p := rules.Presentation
	palette, err := populous2.NativeWorldPalette(source)
	if err != nil {
		return nil, nil, err
	}
	markers := []NamedRequesterField{{"world-code", "ABCDEFGHIJ"}, {"world-number", "1234567"}, {"landscape", "ABCDEFGHIJKLMNOPQRST"}, {"opponent", "ZYXWVUTSRQPONML"}}
	parameters := make([][]byte, len(markers))
	for i, m := range markers {
		parameters[i] = []byte(m.Marker)
	}
	requester, err := p.Compile(populous2.NativeMenuWorld, parameters)
	if err != nil {
		return nil, nil, err
	}
	layout, err := decodeNamedRequester("conquest", p.Requesters, requester, palette, map[int]string{2: "world-code", 4: "opponent", 6: "proceed", 8: "cancel"}, markers)
	if err != nil {
		return nil, nil, err
	}
	for i := range layout.Fields {
		if layout.Fields[i].Name != "world-code" {
			layout.Fields[i].Padding = ' '
		}
	}
	names := [...]string{"build-anywhere", "sea-level-only", "forbid-enemy-terrain", "forbid-raise", "forbid-lower", "fatal-water", "hide-enemy", "disable-emigration", "hide-disasters", "shallow-swamps"}
	for _, cell := range layout.Cells {
		if cell.Glyph == 'y' || cell.Glyph == 'z' {
			index := len(layout.Checkboxes)
			if index >= len(names) {
				return nil, nil, fmt.Errorf("unexpected conquest option checkbox")
			}
			layout.Checkboxes = append(layout.Checkboxes, visualassets.RequesterCheckbox{Name: names[index], Column: cell.Column, Row: cell.Row, CheckedGlyph: 'y', UncheckedGlyph: 'z'})
		}
	}
	if len(layout.Checkboxes) != len(names) {
		return nil, nil, fmt.Errorf("conquest option count differs")
	}
	d := &visualassets.ConquestDescriptor{Version: 1, Layout: *layout}
	code := source.Executable.Hunks[0].Data
	read := func(at int) (string, int, error) {
		if at < 0 || at >= len(code) {
			return "", 0, fmt.Errorf("interface string is outside source")
		}
		n := bytes.IndexByte(code[at:], 0)
		if n < 0 || n > 128 {
			return "", 0, fmt.Errorf("interface string lacks a bounded terminator")
		}
		return string(code[at : at+n]), at + n + 1, nil
	}
	for i := range d.LandscapeNames {
		d.LandscapeNames[i], _, err = read(0xa825 + i*14)
		if err != nil {
			return nil, nil, err
		}
	}
	bio := 0x9b76
	for i := range d.Opponents {
		info := &d.Opponents[i]
		info.Name, _, err = read(0x96d6 + i*14)
		if err != nil {
			return nil, nil, err
		}
		info.Realm, _, err = read(0x9896 + i*23)
		if err != nil {
			return nil, nil, err
		}
		copy(info.FaceParts[:], code[0x96d6+i*14+11:0x96d6+i*14+14])
		for line := range info.Biography {
			info.Biography[line], bio, err = read(bio)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	for i := range d.SpeedLabels {
		at := 0xa877 + i*5
		if at >= 0xa8bd {
			at = 0xa8b3
		}
		d.SpeedLabels[i], _, err = read(at)
		if err != nil {
			return nil, nil, err
		}
	}
	for i := range d.AggressionLabels {
		at := 0xa8be + i*12
		if at >= 0xa91e {
			at = 0xa912
		}
		d.AggressionLabels[i], _, err = read(at)
		if err != nil {
			return nil, nil, err
		}
	}
	// Unique full-width sentinels identify ordinary text fields without keeping
	// the original requester's substitution grammar in portable metadata.
	opponentMarkers := []NamedRequesterField{{"name", strings.Repeat("N", 20)}, {"realm", strings.Repeat("R", 37)}, {"speed", strings.Repeat("S", 25)}, {"aggression", strings.Repeat("A", 33)}}
	for i := 0; i < 5; i++ {
		opponentMarkers = append(opponentMarkers, NamedRequesterField{fmt.Sprintf("biography-%d", i), strings.Repeat(string(byte('B'+i)), 37)})
	}
	parameters = make([][]byte, len(opponentMarkers))
	for i, m := range opponentMarkers {
		parameters[i] = []byte(m.Marker)
	}
	requester, err = p.Compile(populous2.NativeMenuOpponent, parameters)
	if err != nil {
		return nil, nil, err
	}
	opponent, err := decodeNamedRequester("opponent", p.Requesters, requester, palette, map[int]string{2: "return"}, opponentMarkers)
	if err != nil {
		return nil, nil, err
	}
	for i := range opponent.Fields {
		opponent.Fields[i].Padding = ' '
	}
	d.OpponentLayout = *opponent
	backing := int(binary.BigEndian.Uint16(code[0xb12c:]))
	d.FaceBackdropPosition = image.Pt((backing%40)*8, backing/40)
	for part := 2; part >= 0; part-- {
		at := 0xb12e + (2-part)*6
		if int(binary.BigEndian.Uint16(code[at:])) != part*8 {
			return nil, nil, fmt.Errorf("opponent portrait layer bank differs")
		}
		d.FacePositions[part] = image.Pt(int(binary.BigEndian.Uint16(code[at+2:])), int(binary.BigEndian.Uint16(code[at+4:])))
	}
	images := make([]*image.RGBA, 37)
	for slot := 0; slot < 36; slot++ {
		images[slot], err = rules.WorldIcon(slot, palette)
		if err != nil {
			return nil, nil, err
		}
	}
	face := image.NewRGBA(image.Rect(0, 0, 48, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 48; x++ {
			face.SetRGBA(x, y, palette[p.Widgets.FaceBackdrop[x+y*48]])
		}
	}
	images[36] = face
	return d, images, nil
}
