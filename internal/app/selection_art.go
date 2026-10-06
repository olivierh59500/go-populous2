package app

import (
	"fmt"
	"image"
	"image/draw"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

// drawSelectedFollowerArtwork reuses the live actor phase. The amphitheatre
// does not run an independent animation clock or change simulation state.
func (g *Game) drawSelectedFollowerArtwork(id, land, x, y int) {
	f := g.World.Followers[id]
	if selectedTownCenter(g.World, id) {
		art := g.Assets.Visual.Towns
		if art == nil {
			return
		}
		lastTop := y
		for _, layer := range art.CenterLayers(int(f.Stage), int(f.Owner), uint32(f.Population), g.World.Tick) {
			flag := false
			for _, frames := range art.FlagSprites {
				for _, sprite := range frames {
					flag = flag || layer.Sprite == sprite
				}
			}
			if flag {
				// The status arrow precedes the flag and uses the preceding
				// building layer's top, rather than the flag's population height.
				g.drawFollowerStatusIndicator(id, x+layer.X, lastTop, land)
			}
			g.drawArtworkLayers([]visualassets.SpriteLayer{layer}, x, y, land)
			if !flag && layer.Sprite >= 0 && layer.Sprite < len(g.Assets.Visual.Sprites[land]) {
				lastTop = y + layer.Y - g.Assets.Visual.Sprites[land][layer.Sprite].AnchorY
			}
		}
		return
	}
	g.drawFollowerArtwork(id, land, x, y, false)
	if f.State != engine.Walking || f.CombatAftermath.Kind != engine.CombatAftermathNone || f.TerrainDeath.Active || f.Neutral.Kind != engine.NeutralNone || f.Neutral.VictimTime != 0 || f.Conversion.Active || g.World.Air.Carry[id].Phase != engine.AirCarryNone || g.World.AirVictims[id].Phase != engine.LightningVictimNone || g.World.FireDamage.Deaths[id].Mode != engine.FireVictimAlive || g.World.Nature.Deaths[id] != engine.NatureAlive {
		return
	}
	name := fmt.Sprintf("follower/%d/%d/%s", f.Owner, f.AppearanceVariant, compassNames[f.Direction&7])
	if f.IsHero() {
		name = fmt.Sprintf("hero/%s/%s", heroNames[f.Hero.Kind], compassNames[f.Direction&7])
	}
	if f.ContactWaiting {
		name = "contact/waiting"
		if f.IsHero() {
			name = "contact/" + heroNames[f.Hero.Kind]
		}
	}
	animation, ok := g.Assets.Visual.Animations[name]
	if !ok || len(animation.Frames) == 0 {
		return
	}
	frame := int(f.Frame)
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
	if last.Sprite >= 0 && last.Sprite < len(g.Assets.Visual.Sprites[land]) {
		g.drawFollowerStatusIndicator(id, x, y+last.Y-g.Assets.Visual.Sprites[land][last.Sprite].AnchorY, land)
	}
}

func selectedTownCenter(w *engine.World, id int) bool {
	f := w.Followers[id]
	return (f.State == engine.Town || f.State == engine.Fighting && f.BattleWasTown && !f.BattleAggressor) &&
		f.CombatAftermath.Kind == engine.CombatAftermathNone && !f.TerrainDeath.Active &&
		!f.Conversion.Active && w.Air.Carry[id].Phase == engine.AirCarryNone &&
		w.AirVictims[id].Phase == engine.LightningVictimNone && w.FireDamage.Deaths[id].Mode == engine.FireVictimAlive
}

// drawFollowerStatusIndicator draws the direct faction pointer. Both leader
// and hero flags use this same image in the source interface, including the
// selected panel; the separate inspect cursor is intentionally absent here.
func (g *Game) drawFollowerStatusIndicator(id, x, lastTop, land int) {
	f := g.World.Followers[id]
	if f.Owner > 1 || !f.IsHero() && g.World.Players[f.Owner].Leader != id {
		return
	}
	name := "marker/leader-blue"
	if f.Owner != 0 {
		name = "marker/leader-red"
	}
	animation := g.Assets.Visual.Animations[name]
	if len(animation.Frames) == 0 || len(animation.Frames[0].Layers) == 0 {
		return
	}
	spriteID := animation.Frames[0].Layers[0].Sprite
	if spriteID < 0 || spriteID >= len(g.Assets.Visual.Sprites[land]) {
		return
	}
	img := g.Assets.Visual.Sprites[land][spriteID].Image
	if img == nil {
		return
	}
	left, top := x-4, lastTop-8
	draw.Draw(g.framebuffer, image.Rect(left, top, left+img.Bounds().Dx(), top+img.Bounds().Dy()), img, image.Point{}, draw.Over)
}
