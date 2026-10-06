package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
	"image"
	"image/color"
	"testing"
)

func TestActiveBattleUsesSharedTwoFrameArtAndSuppressesDefender(t *testing.T) {
	for _, hero := range []engine.HeroKind{engine.HeroNone, engine.HeroPerseus, engine.HeroAchilles} {
		f := engine.Follower{State: engine.Fighting, BattleAggressor: true, Frame: 3, Hero: engine.HeroState{Kind: hero}}
		name, frame, handled := activeBattleArtwork(f)
		if !handled || name != "combat/attack" || frame != 1 {
			t.Fatal("aggressor art was replaced by walking/hero art")
		}
		f.BattleAggressor = false
		if name, _, handled := activeBattleArtwork(f); !handled || name != "" {
			t.Fatal("ordinary defender received an independent sprite")
		}
	}
	if _, _, handled := activeBattleArtwork(engine.Follower{State: engine.Walking}); handled {
		t.Fatal("battle renderer consumed ordinary walker")
	}
}

func TestLeaderMarkerUsesLastCompositeLayerTop(t *testing.T) {
	visual := &visualassets.Bundle{Animations: map[string]visualassets.Animation{}}
	marker := image.NewRGBA(image.Rect(0, 0, 2, 2))
	marker.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	visual.Sprites[0] = []visualassets.Sprite{{Image: marker, AnchorX: 1, AnchorY: 2}, {Image: image.NewRGBA(image.Rect(0, 0, 16, 20)), AnchorX: 8, AnchorY: 20}}
	visual.Animations["marker/leader-blue"] = visualassets.Animation{Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 0}}}}}
	visual.Animations["follower/sample"] = visualassets.Animation{Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 1, Y: 0}, {Sprite: 1, Y: -12}}}}}
	w := &engine.World{}
	w.Players[0].Leader = 1
	w.Followers[1] = engine.Follower{Owner: 0, State: engine.Walking}
	g := &Game{World: w, Assets: &Assets{Visual: visual}, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.drawLeaderMarker(1, 100, 100, 0, "follower/sample", 0)
	if g.framebuffer.RGBAAt(96, 60).R != 255 {
		t.Fatal("leader marker used an actor constant instead of final composite-layer top")
	}
}
