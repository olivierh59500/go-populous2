package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
	"image"
	"testing"
)

func TestVisibleFollowersUseOneGeometricBucket(t *testing.T) {
	world := &engine.World{}
	world.Followers[1] = engine.Follower{State: engine.Walking, X: 12, Y: 12, Owner: 0}
	world.Followers[2] = engine.Follower{State: engine.Walking, X: 12, Y: 12, Owner: 1}
	world.Followers[3] = engine.Follower{State: engine.Walking, X: 63, Y: 63, Owner: 0}
	visual := &visualassets.Bundle{Background: image.NewRGBA(image.Rect(0, 0, 320, 200)), Animations: map[string]visualassets.Animation{}}
	g := &Game{Assets: &Assets{Visual: visual}, World: world, CameraX: 8, CameraY: 8, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.drawWorld()
	at := 4 + 4*viewSize
	first := g.visibleFollowers[at]
	if first != 2 || g.visibleNext[first] != 1 || g.visibleNext[1] != 0 {
		t.Fatal("mixed followers lost geometric bucket ordering", first, g.visibleNext)
	}
	for _, id := range g.visibleFollowers {
		if id == 3 {
			t.Fatal("offscreen follower entered render bucket")
		}
	}
}
