package app

import (
	"image"
	"image/color"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func TestSelectionPanelRecentersWithoutChangingSimulation(t *testing.T) {
	w := controllerWorld(t)
	id := w.Players[0].Leader
	f := w.Followers[id]
	g := &Game{World: w, SelectedFollower: id, Assets: &Assets{SelectionPanel: &visualassets.SelectionPanel{HitX: 245, HitY: 0, HitWidth: 70, HitHeight: 46}}}
	before := w.Snapshot()
	if !g.handleSelectionPanelClick(282, 22, true) {
		t.Fatal("original information panel click not consumed")
	}
	if g.CameraX != max(0, min(56, int(f.X)-4)) || g.CameraY != max(0, min(56, int(f.Y)-4)) || w.Snapshot() != before {
		t.Fatal("panel recenter changed simulation or selected wrong position")
	}
}

func TestSelectedArtworkUsesPanelAnchorAndDoesNotChangeWorld(t *testing.T) {
	w := &engine.World{}
	w.Followers[1] = engine.Follower{State: engine.Walking, Owner: 0, X: 12, Y: 12, Population: 0}
	visual := &visualassets.Bundle{Animations: map[string]visualassets.Animation{"follower/0/0/north": {Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 0}}}}}}}
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	visual.Sprites[0] = []visualassets.Sprite{{Image: img}}
	g := &Game{World: w, SelectedFollower: 1, Assets: &Assets{Visual: visual, SelectionPanel: &visualassets.SelectionPanel{ActorX: 282, ActorY: 42, PopulationSprite: 0}}, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	before := w.Snapshot()
	g.drawSelectionPanel()
	if g.framebuffer.RGBAAt(282, 42).R != 255 || g.framebuffer.RGBAAt(192, 72).A != 0 {
		t.Fatal("selected actor used its world anchor instead of the original panel")
	}
	if w.Snapshot() != before {
		t.Fatal("selection panel rendering changed the world")
	}
}
