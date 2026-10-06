package app

import (
	"image"
	"image/color"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func TestTownSurroundingsUseOwnParcelAndOffscreenCenter(t *testing.T) {
	world := &engine.World{}
	world.Followers[1] = engine.Follower{Owner: 0, X: 9, Y: 10, State: engine.Town, Stage: 18}
	world.Occupants[9+10*engine.MapSize] = 1
	world.Tiles[10+10*engine.MapSize].BaseAltitude = 1
	art := &visualassets.TownArt{}
	art.Offsets[0] = [2]int{1, 0}
	art.Surroundings[18][0] = visualassets.Frame{Layers: []visualassets.SpriteLayer{{Sprite: 0}}}
	pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
	pixel.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	visual := &visualassets.Bundle{Towns: art}
	visual.Sprites[0] = []visualassets.Sprite{{Image: pixel}}
	g := &Game{World: world, CameraX: 10, CameraY: 10, Assets: &Assets{Visual: visual}, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.drawTownSurroundingsAt(10, 10, 0)
	if g.framebuffer.RGBAAt(192, 64).R != 255 {
		t.Fatal("adjacent town art ignored its own parcel altitude or invisible center")
	}
}

func TestTownCenterUsesPopulationAndFactionFlagFrames(t *testing.T) {
	art := &visualassets.TownArt{FlagSprites: [2][2]int{{0, 1}, {2, 3}}, FlagHeight: 24}
	art.PopulationDivisors[18] = 10
	art.Centers[18] = visualassets.Frame{Layers: []visualassets.SpriteLayer{{Sprite: 0, Y: -24}}}
	visual := &visualassets.Bundle{Towns: art}
	for i := range 4 {
		pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
		pixel.SetRGBA(0, 0, color.RGBA{R: uint8(40 + i), A: 255})
		visual.Sprites[0] = append(visual.Sprites[0], visualassets.Sprite{Image: pixel})
	}
	g := &Game{World: &engine.World{Tick: 1}, Assets: &Assets{Visual: visual}, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.drawTownCenter(engine.Follower{Owner: 1, Stage: 18, Population: 50}, 100, 100, 0)
	if got := g.framebuffer.RGBAAt(100, 95); got.R != 43 {
		t.Fatal("town flag population/owner/phase selection differs", got)
	}
}
