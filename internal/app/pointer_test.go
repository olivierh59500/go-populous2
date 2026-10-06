package app

import (
	"image"
	"image/color"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func TestPointerSelectsInterfaceTerrainRightsAndPowerFrames(t *testing.T) {
	if pointerSequence(engine.RaiseLower, false, true, false) != "forbidden" || pointerSequence(engine.RaiseLower, false, true, true) != "normal" || pointerSequence(engine.Volcano, true, true, true) != "interface" {
		t.Fatal("pointer zone/terrain state differs")
	}
	art := &visualassets.PointerArt{Frames: map[string][]visualassets.Sprite{"power/26": make([]visualassets.Sprite, 4)}}
	for phase := 0; phase < 4; phase++ {
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))
		img.SetRGBA(0, 0, color.RGBA{R: uint8(phase + 1), A: 255})
		art.Frames["power/26"][phase] = visualassets.Sprite{Image: img}
	}
	for phase := 0; phase < 4; phase++ {
		sprite, ok := pointerFrame(art, pointerSequence(engine.Volcano, false, true, true), uint64(phase*8))
		if !ok || sprite.Image.RGBAAt(0, 0).R != uint8(phase+1) {
			t.Fatal("pointer phase did not follow simulation clock")
		}
	}
}

func TestGamePointerUsesLogicalMousePositionAndSharedTerrainAdmission(t *testing.T) {
	g := menuTestGame(t)
	g.Screen = Playing
	g.CameraX, g.CameraY = 28, 28
	g.Selected = engine.RaiseLower
	visual := &visualassets.Bundle{}
	art := &visualassets.PointerArt{Frames: map[string][]visualassets.Sprite{}}
	for name, c := range map[string]color.RGBA{"normal": {G: 255, A: 255}, "forbidden": {R: 255, A: 255}, "interface": {B: 255, A: 255}} {
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))
		img.SetRGBA(0, 0, c)
		art.Frames[name] = []visualassets.Sprite{{Image: img}}
	}
	g.Assets.Visual, g.Assets.Pointers = visual, art
	g.framebuffer = image.NewRGBA(image.Rect(0, 0, 320, 200))
	x, y := g.projectCorner(32, 32)
	if !g.drawGamePointer(x, y) {
		t.Fatal("loaded pointer was not drawn")
	}
	selectedX, selectedY, ok := g.pickCorner(x, y)
	if !ok {
		t.Fatal("fixture did not pick its terrain vertex")
	}
	rights := g.World.CursorTerrainRights(0, engine.Viewport{X: 28, Y: 28, Size: 8})
	allowed := rights.BuildAnywhere || !rights.SeaLevelOnly || g.World.Heights[selectedX+selectedY*engine.CornerSize] > 0
	expected := color.RGBA{R: 255, A: 255}
	if allowed {
		expected = color.RGBA{G: 255, A: 255}
	}
	if got := g.framebuffer.RGBAAt(x, y); got != expected {
		t.Fatal("pointer did not use exact input terrain gate", got, expected)
	}
	if !g.drawGamePointer(7, 99) || g.framebuffer.RGBAAt(7, 99) != (color.RGBA{B: 255, A: 255}) {
		t.Fatal("logical interface pointer placement differs")
	}
	g.Screen = MainMenu
	if g.drawGamePointer(7, 99) {
		t.Fatal("game pointer leaked into menu")
	}
}
