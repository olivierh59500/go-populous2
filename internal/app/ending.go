package app

import (
	"fmt"
	"image"
	"image/draw"

	"go-populous2/internal/visualassets"
)

// EndingPlayback has its own PAL presentation clock. Campaign simulation is
// frozen while the original ending artwork and bottom-row text are displayed.
type EndingPlayback struct {
	Sequence      *visualassets.EndingSequence
	Frame         int
	TextOffset    int
	intro         bool
	wait, updates int
}

func NewEndingPlayback(sequence *visualassets.EndingSequence) (*EndingPlayback, error) {
	if sequence == nil || len(sequence.Frames) == 0 || sequence.LoopStart < 0 || sequence.LoopStart >= len(sequence.Frames) || sequence.FrameWait < 1 || sequence.IntroWait < 1 || sequence.TextStepFrames < 1 || sequence.Text == "" {
		return nil, fmt.Errorf("ending artwork is incomplete")
	}
	return &EndingPlayback{Sequence: sequence, intro: true, wait: sequence.IntroWait}, nil
}

// Update advances one 50 Hz PAL update, not one Draw call. After the initial
// blank, each artwork frame remains visible for four blanks. The text advances
// one font cell every second artwork update, matching the supplied ending.
func (p *EndingPlayback) Update() {
	if p == nil || p.Sequence == nil {
		return
	}
	p.wait--
	if p.wait > 0 {
		return
	}
	if p.intro {
		p.intro = false
		p.wait = p.Sequence.FrameWait
		return
	}
	p.Frame++
	if p.Frame >= len(p.Sequence.Frames) {
		p.Frame = p.Sequence.LoopStart
	}
	p.updates++
	// The original text phase advances the stored offset after composing
	// the current row. Display uses the previous offset on odd updates.
	if p.updates%p.Sequence.TextStepFrames == 0 {
		p.TextOffset++
		if p.TextOffset > len(p.Sequence.Text) {
			p.TextOffset = 0
		}
	}
	p.wait = p.Sequence.FrameWait
}

func (p *EndingPlayback) Draw(dst *image.RGBA, font *visualassets.Font) {
	if p == nil || p.Sequence == nil || dst == nil || font == nil {
		return
	}
	frame := p.Sequence.Frames[p.Frame]
	draw.Draw(dst, dst.Bounds(), frame, image.Point{}, draw.Src)
	if p.updates > 0 {
		font.Draw(dst, p.Sequence.Text[p.TextOffset:], 0, 192, p.Sequence.Palette)
	}
}

func (g *Game) startEnding() error {
	playback, err := NewEndingPlayback(g.Assets.Visual.EndingSequence)
	if err != nil {
		return err
	}
	g.Ending = playback
	g.Screen = EndingScreen
	return nil
}

func (g *Game) drawEnding() {
	if g.Ending != nil {
		g.Ending.Draw(g.framebuffer, g.Assets.Visual.Font)
	}
}
