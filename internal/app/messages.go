package app

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
)

// drawMessage wraps useful error details to the original eight-pixel font.
func (g *Game) drawMessage(y int) {
	draw.Draw(g.framebuffer, image.Rect(0, y, 320, 200), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	text := strings.ToUpper(g.Message)
	for line := 0; len(text) > 0 && y+line*9 < 193; line++ {
		end := min(38, len(text))
		if end < len(text) {
			if space := strings.LastIndexByte(text[:end], ' '); space > 0 {
				end = space
			}
		}
		g.text(text[:end], 8, y+line*9)
		text = strings.TrimSpace(text[end:])
	}
}
