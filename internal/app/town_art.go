package app

import (
	"image"
	"image/draw"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func (g *Game) drawArtworkLayers(layers []visualassets.SpriteLayer, x, y, land int) {
	for _, layer := range layers {
		if layer.Sprite < 0 || layer.Sprite >= len(g.Assets.Visual.Sprites[land]) {
			continue
		}
		sprite := g.Assets.Visual.Sprites[land][layer.Sprite]
		if sprite.Image == nil {
			continue
		}
		left, top := x+layer.X-sprite.AnchorX, y+layer.Y-sprite.AnchorY
		draw.Draw(g.framebuffer, image.Rect(left, top, left+sprite.Image.Bounds().Dx(), top+sprite.Image.Bounds().Dy()), sprite.Image, image.Point{}, draw.Over)
	}
}

func (g *Game) drawTownCenter(f engine.Follower, x, y, land int) {
	art := g.Assets.Visual.Towns
	if art == nil {
		return
	}
	layers := art.CenterLayers(int(f.Stage), int(f.Owner), uint32(f.Population), g.World.Tick)
	g.drawArtworkLayers(layers, x, y, land)
}

// drawTownSurroundingsAt belongs inside the terrain painter's current parcel.
// Each adjacent structure uses that parcel's altitude and is drawn before its
// actors, including when its town center lies just outside the visible view.
func (g *Game) drawTownSurroundingsAt(x, y, land int) {
	art := g.Assets.Visual.Towns
	if art == nil {
		return
	}
	selectedID := -1
	var selected visualassets.Frame
	for index, offset := range art.Offsets {
		cx, cy := x-offset[0], y-offset[1]
		if cx < 0 || cy < 0 || cx >= engine.MapSize || cy >= engine.MapSize {
			continue
		}
		for id, visits := int(g.World.Occupants[cx+cy*engine.MapSize]), 0; id > 0 && id < engine.FollowerCapacity && visits < engine.FollowerCapacity; visits++ {
			followerID := id
			f := g.World.Followers[id]
			id = f.NextFollower
			if f.State != engine.Town || int(f.Stage) >= len(art.Surroundings) {
				continue
			}
			if int(f.X)+offset[0] != x || int(f.Y)+offset[1] != y {
				continue
			}
			frame := art.Surroundings[f.Stage][index]
			if len(frame.Layers) == 0 {
				continue
			}
			// Settlement evaluation visits followers in ascending slot order;
			// the later center wins a shared overlay parcel.
			if followerID > selectedID {
				selectedID = followerID
				selected = frame
			}
		}
	}
	if selectedID >= 0 {
		cell := g.World.Cell(x, y)
		px := 192 + 16*(x-g.CameraX-y+g.CameraY)
		py := 72 + 8*(x-g.CameraX+y-g.CameraY) - int(cell.BaseAltitude)*8
		g.drawArtworkLayers(selected.Layers, px, py, land)
	}
}
