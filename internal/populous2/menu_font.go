package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"go-populous2/internal/amiga"
)

const (
	NativeMenuWidth  = 320
	NativeMenuHeight = 200
	NativeGlyphSize  = 8
	NativeGlyphFirst = 32
	NativeGlyphCount = 96
)

// NativeMenuFont retains color indices from the original eight-row glyphs.
// Lowercase codes include interface symbols and must not be uppercased.
type NativeMenuFont struct {
	Glyphs [NativeGlyphCount][NativeGlyphSize * NativeGlyphSize]uint8
}

// DecodeNativeMenuFont follows $509a: each glyph has eight rows of four
// plane bytes at CODE:$33c68. These 96 byte codes are the verified printable
// range; the conversion does not interpret arbitrary data after that bank.
func DecodeNativeMenuFont(exe *amiga.Executable) (*NativeMenuFont, error) {
	const start = 0x33c68
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < start+NativeGlyphCount*32 {
		return nil, fmt.Errorf("native menu font missing")
	}
	code := exe.Hunks[0].Data
	if binary.BigEndian.Uint16(code[0x50ac:]) != 0x47f9 || binary.BigEndian.Uint32(code[0x50ae:]) != start {
		return nil, fmt.Errorf("unsupported native menu font descriptor")
	}
	font := &NativeMenuFont{}
	for glyph := range font.Glyphs {
		for y := range NativeGlyphSize {
			for x := range NativeGlyphSize {
				for plane := range 4 {
					bit := code[start+glyph*32+y*4+plane] >> uint(7-x) & 1
					font.Glyphs[glyph][x+y*NativeGlyphSize] |= bit << uint(plane)
				}
			}
		}
	}
	return font, nil
}

// DrawIndices translates $509a into a 320x200 color-index buffer. Column is
// measured in eight-pixel cells; row is measured in pixels. Newlines restore
// the initial column and advance eight rows. A full line stops on the next
// ordinary byte, as in the original, while a newline can still wrap it.
func (font *NativeMenuFont) DrawIndices(pixels []uint8, text string, column, row int) error {
	if font == nil || len(pixels) != NativeMenuWidth*NativeMenuHeight || column < 0 || column >= NativeMenuWidth/NativeGlyphSize || row < 0 || row+NativeGlyphSize > NativeMenuHeight {
		return fmt.Errorf("invalid native text buffer/position")
	}
	x, y := column, row
	for index := range len(text) {
		code := text[index]
		if code == 0 {
			return nil
		}
		if code == '\n' {
			x, y = column, y+NativeGlyphSize
			if y+NativeGlyphSize > NativeMenuHeight {
				return fmt.Errorf("native text exceeds the display height")
			}
			continue
		}
		if x >= NativeMenuWidth/NativeGlyphSize {
			return nil
		}
		if code < NativeGlyphFirst || int(code) >= NativeGlyphFirst+NativeGlyphCount {
			return fmt.Errorf("unverified native glyph code %d", code)
		}
		glyph := &font.Glyphs[int(code)-NativeGlyphFirst]
		for dy := range NativeGlyphSize {
			copy(pixels[x*NativeGlyphSize+(y+dy)*NativeMenuWidth:], glyph[dy*NativeGlyphSize:(dy+1)*NativeGlyphSize])
		}
		x++
	}
	return nil
}

// Atlas exposes the original glyph colors for resource inspection. Color
// index zero remains opaque, matching the native plane-byte replacement.
func (font *NativeMenuFont) Atlas(palette [16]color.RGBA) *image.Paletted {
	colors := make(color.Palette, len(palette))
	for index, entry := range palette {
		colors[index] = entry
	}
	atlas := image.NewPaletted(image.Rect(0, 0, 16*NativeGlyphSize, 6*NativeGlyphSize), colors)
	if font == nil {
		return atlas
	}
	for index, glyph := range font.Glyphs {
		x, y := index%16*NativeGlyphSize, index/16*NativeGlyphSize
		for row := range NativeGlyphSize {
			copy(atlas.Pix[(y+row)*atlas.Stride+x:], glyph[row*NativeGlyphSize:(row+1)*NativeGlyphSize])
		}
	}
	return atlas
}
