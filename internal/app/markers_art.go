package app

import (
	"fmt"
	"go-populous2/internal/engine"
	"image"
	"image/draw"
)

func activeBattleArtwork(f engine.Follower) (string, int, bool) {
	if f.State != engine.Fighting {
		return "", 0, false
	}
	if f.BattleAggressor {
		return "combat/attack", int(f.Frame) % 2, true
	}
	return "", 0, true
}

// drawActiveBattle draws the shared aggressor composition, including heroes.
// Ordinary defenders have no separate source sprite; defending towns retain
// their town body while the aggressor composition covers the local battle.
func (g *Game) drawActiveBattle(id, x, y, land int) bool {
	f := g.World.Followers[id]
	name, frame, handled := activeBattleArtwork(f)
	if !handled {
		return false
	}
	if name != "" {
		g.animation(name, frame, x, y, land)
	} else if f.BattleWasTown {
		g.drawTownCenter(f, x, y, land)
	}
	return true
}

func (g *Game) drawMagnetAtCell(x, y, land int) {
	for owner, p := range g.World.Players {
		if p.RallyX != x || p.RallyY != y {
			continue
		}
		name := fmt.Sprintf("magnet/%d", owner)
		animation, ok := g.Assets.Visual.Animations[name]
		if !ok || len(animation.Frames) == 0 {
			continue
		}
		ax, ay := g.projectCorner(x, y)
		g.animation(name, int(g.World.Tick)%len(animation.Frames), ax, ay+8, land)
	}
}

// drawLeaderMarker uses the faction's original pointer art above the group's
// actor anchor. Selection and hero indicators remain independent overlays.
func (g *Game) drawLeaderMarker(id, x, y, land int, animationName string, frame int) {
	f := g.World.Followers[id]
	if f.Owner > 1 || g.World.Players[f.Owner].Leader != id || f.State == engine.Inactive {
		return
	}
	name := "marker/leader-blue"
	if f.Owner == 1 {
		name = "marker/leader-red"
	}
	animation, ok := g.Assets.Visual.Animations[animationName]
	if !ok || len(animation.Frames) == 0 {
		return
	}
	if animation.Loop {
		frame %= len(animation.Frames)
	} else {
		frame = min(frame, len(animation.Frames)-1)
	}
	layers := animation.Frames[frame].Layers
	if len(layers) == 0 {
		return
	}
	last := layers[len(layers)-1]
	if last.Sprite < 0 || last.Sprite >= len(g.Assets.Visual.Sprites[land]) {
		return
	}
	lastY := y + last.Y - g.Assets.Visual.Sprites[land][last.Sprite].AnchorY
	marker := g.Assets.Visual.Animations[name]
	if len(marker.Frames) == 0 || len(marker.Frames[0].Layers) == 0 {
		return
	}
	spriteID := marker.Frames[0].Layers[0].Sprite
	if spriteID < 0 || spriteID >= len(g.Assets.Visual.Sprites[land]) {
		return
	}
	img := g.Assets.Visual.Sprites[land][spriteID].Image
	if img == nil {
		return
	}
	left, top := x-4, lastY-8
	draw.Draw(g.framebuffer, image.Rect(left, top, left+img.Bounds().Dx(), top+img.Bounds().Dy()), img, image.Point{}, draw.Over)
}
