package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeTownCenterArt is the kind-4/state-6 center renderer at $e72e.
// The neighboring eight-cell compositor is a separate overlay plane.
type NativeTownCenterArt struct {
	Pointers           [TownStages]uint16
	Frames             [TownStages]AnimationFrame
	PopulationDivisors [TownStages]uint16
}

func DecodeNativeTownCenterArt(exe *amiga.Executable) (NativeTownCenterArt, error) {
	var art NativeTownCenterArt
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x26956 {
		return art, fmt.Errorf("native town center tables missing")
	}
	code := exe.Hunks[0].Data
	for stage := range art.Frames {
		art.Pointers[stage] = binary.BigEndian.Uint16(code[0x20b14+stage*2:])
		art.PopulationDivisors[stage] = binary.BigEndian.Uint16(code[0x20aee+stage*2:])
		at := 0x23d1a + int(art.Pointers[stage])
		if at+4 > len(code) || art.Pointers[stage]&3 != 0 || art.PopulationDivisors[stage] == 0 {
			return NativeTownCenterArt{}, fmt.Errorf("invalid native town center record")
		}
		image := int16(binary.BigEndian.Uint16(code[at:]))
		if image < 0 {
			return NativeTownCenterArt{}, fmt.Errorf("native town center points to animation marker")
		}
		layers, err := decodeImageLayers(code, uint16(image))
		if err != nil {
			return NativeTownCenterArt{}, err
		}
		// Unlike $ee32, this renderer reads the image directly and does not
		// trigger the animation record's cue word.
		art.Frames[stage] = AnimationFrame{Layers: layers}
	}
	return art, nil
}

// Frame returns layers relative to the ordinary projected actor anchor.
// Native $e756 adds eight pixels to that anchor before drawing the center;
// these layers include that shift. A doubled display scales each offset by2.
// Owner is the original positive byte1/2, rather than a Go player index.
func (art *NativeTownCenterArt) Frame(stage int, owner uint8, population uint32, tick uint16) (AnimationFrame, bool) {
	if art == nil || stage < 0 || stage >= len(art.Frames) || owner < 1 || owner > 2 || art.PopulationDivisors[stage] == 0 {
		return AnimationFrame{}, false
	}
	frame := AnimationFrame{Layers: append([]SpriteLayer(nil), art.Frames[stage].Layers...)}
	for index := range frame.Layers {
		layer := &frame.Layers[index]
		layer.Y += 8
		if layer.Sprite != 89 {
			continue
		}
		// Original DIVU leaves its dividend unchanged on quotient overflow.
		// CMP.W/BLE then compares the result's low word as a signed value.
		quotient := population / uint32(art.PopulationDivisors[stage])
		word := uint16(quotient)
		if quotient > 0xffff {
			word = uint16(population)
		}
		height := int16(word)
		if height > 24 {
			height = 24
		}
		shift := uint16(24) - uint16(height)
		layer.Y = int(int16(uint16(layer.Y) + shift))
		layer.Sprite += int(tick&1) + int(owner-1)*2
	}
	return frame, true
}
