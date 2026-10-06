package app

import (
	"image"
	"image/draw"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

type EnvironmentalDraw struct {
	Animation   string
	Frame, X, Y int
	Strike      bool
}

// stormDraws retains the original drawing order: its strike, a ground impact,
// then the cloud canopy. The canopy is held at least 75 pixels above ground.
func stormDraws(e engine.StormEffect, x, y int) []EnvironmentalDraw {
	if !e.Active {
		return nil
	}
	cloudY := max(0, y-75)
	plan := make([]EnvironmentalDraw, 0, 3)
	if e.Timer != 0 {
		plan = append(plan, EnvironmentalDraw{Animation: "storm/strike", Frame: e.Timer & 1, X: x, Y: cloudY, Strike: true})
	}
	if e.ImpactActive {
		name := "fire-impact/land"
		if e.WaterImpact {
			name = "fire-impact/water"
		}
		plan = append(plan, EnvironmentalDraw{Animation: name, Frame: e.ImpactFrame, X: x, Y: y + 8})
	}
	return append(plan, EnvironmentalDraw{Animation: "storm/cloud", Frame: e.Frame, X: x, Y: cloudY})
}

func lavaAnimation(shape uint8, direction int) string {
	if shape == 15 {
		return [4]string{"lava/north", "lava/east", "lava/south", "lava/west"}[direction&3]
	}
	switch shape {
	case 3:
		return "lava/slope-3"
	case 6:
		return "lava/slope-6"
	case 9:
		return "lava/slope-9"
	case 12:
		return "lava/slope-12"
	}
	return ""
}

func (g *Game) environmentAnchor(fixedX, fixedY int) (int, int) {
	return g.projectActor(fixedX, fixedY)
}

func (g *Game) drawStormAtCell(x, y, land int) {
	for _, e := range g.World.Air.Storms {
		if !e.Active || e.X/256 != x || e.Y/256 != y {
			continue
		}
		ax, ay := g.environmentAnchor(e.X, e.Y)
		for _, d := range stormDraws(e, ax, ay) {
			if d.Strike {
				g.drawStormStrike(d, ay, land)
			} else {
				g.animation(d.Animation, d.Frame, d.X, d.Y, land)
			}
		}
	}
}

func (g *Game) drawStormStrike(d EnvironmentalDraw, groundY, land int) {
	animation, ok := g.Assets.Visual.Animations[d.Animation]
	if !ok || len(animation.Frames) == 0 {
		return
	}
	frame := d.Frame % len(animation.Frames)
	g.playAnimationCue(d.Animation, frame, 0)
	for _, layer := range animation.Frames[frame].Layers {
		if layer.Sprite < 0 || layer.Sprite >= len(g.Assets.Visual.Sprites[land]) {
			continue
		}
		sprite := g.Assets.Visual.Sprites[land][layer.Sprite]
		if sprite.Image == nil {
			continue
		}
		left, top, height, visible := strikeLayerGeometry(layer, sprite, d.X, d.Y, groundY)
		if !visible {
			continue
		}
		img := g.Assets.Visual.StormStrikeImage(land, layer.Sprite, height)
		if img == nil {
			continue
		}
		draw.Draw(g.framebuffer, image.Rect(left, top, left+img.Bounds().Dx(), top+height), img, image.Point{}, draw.Over)
	}
}

func (g *Game) drawLavaAtCell(x, y, land int) {
	for _, e := range g.World.Fire.Lava {
		if !e.Active || e.X/256 != x || e.Y/256 != y {
			continue
		}
		name := lavaAnimation(g.World.Cell(x, y).Shape, e.Direction)
		if name == "" {
			continue
		}
		ax, ay := g.environmentAnchor(e.X, e.Y)
		g.animation(name, e.Frame, ax, ay+8, land)
	}
}

// Volcano is presented by its actual terrain tiles, fire columns and lava.
// Its unlinked controller has no additional independent sprite layer.
func volcanoHasIndependentSprite() bool { return false }

func strikeLayerGeometry(layer visualassets.SpriteLayer, sprite visualassets.Sprite, x, y, groundY int) (left, top, height int, visible bool) {
	left, top = x+layer.X-sprite.AnchorX, y+layer.Y-sprite.AnchorY
	if sprite.Image == nil {
		return left, top, 0, false
	}
	height = sprite.Image.Bounds().Dy()
	if overflow := top + height - groundY; overflow > 0 {
		if height < overflow {
			return left, top, 0, false
		}
		height = overflow
	}
	return left, top, height, height > 0
}
