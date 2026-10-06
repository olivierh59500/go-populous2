package app

import (
	"image"

	"go-populous2/internal/engine"
	"go-populous2/internal/mobileui"
)

// syncMobileScreen suppresses a finger held across a menu or result transition.
// Call it again after simulation, since a main pass can finish the conquest.
func (g *Game) syncMobileScreen() {
	if m := g.mobile; m != nil && m.LastScreen != g.Screen {
		m.Gesture.Cancel()
		m.LastScreen = g.Screen
	}
}

// syncMobileWorld distinguishes a new local game from the detached World
// values published by lockstep. A network publication refreshes artwork but
// leaves the player's camera and an ongoing two-finger drag undisturbed.
func (g *Game) syncMobileWorld() {
	m := g.mobile
	if m == nil || g.World == nil {
		return
	}
	if g.World != m.World {
		if m.World == nil || g.Network == nil {
			m.NeedsCenter = true
			m.Gesture.Cancel()
		}
		m.World = g.World
	}
	if m.NeedsCenter {
		m.View.CenterX, m.View.CenterY = float64(g.CameraX)+3.5, float64(g.CameraY)+3.5
		m.NeedsCenter = false
	}
}

// handleMobileTap translates a completed gesture through the same geometry
// used to draw controls. It is independent of Android's touch ID lifecycle.
func (g *Game) handleMobileTap(tap mobileui.Tap) error {
	m := g.mobile
	if m == nil {
		return nil
	}
	if tap.Region == mobileui.RegionUI {
		a, ok := mobileButtonAt(m.Buttons, int(tap.X), int(tap.Y))
		start, startOK := mobileButtonAt(m.Buttons, int(tap.Start.X), int(tap.Start.Y))
		if !ok || !startOK || a.Action != start.Action || !a.Enabled {
			return nil
		}
		m.Gesture.Cancel()
		if g.Screen == Playing {
			return g.handleMobileAction(a.Action)
		}
		return g.handleMobileFront(a.Action)
	}
	if tap.Region != mobileui.RegionTerrain || g.mobileRegion(tap.Point) != mobileui.RegionTerrain {
		return nil
	}
	if g.Screen == EditorScreen && m.Overlay == "paint" {
		if g.Editor == nil || g.Editor.Draft == nil {
			return nil
		}
		x, y := int(tap.X)-(m.Width-320)/2, int(tap.Y)-24
		view := *g
		view.World = g.Editor.Draft
		cx, cy, ok := view.pickCorner(x, y)
		if ok {
			return g.Editor.paint(cx, cy)
		}
		return nil
	}
	if g.Screen == Playing && m.Overlay == "map" {
		mapRect := image.Rect((m.Width-128)/2, 32, (m.Width+128)/2, 160)
		if !image.Pt(int(tap.X), int(tap.Y)).In(mapRect) {
			return nil
		}
		m.View.CenterX = float64(int(tap.X)-mapRect.Min.X) / 2
		m.View.CenterY = float64(int(tap.Y)-mapRect.Min.Y) / 2
		m.View = m.View.Clamp(engine.MapSize, engine.MapSize)
		m.Overlay = ""
		m.Gesture.Cancel()
		return nil
	}
	if g.Screen == Playing && m.Overlay == "" {
		return g.applyMobileTerrain(int(tap.X), int(tap.Y))
	}
	return nil
}
