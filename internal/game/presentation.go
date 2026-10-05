package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous2/internal/fixedstep"
	"go-populous2/internal/populous2"
)

func (g *Game) initializeNativePresentation() error {
	p, err := populous2.DecodeNativePresentation(g.Bundle.Executable)
	if err != nil {
		return err
	}
	startup, err := p.StartupImage()
	if err != nil {
		return err
	}
	g.nativePresentation = p
	g.startupImage = ebiten.NewImageFromImage(startup)
	g.worldRequesters, err = populous2.DecodeNativeInGameRequesterRules(g.Bundle.Executable)
	if err != nil {
		return err
	}
	return nil
}

func (g *Game) updateNativeStartup() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.OpenDeity()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		g.LevelIndex = max(0, g.LevelIndex-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		g.LevelIndex = min(len(g.Bundle.Levels)-1, g.LevelIndex+1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return g.openWorldRequester()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyD) {
		return g.start(g.LevelIndex, false, true)
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if y < 40 {
		return nil
	}
	action := g.nativePresentation.Requesters.Click(g.nativePresentation.StartupRequester, x/2, (y-40)/2)
	switch action {
	case 2:
		g.OpenDeity()
	case 4:
		return g.openWorldRequester()
	case 6:
		return g.start(g.LevelIndex, true, false)
	case 8:
		g.load()
	case 10:
		return ebiten.Termination
	}
	return nil
}

func (g *Game) updateNativeResult() error {
	g.playNativeResultCue()
	if g.resultPlayback == nil {
		r := g.World.NativeResult
		plan, err := g.nativePresentation.Result(uint16(g.World.NativeProfileSide), r.Eliminated, g.World.NativeClock, r.Local, r.Opponent, r.Score.Value)
		if err != nil {
			return err
		}
		g.resultRequester = plan.Requester
		g.resultPlayback = &populous2.NativeResultPlayback{Wait: plan.WaitVBlanks}
		g.resultScheduler = fixedstep.New(populous2.SimulationRate, 60)
	}
	for range g.resultScheduler.Advance() {
		if g.resultPlayback.VBlank() && g.resultImage == nil {
			img, err := g.nativePresentation.Overlay(g.resultRequester, g.World.Landscape.Palettes[0])
			if err != nil {
				return err
			}
			g.resultImage = ebiten.NewImageFromImage(img)
		}
	}
	if g.resultImage == nil || g.World.NativeResult.ScoreError != "" {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return g.advanceNativeCampaign()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if y >= 40 && g.nativePresentation.Requesters.Click(g.resultRequester, x/2, (y-40)/2) == 2 {
			return g.advanceNativeCampaign()
		}
	}
	return nil
}

func (g *Game) resetNativeResultPresentation() {
	g.resultCuePlayed = false
	g.resultPlayback, g.resultScheduler, g.resultRequester, g.resultImage, g.resultBackground = nil, nil, nil, nil, nil
}

func (g *Game) drawNativeResultOverlay(screen *ebiten.Image) {
	if g.resultImage != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(0, 40)
		screen.DrawImage(g.resultImage, op)
	}
	if g.World.NativeResult.ScoreError != "" {
		label(screen, truncate(g.World.NativeResult.ScoreError, 82), 132, 427, ink)
	}
}
