package app

import (
	"image"
	"image/draw"

	"go-populous2/internal/engine"
)

type terrainBackdropStrip struct {
	Destination image.Rectangle
	Source      image.Point
}

// terrainBackdropStrips restores the scenery behind the raised left/right
// map edges. The original copies 64-pixel strips ending at destination row134,
// from the background's corresponding row166, before drawing terrain.
func terrainBackdropStrips(w *engine.World, cameraX, cameraY int) []terrainBackdropStrip {
	if w == nil {
		return nil
	}
	var strips [2]terrainBackdropStrip
	count := 0
	for side, corner := range [2][2]int{{cameraX, cameraY + viewSize}, {cameraX + viewSize, cameraY}} {
		x, y := corner[0], corner[1]
		if x < 0 || y < 0 || x >= engine.CornerSize || y >= engine.CornerSize {
			continue
		}
		rows := int(w.Heights[x+y*engine.CornerSize])*8 - 1
		if rows <= 0 {
			continue
		}
		destinationX, sourceX := 64, 128
		if side == 1 {
			destinationX, sourceX = 256, 192
		}
		strips[count] = terrainBackdropStrip{Destination: image.Rect(destinationX, 135-rows, destinationX+64, 135), Source: image.Pt(sourceX, 167-rows)}
		count++
	}
	return strips[:count]
}

func (g *Game) drawTerrainBackdrop() {
	background := g.Assets.Visual.Background
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), background, image.Point{}, draw.Src)
	for _, strip := range terrainBackdropStrips(g.World, g.CameraX, g.CameraY) {
		draw.Draw(g.framebuffer, strip.Destination, background, strip.Source, draw.Src)
	}
}
