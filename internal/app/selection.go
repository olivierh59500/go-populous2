package app

import (
	"image"
	"image/draw"

	"go-populous2/internal/engine"
)

func (g *Game) refreshSelectedFollower() {
	g.SelectedFollower = g.SelectionReturn.checked(g.SelectedFollower, func(id int) bool {
		return selectedFollowerExists(g.World, id)
	})
}

// pickFollower follows the original anchor-based inspect hit test. The first
// rendered eligible actor consumes the click; map cells alone are insufficient.
func (g *Game) pickFollower(x, y int) int {
	if g.World == nil || x < 104 || y < 45 || y >= 178 {
		return 0
	}
	var commands [viewSize*viewSize + engine.FollowerCapacity + engine.EffectCapacity + engine.SceneryCapacity + engine.WallCapacity + 2]RenderCommand
	for _, command := range sourceRenderPlan(g.World, g.CameraX, g.CameraY, commands[:0]) {
		if command.Actor.Kind != engine.ActorFollower {
			continue
		}
		id := int(command.Actor.Index)
		f := g.World.Followers[id]
		if f.State == engine.Inactive {
			continue
		}
		ax, ay := g.followerRenderAnchor(f)
		width, height := 8, 18
		if f.State == engine.Town || f.BattleWasTown {
			width = 16
			height = g.townInspectHeight(f, g.World.Level.Landscape)
		} else if f.State != engine.Walking || f.ContactWaiting {
			continue
		}
		if absInt(x-ax) <= width && y <= ay && ay-y <= height {
			return id
		}
	}
	return 0
}

func (g *Game) townInspectHeight(f engine.Follower, land int) int {
	if panel := g.Assets.SelectionPanel; panel != nil {
		return panel.TownHitHeight(int(f.Stage))
	}
	art := g.Assets.Visual.Towns
	if art == nil {
		return 18
	}
	height := 18
	for _, layer := range art.CenterLayers(int(f.Stage), int(f.Owner), uint32(f.Population), g.World.Tick) {
		if layer.Sprite == art.FlagSprites[0][0] || layer.Sprite == art.FlagSprites[0][1] || layer.Sprite == art.FlagSprites[1][0] || layer.Sprite == art.FlagSprites[1][1] {
			continue
		}
		if layer.Sprite >= 0 && layer.Sprite < len(g.Assets.Visual.Sprites[land]) {
			s := g.Assets.Visual.Sprites[land][layer.Sprite]
			if s.Image != nil {
				height = s.Image.Bounds().Dy()
			}
		}
	}
	return max(0, height)
}

func (g *Game) handleSelectionPanelClick(x, y int, clicked bool) bool {
	panel := g.Assets.SelectionPanel
	if !clicked || panel == nil || !image.Pt(x, y).In(panel.HitRect()) {
		return false
	}
	g.refreshSelectedFollower()
	if id := g.SelectedFollower; id > 0 {
		f := g.World.Followers[id]
		g.CameraX, g.CameraY = max(0, min(56, int(f.X)-4)), max(0, min(56, int(f.Y)-4))
	}
	return true
}

func (g *Game) drawSelectionPanel() {
	panel := g.Assets.SelectionPanel
	if panel == nil || g.World == nil {
		return
	}
	g.refreshSelectedFollower()
	id := g.SelectedFollower
	if id == 0 {
		return
	}
	f := g.World.Followers[id]
	land := g.World.Level.Landscape
	// The selected copy hides its inspect cursor but retains leader/hero status
	// arrows. A local renderer copy prevents a second sound trigger.
	preview := *g
	preview.music = nil
	preview.drawSelectedFollowerArtwork(id, land, panel.ActorX, panel.ActorY)
	if f.Weapons >= 0 && f.Weapons < len(panel.Weapons) {
		preview.drawArtworkLayers(panel.Weapons[f.Weapons].Layers, panel.WeaponX, panel.WeaponY, land)
	}
	if panel.PopulationSprite < 0 || panel.PopulationSprite >= len(g.Assets.Visual.Sprites[land]) {
		return
	}
	sprite := g.Assets.Visual.Sprites[land][panel.PopulationSprite].Image
	if sprite == nil {
		return
	}
	for _, point := range panel.PopulationIndicators(uint32(f.Population)) {
		draw.Draw(g.framebuffer, image.Rect(point.X, point.Y, point.X+sprite.Bounds().Dx(), point.Y+sprite.Bounds().Dy()), sprite, image.Point{}, draw.Over)
	}
}
