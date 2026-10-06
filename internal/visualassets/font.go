package visualassets

import (
	"image"
	"image/color"
)

// Font retains palette indices, allowing the same glyph artwork to use the
// current menu palette. It includes interface symbols as well as letters.
type Font struct {
	FirstCode, Width, Height int
	Glyphs                   [][]uint8
}

// Draw writes complete glyph cells, including their background pixels.
// Newlines return to the starting X coordinate; drawing clips to the image.
func (f *Font) Draw(dst *image.RGBA, text string, x, y int, palette [16]color.RGBA) {
	if f == nil || dst == nil {
		return
	}
	origin := x
	for _, code := range []byte(text) {
		if code == 0 {
			return
		}
		if code == '\n' {
			x = origin
			y += f.Height
			continue
		}
		index := int(code) - f.FirstCode
		if index >= 0 && index < len(f.Glyphs) {
			glyph := f.Glyphs[index]
			for row := range f.Height {
				for col := range f.Width {
					pixel := image.Pt(x+col, y+row)
					if pixel.In(dst.Bounds()) {
						dst.SetRGBA(pixel.X, pixel.Y, palette[glyph[row*f.Width+col]])
					}
				}
			}
		}
		x += f.Width
	}
}
