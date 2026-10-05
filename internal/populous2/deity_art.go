package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
	"image"
	"image/color"
	"image/draw"
)

type DeityArt struct{ Parts [3][8]*image.RGBA }

// DecodeDeityArt reads the FACES.PAK descriptors at CODE:$212ba. The creation
// routine $baee reads the profile backward to draw mouth, eyes and headpiece;
// the three descriptor banks are headpiece, eyes and mouth.
func DecodeDeityArt(exe *amiga.Executable, faces []byte, palette [16]color.RGBA) (*DeityArt, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x212ba+24*12 {
		return nil, fmt.Errorf("native deity image descriptors missing")
	}
	code := exe.Hunks[0].Data
	art := &DeityArt{}
	for part := range art.Parts {
		for variant := range art.Parts[part] {
			at := 0x212ba + (part*8+variant)*12
			offset := int(binary.BigEndian.Uint32(code[at:])) - 0xa6cc
			width := int(binary.BigEndian.Uint16(code[at+4:])) * 2
			height := int(binary.BigEndian.Uint16(code[at+6:]))
			length := width / 8 * 5 * height
			if width != 32 || height < 1 || height > 64 || offset < 0 || offset+length > len(faces) {
				return nil, fmt.Errorf("invalid deity part %d variant %d", part, variant)
			}
			prepared, err := PrepareNativeMaskedPlanes(faces[offset:offset+length], width, height)
			if err != nil {
				return nil, err
			}
			img, err := DecodeNativeMaskedPlanes(prepared, width, height, palette)
			if err != nil {
				return nil, err
			}
			art.Parts[part][variant] = img
		}
	}
	return art, nil
}

// Portrait uses the creation-screen strip positions from CODE:$b72e. Mouth,
// eyes and headpiece are drawn in the original reverse bank order; shorter
// strips are bottom-aligned to their sixteen-pixel band as in $bb0e.
// Opaque mask bits from the contiguous FACES planes retain their alpha.
func (art *DeityArt) Portrait(parts [3]uint8) (*image.RGBA, error) {
	if art == nil {
		return nil, fmt.Errorf("missing deity artwork")
	}
	portrait := image.NewRGBA(image.Rect(0, 0, 32, 96))
	for part := len(parts) - 1; part >= 0; part-- {
		if parts[part] > 7 {
			return nil, fmt.Errorf("invalid deity face variant")
		}
		img := art.Parts[part][parts[part]]
		y := part * 16
		if img.Bounds().Dy() < 16 {
			y += 16 - img.Bounds().Dy()
		}
		draw.Draw(portrait, image.Rect(0, y, 32, y+img.Bounds().Dy()), img, image.Point{}, draw.Over)
	}
	return portrait, nil
}
