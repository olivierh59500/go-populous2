package app

import (
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func TestInspectHitUsesDisplayedActorAndCameraBeforeFollowingPhysics(t *testing.T) {
	w := &engine.World{}
	w.Followers[1] = engine.Follower{State: engine.Walking, X: 12, Y: 12, Population: 100}
	w.Actors.Link(engine.ActorRef{Kind: engine.ActorFollower, Index: 1}, 12*256+128, 12*256+128)
	g := &Game{Screen: Playing, World: w, CameraX: 8, CameraY: 8, Assets: &Assets{Visual: &visualassets.Bundle{}}}
	x, y := g.followerRenderAnchor(w.Followers[1])
	g.presentation.Capture(g, false)
	w.Followers[1].X = 30
	w.Actors.Link(engine.ActorRef{Kind: engine.ActorFollower, Index: 1}, 30*256+128, 12*256+128)
	g.CameraX = 24
	if got := g.pickFollower(x, y-4); got != 1 {
		t.Fatal("click missed the actor still displayed before the next main image", got)
	}
	g.presentation.Reset()
	if got := g.pickFollower(x, y-4); got != 0 {
		t.Fatal("reset continued using an obsolete displayed actor", got)
	}
}

func TestCornerHitUsesDisplayedHeightBeforeLiveTerrainChanges(t *testing.T) {
	w := &engine.World{}
	g := &Game{Screen: Playing, World: w, CameraX: 8, CameraY: 8}
	w.Heights[12+12*engine.CornerSize] = 2
	x, y := g.projectCorner(12, 12)
	g.presentation.Capture(g, false)
	w.Heights[12+12*engine.CornerSize] = 5
	g.CameraX = 24
	cx, cy, ok := g.pickCorner(x, y)
	if !ok || cx != 12 || cy != 12 {
		t.Fatal("terrain click used geometry from the following simulation", cx, cy, ok)
	}
}
