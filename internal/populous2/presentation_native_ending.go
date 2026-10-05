package populous2

import (
	"fmt"
	"image"
)

const (
	NativeEndingIntroWaitVBlanks = 1
	NativeEndingWaitVBlanks      = 4
)

// NativeEnding translates $b142's presentation loop. The supplied END.PAK
// artwork and CODE:$aa24 text remain unmodified. The text row is 192 pixels;
// one byte advances every other four-VBlank presentation update.
type NativeEnding struct {
	Animation    *NativeScreenAnimation
	TextOffset   uint16
	ScrollPhase  uint16
	presentation *NativePresentation
}

func NewNativeEnding(raw []byte, presentation *NativePresentation, scrollPhase uint16) (*NativeEnding, error) {
	if presentation == nil || presentation.Font == nil || len(presentation.EndingText) == 0 {
		return nil, fmt.Errorf("native ending presentation missing")
	}
	a, err := NewNativeScreenAnimation(raw)
	if err != nil {
		return nil, err
	}
	// INIT is displayed, then FRM2 prepares the other screen after one
	// VBlank. Unlike subsequent iterations, this first delta is not swapped.
	if err := a.Advance(); err != nil {
		return nil, err
	}
	a.Planes, a.otherPlanes = a.otherPlanes, a.Planes
	return &NativeEnding{Animation: a, ScrollPhase: scrollPhase, presentation: presentation}, nil
}

func nativePackScreen(pixels []byte, planes *[32000]byte) error {
	if len(pixels) != 320*200 || planes == nil {
		return fmt.Errorf("native planar screen input missing")
	}
	clear(planes[:])
	for y := range 200 {
		for x := range 320 {
			for plane := range 4 {
				planes[plane*8000+y*40+x/8] |= (pixels[y*320+x] >> uint(plane) & 1) << uint(7-x%8)
			}
		}
	}
	return nil
}

func (ending *NativeEnding) Advance() error {
	if ending == nil || ending.Animation == nil || ending.presentation == nil {
		return fmt.Errorf("native ending state missing")
	}
	offset := int(ending.TextOffset)
	if offset > len(ending.presentation.EndingText) {
		return fmt.Errorf("native ending scroll offset outside text")
	}
	ending.ScrollPhase = ^ending.ScrollPhase
	if ending.ScrollPhase != 0 {
		ending.TextOffset++
	}
	if offset == len(ending.presentation.EndingText) {
		offset, ending.TextOffset = 0, 0
	}
	for _, planes := range []*[32000]byte{&ending.Animation.Planes, &ending.Animation.otherPlanes} {
		pixels, err := nativeScreenIndices(planes[:])
		if err != nil {
			return err
		}
		if err := ending.presentation.Font.DrawIndices(pixels, string(ending.presentation.EndingText[offset:]), 0, 192); err != nil {
			return err
		}
		if err := nativePackScreen(pixels, planes); err != nil {
			return err
		}
	}
	return ending.Animation.Advance()
}

func (ending *NativeEnding) Image() (*image.Paletted, error) {
	if ending == nil || ending.Animation == nil {
		return nil, fmt.Errorf("native ending state missing")
	}
	return ending.Animation.Image()
}
