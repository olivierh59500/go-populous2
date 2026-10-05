package populous2

import (
	"fmt"
	"image"
	"image/color"
)

// Overlay preserves pixels outside requester glyph cells. Index zero inside
// those cells remains opaque, as in the original four-plane font writer.
func (p *NativePresentation) Overlay(requester *NativeRequester, palette [16]color.RGBA) (*image.RGBA, error) {
	if p == nil || p.Font == nil || requester == nil {
		return nil, fmt.Errorf("native requester overlay input missing")
	}
	indices := make([]byte, NativeMenuWidth*NativeMenuHeight)
	for index := range indices {
		indices[index] = 16
	}
	if err := p.Font.DrawIndices(indices, string(requester.Text), requester.Column, requester.Row); err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, NativeMenuWidth, NativeMenuHeight))
	for index, value := range indices {
		if value < 16 {
			c := palette[value]
			img.Pix[index*4], img.Pix[index*4+1], img.Pix[index*4+2], img.Pix[index*4+3] = c.R, c.G, c.B, 255
		}
	}
	return img, nil
}

// NativeResultPlayback retains the original wait before requester composition.
// It runs at PAL VBlank cadence independently of the simulation speed override.
type NativeResultPlayback struct {
	Wait int
}

func (playback *NativeResultPlayback) VBlank() bool {
	if playback.Wait > 0 {
		playback.Wait--
	}
	return playback.Wait == 0
}
