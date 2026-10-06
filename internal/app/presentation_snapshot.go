package app

import "go-populous2/internal/engine"

// PlayingPresentation retains the ordinary game values seen by the source
// main renderer before its following physics pass. Its buffer is allocated
// once, keeping repeated captures and small Game renderer copies inexpensive.
type PlayingPresentation struct {
	World                              *engine.World
	Ready                              bool
	SelectedFollower, CameraX, CameraY int
	Category                           engine.Element
	Selected                           engine.PowerID
	Inspecting, PickingPower, Paused   bool
}

func (p *PlayingPresentation) Capture(g *Game, advanceClock bool) {
	p.Ready = g != nil && g.World != nil
	if !p.Ready {
		return
	}
	if p.World == nil {
		p.World = new(engine.World)
	}
	*p.World = *g.World
	// The original main clock advances before drawing; actor/effect state
	// advances only in the later physics pass. Paused callers pass false.
	if advanceClock {
		p.World.Tick++
	}
	p.SelectedFollower, p.CameraX, p.CameraY = g.SelectedFollower, g.CameraX, g.CameraY
	p.Category, p.Selected = g.Category, g.Selected
	p.Inspecting, p.PickingPower, p.Paused = g.Inspecting, g.PickingPower, g.Paused
}

// Renderer borrows the captured values for drawing. Input, simulation and
// selection transfer consumers continue to use the live Game and World.
func (p *PlayingPresentation) Renderer(g *Game) Game {
	view := *g
	if !p.Ready {
		return view
	}
	view.World = p.World
	view.SelectedFollower, view.CameraX, view.CameraY = p.SelectedFollower, p.CameraX, p.CameraY
	view.Category, view.Selected = p.Category, p.Selected
	view.Inspecting, view.PickingPower, view.Paused = p.Inspecting, p.PickingPower, p.Paused
	return view
}

func (p *PlayingPresentation) Reset() { p.Ready = false }
