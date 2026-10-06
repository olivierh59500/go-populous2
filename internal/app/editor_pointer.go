package app

import (
	"image"
	"image/draw"
)

func (g *Game) drawEditorPointer(x, y int) bool {
	if g.Screen != EditorScreen || g.Editor == nil || g.Assets == nil {
		return false
	}
	tool := map[EditorTool]string{EditorBlue: "blue", EditorRed: "red", EditorTree: "tree", EditorRock: "rock"}[g.Editor.Tool]
	if frame, ok := g.Assets.EditorPreview[tool]; ok {
		land := g.Editor.Draft.Level.Landscape
		g.drawArtworkLayers(frame.Layers, x, y, land)
	}
	sprite, ok := pointerFrame(g.Assets.Pointers, "normal", 0)
	if !ok || sprite.Image == nil {
		return false
	}
	draw.Draw(g.framebuffer, image.Rect(x-sprite.AnchorX, y-sprite.AnchorY, x-sprite.AnchorX+16, y-sprite.AnchorY+16), sprite.Image, image.Point{}, draw.Over)
	return true
}
