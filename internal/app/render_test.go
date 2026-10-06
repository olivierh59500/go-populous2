package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestVisibleFollowersUseMixedRegistryAndExcludeOffscreenActor(t *testing.T) {
	world := &engine.World{}
	world.Followers[1] = engine.Follower{State: engine.Walking, X: 12, Y: 12, Owner: 0}
	world.Followers[2] = engine.Follower{State: engine.Walking, X: 12, Y: 12, Owner: 1}
	world.Followers[3] = engine.Follower{State: engine.Walking, X: 63, Y: 63, Owner: 0}
	for id := 1; id <= 3; id++ {
		f := world.Followers[id]
		world.Actors.Link(engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(id)}, int(f.X)*256, int(f.Y)*256)
	}
	visual := &visualassets.Bundle{Background: image.NewRGBA(image.Rect(0, 0, 320, 200)), Animations: map[string]visualassets.Animation{}}
	for _, c := range []color.RGBA{{B: 255, A: 255}, {R: 255, A: 255}} {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
		visual.Sprites[0] = append(visual.Sprites[0], visualassets.Sprite{Image: img})
	}
	visual.Animations["follower/0/0/north"] = visualassets.Animation{Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 0}}}}}
	visual.Animations["follower/1/0/north"] = visualassets.Animation{Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 1}}}}}
	g := &Game{Assets: &Assets{Visual: visual}, World: world, CameraX: 8, CameraY: 8, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.drawWorld()
	ax, ay := g.followerRenderAnchor(world.Followers[1])
	if got := g.framebuffer.RGBAAt(ax, ay); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatal("newer mixed-chain actor did not cover the older actor", got)
	}
	world.Followers[3].X, world.Followers[3].Y = 12, 12 // Its registry location remains outside the view.
	g.drawWorld()
	if got := g.framebuffer.RGBAAt(ax, ay); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatal("offscreen registry actor was drawn through a geometric pool scan", got)
	}
}

func TestFollowerArtUsesSavedAppearanceAndSilentContactState(t *testing.T) {
	world := &engine.World{}
	world.Followers[1] = engine.Follower{State: engine.Walking, X: 12, Y: 12, Owner: 0, AppearanceVariant: 7}
	visual := &visualassets.Bundle{Animations: map[string]visualassets.Animation{}}
	for _, c := range []color.RGBA{{B: 255, A: 255}, {R: 255, A: 255}, {G: 255, A: 255}} {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.SetRGBA(0, 0, c)
		visual.Sprites[0] = append(visual.Sprites[0], visualassets.Sprite{Image: img})
	}
	for name, id := range map[string]int{"follower/0/0/north": 0, "follower/0/7/north": 1, "contact/waiting": 2} {
		visual.Animations[name] = visualassets.Animation{Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: id}}}}}
	}
	g := &Game{World: world, Assets: &Assets{Visual: visual}, CameraX: 8, CameraY: 8, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	ax, ay := g.followerRenderAnchor(world.Followers[1])
	g.drawFollowerActor(1, 0)
	if g.framebuffer.RGBAAt(ax, ay) != (color.RGBA{R: 255, A: 255}) {
		t.Fatal("saved appearance was replaced by slot-derived bank")
	}
	world.Followers[1].ContactWaiting = true
	g.drawFollowerActor(1, 0)
	if g.framebuffer.RGBAAt(ax, ay) != (color.RGBA{G: 255, A: 255}) {
		t.Fatal("waiting contact fell through to walking artwork")
	}
}
