package app

import (
	"testing"

	"go-populous2/internal/engine"
)

func TestPlayingPresentationKeepsPrePhysicsStateAndEarlyClock(t *testing.T) {
	g := selectionReturnGame()
	g.World.Tick = 12
	g.World.Heights[0] = 2
	g.World.Followers[1].Frame = 1
	g.CameraX, g.CameraY = 8, 9
	g.Category, g.Selected = engine.Fire, engine.FireRain
	var p PlayingPresentation
	p.Capture(g, true)
	buffer := p.World
	g.World.Tick++
	g.World.Heights[0] = 3
	g.World.Followers[1].Frame = 2
	g.SelectedFollower = 2
	g.CameraX = 10
	view := p.Renderer(g)
	if view.World.Tick != 13 || view.World.Followers[1].Frame != 1 || view.World.Heights[0] != 2 || view.SelectedFollower != 1 || view.CameraX != 8 || view.CameraY != 9 || view.Category != engine.Fire || view.Selected != engine.FireRain {
		t.Fatal("presentation saw following physics/input state instead of the source's pre-physics image")
	}
	if g.World.Followers[1].Frame != 2 || g.SelectedFollower != 2 {
		t.Fatal("presentation replaced the live simulation")
	}
	p.Capture(g, false)
	if p.World != buffer || p.World.Tick != g.World.Tick {
		t.Fatal("paused capture advanced its clock or replaced its persistent buffer")
	}
	p.Reset()
	if p.Ready || p.World != buffer {
		t.Fatal("reset discarded reusable presentation storage")
	}
}

func TestPlayingPresentationCaptureHasNoSteadyStateAllocation(t *testing.T) {
	g := selectionReturnGame()
	var p PlayingPresentation
	p.Capture(g, false)
	if got := testing.AllocsPerRun(100, func() { p.Capture(g, true) }); got != 0 {
		t.Fatal("presentation capture allocated per main frame", got)
	}
}

func BenchmarkPlayingPresentationCapture(b *testing.B) {
	g := &Game{World: &engine.World{}}
	var p PlayingPresentation
	p.Capture(g, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Capture(g, true)
	}
}
