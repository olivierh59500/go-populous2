package game

import (
	"image"
	"image/draw"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous2/internal/fixedstep"
	"go-populous2/internal/populous2"
)

func (g *Game) beginNativeEnding(progress populous2.CampaignProgress, custom bool) error {
	presentation, err := populous2.DecodeNativePresentation(g.Bundle.Executable)
	if err != nil {
		return err
	}
	playback, err := populous2.NewNativeEndingPlayback(g.Bundle.Raw["end.pak"], presentation, g.endingScrollPhase)
	if err != nil {
		return err
	}
	g.ending = playback
	g.endingProgress, g.endingCustom = progress, custom
	g.endingScheduler = fixedstep.New(populous2.SimulationRate, 60)
	g.endingPixels = image.NewRGBA(image.Rect(0, 0, 320, 200))
	g.endingImage = ebiten.NewImage(320, 200)
	return g.updateEndingImage()
}

func (g *Game) updateEndingImage() error {
	frame, err := g.ending.Ending.Image()
	if err != nil {
		return err
	}
	draw.Draw(g.endingPixels, g.endingPixels.Bounds(), frame, image.Point{}, draw.Src)
	g.endingImage.WritePixels(g.endingPixels.Pix)
	return nil
}

func (g *Game) updateNativeEnding() error {
	if len(inpututil.AppendJustPressedKeys(nil)) != 0 {
		g.ending.PressKey()
	}
	for range g.endingScheduler.Advance() {
		changed, err := g.ending.VBlank()
		if err != nil {
			return err
		}
		if changed {
			if err := g.updateEndingImage(); err != nil {
				return err
			}
		}
		if g.ending.Finished {
			g.endingScrollPhase = g.ending.Ending.ScrollPhase
			progress, custom := g.endingProgress, g.endingCustom
			g.ending, g.endingImage, g.endingPixels, g.endingScheduler = nil, nil, nil, nil
			// $b67c stops audio on exit, then opens the profile award screen.
			if g.audioPlayer != nil {
				if err := g.audioPlayer.Close(); err != nil {
					return err
				}
				g.audioPlayer, g.audioReplay = nil, nil
			}
			if err := g.start(int(progress.NextWorld), custom, false); err != nil {
				return err
			}
			if progress.OpenDeity && g.Profile.Bolts != 0 {
				g.OpenDeity()
			}
			return nil
		}
	}
	return nil
}

func (g *Game) playNativeResultCue() {
	if g.resultCuePlayed || g.audioReplay == nil {
		return
	}
	cue := 36 // Native descriptor $168, divided by its ten-byte stride.
	if g.World.NativeResult.Eliminated == 1 {
		cue = 128 // Native descriptor $500.
	}
	g.audioReplay.PlayCue(cue)
	g.resultCuePlayed = true
}
