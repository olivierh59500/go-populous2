package app

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/hajimehoshi/ebiten/v2"
	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func pointerSequence(power engine.PowerID, interfaceZone, terrainSelected, terrainAllowed bool) string {
	if interfaceZone {
		return "interface"
	}
	if power == engine.RaiseLower {
		if terrainSelected && !terrainAllowed {
			return "forbidden"
		}
		return "normal"
	}
	return fmt.Sprintf("power/%d", power)
}

func pointerFrame(art *visualassets.PointerArt, name string, tick uint64) (visualassets.Sprite, bool) {
	if art == nil {
		return visualassets.Sprite{}, false
	}
	frames := art.Frames[name]
	if len(frames) == 0 {
		frames = art.Frames["normal"]
	}
	if len(frames) == 0 {
		return visualassets.Sprite{}, false
	}
	return frames[int((tick&24)>>3)%len(frames)], true
}

// drawGamePointer presents the original attached mouse sprite at its logical
// top-left coordinates. The display gate uses visible terrain rights; actual
// sculpting retains its separate mana and scenario admission checks.
func (g *Game) drawGamePointer(mouseX, mouseY int) bool {
	view := g.displayedGame()
	return view.drawDisplayedPointer(mouseX, mouseY)
}

func (g *Game) drawDisplayedPointer(mouseX, mouseY int) bool {
	if g.Assets == nil || g.Assets.Pointers == nil || g.World == nil || g.Screen != Playing {
		return false
	}
	selectedX, selectedY, selected := g.pickDisplayedCorner(mouseX, mouseY)
	allowed := true
	if selected && g.Selected == engine.RaiseLower {
		rights := g.World.CursorTerrainRights(g.playerSide(), engine.Viewport{X: g.CameraX, Y: g.CameraY, Size: viewSize})
		allowed = rights.BuildAnywhere || !rights.SeaLevelOnly || g.World.Heights[selectedX+selectedY*engine.CornerSize] > 0
	}
	interfaceZone := g.PickingPower || mouseX < 104 || mouseY < 45 || mouseY > 177
	if g.Assets.HUD != nil {
		kind, index := visualassets.HUDHit(mouseX, mouseY)
		if kind == "category" || kind == "power" || (kind == "control" && g.Assets.HUD.Controls[index] != "") {
			interfaceZone = true
		}
	}
	if g.Assets.SelectionPanel != nil && image.Pt(mouseX, mouseY).In(g.Assets.SelectionPanel.HitRect()) {
		interfaceZone = true
	}
	name := pointerSequence(g.Selected, interfaceZone, selected, allowed)
	if g.Inspecting {
		name = "inspect"
	}
	if selected && !interfaceZone && !g.Inspecting {
		g.drawTerrainPointer(selectedX, selectedY)
	}
	sprite, ok := pointerFrame(g.Assets.Pointers, name, g.World.Tick)
	if !ok || sprite.Image == nil {
		return false
	}
	left, top := mouseX-sprite.AnchorX, mouseY-sprite.AnchorY
	draw.Draw(g.framebuffer, image.Rect(left, top, left+16, top+16), sprite.Image, image.Point{}, draw.Over)
	return true
}

func (g *Game) updateSystemPointerVisibility() {
	available := g.Assets != nil && g.Assets.Pointers != nil
	if available {
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	}
}

// Original requesters normally inherit the normal hardware mouse sprite.
// Direct spell help can retain the active game power cursor instead.
func (g *Game) drawRequesterPointer(x, y int) bool {
	if g.Screen == Playing || g.Screen == EditorScreen || g.Assets == nil || g.Assets.Pointers == nil {
		return false
	}
	name, tick := "normal", uint64(0)
	if g.Screen == PowerHelpScreen && g.PowerPreview != nil && g.PowerPreview.Return == Playing && g.World != nil {
		name = pointerSequence(g.Selected, false, false, true)
		tick = g.World.Tick
	}
	sprite, ok := pointerFrame(g.Assets.Pointers, name, tick)
	if !ok || sprite.Image == nil {
		return false
	}
	left, top := x-sprite.AnchorX, y-sprite.AnchorY
	draw.Draw(g.framebuffer, image.Rect(left, top, left+16, top+16), sprite.Image, image.Point{}, draw.Over)
	return true
}

func (g *Game) drawTerrainPointer(x, y int) {
	art := g.Assets.Pointers
	if art == nil || art.MapMarkerSprite >= len(g.Assets.Visual.Sprites[g.World.Level.Landscape]) {
		return
	}
	sx, sy := g.projectCorner(x, y)
	marker := g.Assets.Visual.Sprites[g.World.Level.Landscape][art.MapMarkerSprite].Image
	if marker != nil {
		draw.Draw(g.framebuffer, image.Rect((sx&0x1f0)-3, sy+1, (sx&0x1f0)-3+marker.Bounds().Dx(), sy+1+marker.Bounds().Dy()), marker, image.Point{}, draw.Over)
	}
	cellX, cellY := min(x, engine.MapSize-1), min(y, engine.MapSize-1)
	shape := g.Assets.Visual.TileRasters[g.World.Cell(cellX, cellY).Code] & 15
	vertices := []int{0, 1, 2, 3}
	if y-g.CameraY >= viewSize {
		vertices = []int{0, 1}
	}
	if x-g.CameraX >= viewSize {
		vertices = []int{0, 3}
		if y-g.CameraY >= viewSize {
			vertices = []int{0}
		}
	}
	for _, vertex := range vertices {
		offset := art.MapOutlines[shape][vertex]
		g.framebuffer.SetRGBA(min(319, (sx&0x1f0)+offset[0]), sy+3+offset[1], g.Assets.Visual.Palettes[g.World.Level.Landscape][5])
	}
}
