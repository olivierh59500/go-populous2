package visualassets

import (
	"fmt"
	"image"
	"image/color"
)

type EndingDescriptor struct {
	Frames         []string `json:"frames"`
	LoopStart      int      `json:"loop_start"`
	Text           string   `json:"text"`
	IntroWait      int      `json:"intro_wait"`
	FrameWait      int      `json:"frame_wait"`
	TextStepFrames int      `json:"text_step_frames"`
}

type EndingSequence struct {
	Frames                               []*image.Paletted
	Palette                              [16]color.RGBA
	LoopStart                            int
	Text                                 string
	IntroWait, FrameWait, TextStepFrames int
}

func (l *imageLoader) ending(desc *EndingDescriptor) (*EndingSequence, error) {
	if desc == nil {
		return nil, nil
	}
	if len(desc.Frames) < 1 || len(desc.Frames) > 256 || desc.LoopStart < 0 || desc.LoopStart >= len(desc.Frames) || len(desc.Text) == 0 || len(desc.Text) > 8192 || desc.IntroWait < 1 || desc.IntroWait > 50 || desc.FrameWait < 1 || desc.FrameWait > 50 || desc.TextStepFrames < 1 || desc.TextStepFrames > 50 {
		return nil, fmt.Errorf("invalid ending animation metadata")
	}
	s := &EndingSequence{Frames: make([]*image.Paletted, len(desc.Frames)), LoopStart: desc.LoopStart, Text: desc.Text, IntroWait: desc.IntroWait, FrameWait: desc.FrameWait, TextStepFrames: desc.TextStepFrames}
	for i, path := range desc.Frames {
		img, err := l.image(path)
		if err != nil {
			return nil, err
		}
		frame, ok := img.(*image.Paletted)
		if !ok || frame.Bounds() != image.Rect(0, 0, 320, 200) || len(frame.Palette) != 16 {
			return nil, fmt.Errorf("ending frame must be an indexed 320x200 PNG with 16 colors")
		}
		for index, c := range frame.Palette {
			rgba := color.RGBAModel.Convert(c).(color.RGBA)
			if i == 0 {
				s.Palette[index] = rgba
			} else if s.Palette[index] != rgba {
				return nil, fmt.Errorf("ending palette differs between frames")
			}
		}
		s.Frames[i] = frame
	}
	for _, code := range []byte(desc.Text) {
		if code < 32 || code > 127 {
			return nil, fmt.Errorf("ending text contains a code outside the imported font")
		}
	}
	return s, nil
}
