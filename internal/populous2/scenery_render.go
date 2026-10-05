package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type SceneryRenderDescriptor struct {
	Sprite                  int
	X, Y, HalfWidth, Height int16
}
type SceneryRenderRules struct {
	Frames map[uint16][]SceneryRenderDescriptor
}

// DecodeSceneryRenderRules retains the actual 12-byte sprite descriptors and
// image layers used by $eae4/$eb6a. Sprite indices are descriptor offsets/12.
func DecodeSceneryRenderRules(exe *amiga.Executable) (SceneryRenderRules, error) {
	var rules SceneryRenderRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x26956 {
		return rules, fmt.Errorf("native scenery rendering tables missing")
	}
	code := exe.Hunks[0].Data
	rules.Frames = make(map[uint16][]SceneryRenderDescriptor)
	for _, start := range []int{0x25c, 0x26c, 0xad8, 0xae0, 0xae8, 0xaf0, 0xaf8, 0xb00, 0xf10} {
		for index := 0; index < 256; index++ {
			offset := start + index*4
			at := 0x23d1a + offset
			if at+4 > len(code) {
				return SceneryRenderRules{}, fmt.Errorf("native scenery animation exceeds CODE")
			}
			image := int16(binary.BigEndian.Uint16(code[at:]))
			if image < 0 {
				break
			}
			layers, err := decodeImageLayers(code, uint16(image))
			if err != nil {
				return SceneryRenderRules{}, err
			}
			descriptors := []SceneryRenderDescriptor{}
			for _, layer := range layers {
				sprite := 0x21626 + layer.Sprite*12
				if sprite+8 > len(code) {
					return SceneryRenderRules{}, fmt.Errorf("native scenery sprite descriptor exceeds CODE")
				}
				half, height := int16(binary.BigEndian.Uint16(code[sprite+4:])), int16(binary.BigEndian.Uint16(code[sprite+6:]))
				if half != 8 && half != 16 || height <= 0 {
					return SceneryRenderRules{}, fmt.Errorf("unsupported native scenery sprite dimensions")
				}
				descriptors = append(descriptors, SceneryRenderDescriptor{Sprite: layer.Sprite, X: int16(layer.X), Y: int16(layer.Y), HalfWidth: half, Height: height})
			}
			rules.Frames[uint16(offset)] = descriptors
		}
	}
	return rules, nil
}

type SceneryRenderSlice struct {
	Sprite                                int
	X, Y, SourceX, SourceY, Width, Height int
	UnclippedX, UnclippedY, VisibleHeight int16
	SourceHeight, PlaneStride             int
}

// Plan returns native 320x200 logical pixel rectangles. X/Y are ordinary
// projected actor anchors. On doubled displays, scale every coordinate by 2.
// Nonzero age uses only the first image layer, adds 8 to the bottom anchor,
// moves its top downward by abs(age), and omits that many bottom source rows.
func (rules SceneryRenderRules) Plan(animation uint16, age int8, x, y int16) ([]SceneryRenderSlice, error) {
	descriptors, ok := rules.Frames[animation]
	if !ok || len(descriptors) == 0 {
		return nil, fmt.Errorf("native scenery render animation outside decoded bank")
	}
	if age != 0 {
		descriptors = descriptors[:1]
	}
	result := []SceneryRenderSlice{}
	for _, descriptor := range descriptors {
		height := descriptor.Height
		anchorY := y
		if age != 0 {
			amount := int16(age)
			if amount < 0 {
				amount = -amount
			}
			height -= amount
			anchorY = int16(uint16(anchorY) + 8)
			if height <= 0 {
				continue
			}
		}
		xLeft := int16(uint16(x) + uint16(descriptor.X) - uint16(descriptor.HalfWidth))
		yTop := int16(uint16(anchorY) + uint16(descriptor.Y) - uint16(height))
		slice := SceneryRenderSlice{Sprite: descriptor.Sprite, UnclippedX: xLeft, UnclippedY: yTop, VisibleHeight: height, SourceHeight: int(descriptor.Height), PlaneStride: int(descriptor.Height) * int(descriptor.HalfWidth) / 4}
		slice.SourceX = max(0, -int(xLeft))
		slice.SourceY = max(0, -int(yTop))
		slice.X = max(0, int(xLeft))
		slice.Y = max(0, int(yTop))
		slice.Width = min(int(descriptor.HalfWidth)*2-slice.SourceX, 320-slice.X)
		slice.Height = min(int(height)-slice.SourceY, 200-slice.Y)
		if slice.Width > 0 && slice.Height > 0 {
			result = append(result, slice)
		}
	}
	return result, nil
}
