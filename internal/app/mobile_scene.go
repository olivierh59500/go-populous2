package app

import (
	"image"
	"image/draw"

	"go-populous2/internal/engine"
	"go-populous2/internal/mobileui"
)

type mobileSceneCache struct {
	Image            *image.RGBA
	World            *engine.World
	Assets           *Assets
	Update           int
	Origin           image.Point
	Clip             image.Rectangle
	Ready            bool
	SelectedFollower int
	Inspecting       bool
}

// drawCachedMobileScene paints the expensive terrain/actor layer once per
// original main cycle. HUD and touch previews remain responsive on every
// display update, and moving the camera immediately refreshes this layer.
func (g *Game) drawCachedMobileScene(destination *image.RGBA, view mobileui.Viewport, spritePadding int, cache *mobileSceneCache) {
	if cache == nil {
		g.drawMobileScene(destination, view, spritePadding)
		return
	}
	clip := view.Rect.Intersect(destination.Bounds())
	if g.World == nil || clip.Empty() {
		cache.Ready = false
		return
	}
	originX, originY := view.Origin()
	origin := image.Pt(originX, originY)
	displayed := g.displayedGame()
	if cache.Image == nil || cache.Clip != clip {
		cache.Image = image.NewRGBA(clip)
		cache.Ready = false
	}
	if !cache.Ready || cache.World != g.World || cache.Assets != g.Assets || cache.Update != g.Updates/4 || cache.Origin != origin || cache.SelectedFollower != displayed.SelectedFollower || cache.Inspecting != displayed.Inspecting {
		g.drawMobileScene(cache.Image, view, spritePadding)
		cache.World, cache.Assets, cache.Update, cache.Origin, cache.Clip = g.World, g.Assets, g.Updates/4, origin, clip
		cache.SelectedFollower, cache.Inspecting, cache.Ready = displayed.SelectedFollower, displayed.Inspecting, true
	}
	draw.Draw(destination, clip, cache.Image, clip.Min, draw.Src)
}

// drawMobileScene reuses every original actor/effect drawing handler. Parcels
// retain Populous II's row traversal and mixed oldest-to-newest actor chain;
// sorting followers separately would change overlaps with walls and spells.
// Clipping is carried by the destination image, including tall effect layers.
func (g *Game) drawMobileScene(destination *image.RGBA, view mobileui.Viewport, spritePadding int) {
	if g.World == nil || destination == nil {
		return
	}
	clip := view.Rect.Intersect(destination.Bounds())
	if clip.Empty() {
		return
	}
	renderer := g.displayedGame()
	renderer.framebuffer = destination.SubImage(clip).(*image.RGBA)
	renderer.sceneProjection = &view
	w := renderer.World
	land := w.Level.Landscape
	if land < 0 || land >= len(g.Assets.Visual.Tiles) {
		return
	}
	background := image.Uniform{C: g.Assets.Visual.Palettes[land][0]}
	draw.Draw(renderer.framebuffer, clip, &background, image.Point{}, draw.Src)
	bounds := view.Bounds(engine.MapSize, engine.MapSize, 8, spritePadding)
	var chain [engine.FollowerCapacity + engine.EffectCapacity + engine.SceneryCapacity + engine.WallCapacity + 2]engine.ActorRef
	originX, originY := view.Origin()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cell := w.Cell(x, y)
			sx, sy := originX+16*(x-y)-16, originY+8*(x+y)-int(cell.BaseAltitude)*8-8
			index := cell.TileIndex(int(w.Tick), x-g.CameraX, y-g.CameraY)
			if index >= 0 && index < len(g.Assets.Visual.Tiles[land]) {
				tile := g.Assets.Visual.Tiles[land][index]
				draw.Draw(renderer.framebuffer, image.Rect(sx, sy, sx+tile.Bounds().Dx(), sy+tile.Bounds().Dy()), tile, image.Point{}, draw.Over)
			}
			renderer.drawTownSurroundingsAt(x, y, land)
			count := mobileParcelChain(w, x, y, chain[:])
			for i := count - 1; i >= 0; i-- {
				renderer.drawRegisteredActor(chain[i], land)
			}
		}
	}
	renderer.drawLightningBeams(land)
	g.AnimationSounds = renderer.AnimationSounds
}

func mobileParcelChain(w *engine.World, x, y int, chain []engine.ActorRef) int {
	count := 0
	for ref := w.Actors.Heads[x+y*engine.MapSize]; ref.Kind != engine.ActorNone && count < len(chain); ref = w.Actors.Next(ref) {
		chain[count] = ref
		count++
	}
	return count
}

// mobileAnimationVisible rejects completely clipped layers before their cue
// or sprite copy. Tall-spell culling must not make hidden ordinary followers
// restart sounds simply because their parcels share the larger search box.
func (g *Game) mobileAnimationVisible(animationName string, frame, x, y, land, age int) bool {
	animation, ok := g.Assets.Visual.Animations[animationName]
	if !ok || frame < 0 || frame >= len(animation.Frames) || land < 0 || land >= len(g.Assets.Visual.Sprites) {
		return false
	}
	for index, layer := range animation.Frames[frame].Layers {
		if age != 0 && index > 0 {
			break
		}
		if layer.Sprite < 0 || layer.Sprite >= len(g.Assets.Visual.Sprites[land]) {
			continue
		}
		sprite := g.Assets.Visual.Sprites[land][layer.Sprite]
		if sprite.Image == nil {
			continue
		}
		height, anchor := sprite.Image.Bounds().Dy(), sprite.AnchorY
		if age != 0 {
			height -= absInt(age)
			anchor -= 8 + absInt(age)
		}
		left, top := x+layer.X-sprite.AnchorX, y+layer.Y-anchor
		if height > 0 && image.Rect(left, top, left+sprite.Image.Bounds().Dx(), top+height).Overlaps(g.framebuffer.Bounds()) {
			return true
		}
	}
	return false
}

// mobileSpritePadding includes the atlas layers themselves and the storm's
// additional cloud displacement. It avoids a second approximation of effect
// extents when new original animation banks are added to the exported assets.
func mobileSpritePadding(assets *Assets) int {
	padding := 96
	if assets == nil || assets.Visual == nil {
		return padding
	}
	for _, animation := range assets.Visual.Animations {
		for _, frame := range animation.Frames {
			for _, layer := range frame.Layers {
				for _, sprites := range assets.Visual.Sprites {
					if layer.Sprite < 0 || layer.Sprite >= len(sprites) || sprites[layer.Sprite].Image == nil {
						continue
					}
					sprite := sprites[layer.Sprite]
					left, top := layer.X-sprite.AnchorX, layer.Y-sprite.AnchorY
					padding = max(padding, absInt(left), absInt(left+sprite.Image.Bounds().Dx()), absInt(top)+75, absInt(top+sprite.Image.Bounds().Dy())+75)
				}
			}
		}
	}
	return padding
}
