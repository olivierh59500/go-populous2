package app

import (
	"image"
	"image/color"
	"path/filepath"
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

func TestSelectedPanelRetainsLeaderAndHeroStatusAtLiveActorPhase(t *testing.T) {
	for _, hero := range []bool{false, true} {
		w := &engine.World{Tick: 99}
		w.Followers[1] = engine.Follower{State: engine.Walking, Owner: 0, X: 12, Y: 12, Frame: 1}
		name := "follower/0/0/north"
		if hero {
			w.Followers[1].Hero.Kind = engine.HeroPerseus
			name = "hero/perseus/north"
		} else {
			w.Players[0].Leader = 1
		}
		visual := &visualassets.Bundle{Animations: map[string]visualassets.Animation{
			name:                 {Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 0}}}, {Layers: []visualassets.SpriteLayer{{Sprite: 1}}}}, Loop: true},
			"marker/leader-blue": {Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 2}}}}},
		}}
		for _, c := range []color.RGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}} {
			pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
			pixel.SetRGBA(0, 0, c)
			visual.Sprites[0] = append(visual.Sprites[0], visualassets.Sprite{Image: pixel, AnchorY: 8})
		}
		g := &Game{World: w, SelectedFollower: 1, Assets: &Assets{Visual: visual, SelectionPanel: &visualassets.SelectionPanel{ActorX: 282, ActorY: 42, PopulationSprite: -1}}, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
		before := w.Snapshot()
		g.drawSelectionPanel()
		if g.framebuffer.RGBAAt(282, 34).G != 255 || g.framebuffer.RGBAAt(278, 26).B != 255 {
			t.Fatal("panel changed the live phase or hid its leader/hero indicator", hero)
		}
		if w.Snapshot() != before {
			t.Fatal("panel status presentation changed simulation")
		}
	}
}

func TestSelectedGroupAndInspectModeSurviveSessionSave(t *testing.T) {
	g := browserGame(t)
	g.SelectedFollower = g.World.Players[0].Leader
	g.Inspecting = true
	g.SavePath = filepath.Join(t.TempDir(), "selected.json")
	id := g.SelectedFollower
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	g.SelectedFollower, g.Inspecting = 0, false
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.SelectedFollower != id || !g.Inspecting {
		t.Fatal("saved selection or inspection mode was lost")
	}
}

func TestInspectPickingUsesOriginalAnchorAndLeavesManaUntouched(t *testing.T) {
	w := &engine.World{}
	w.Followers[1] = engine.Follower{State: engine.Walking, Owner: 0, X: 12, Y: 12, Population: 100}
	w.Actors.Link(engine.ActorRef{Kind: engine.ActorFollower, Index: 1}, 12*256+128, 12*256+128)
	g := &Game{World: w, CameraX: 8, CameraY: 8, Assets: &Assets{Visual: &visualassets.Bundle{}}}
	x, y := g.followerRenderAnchor(w.Followers[1])
	before := w.Snapshot()
	if id := g.pickFollower(x+8, y-18); id != 1 {
		t.Fatal("original walking inspect boundary did not select actor", id)
	}
	if id := g.pickFollower(x+9, y-18); id != 0 {
		t.Fatal("inspect hit extended beyond original horizontal boundary")
	}
	if w.Snapshot() != before {
		t.Fatal("inspect hit query changed world or mana")
	}
}
