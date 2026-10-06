package visualassets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"io"
	"io/fs"
)

const SpellHelpArtName = "spell-help.json"

type HelpFrameDescriptor struct {
	Image     Region
	Position  image.Point
	SoundCues []int `json:"sound_cues,omitempty"`
}

type HelpSequenceDescriptor struct {
	Frames    []HelpFrameDescriptor
	LoopStart int
}

// SpellHelpDescriptor is ordinary presentation data: English glyphs, icons,
// cropped graphics, sound cue indices and an explicit animation loop.
type SpellHelpDescriptor struct {
	Version, FramesPerSecond          int
	Layout                            RequesterLayout
	IconPosition, DescriptionPosition image.Point
	Descriptions                      [36]string
	Icons                             [36]Region
	Sequences                         [4][36]HelpSequenceDescriptor
}

type HelpFrame struct {
	Image     *image.RGBA
	Position  image.Point
	SoundCues []int
}

type HelpSequence struct {
	Frames    []HelpFrame
	LoopStart int
}

func (s *HelpSequence) FrameIndex(age int) int {
	if s == nil || len(s.Frames) == 0 {
		return -1
	}
	if age < 0 {
		age = 0
	}
	if age < len(s.Frames) {
		return age
	}
	return s.LoopStart + (age-s.LoopStart)%(len(s.Frames)-s.LoopStart)
}

type SpellHelpArt struct {
	Descriptor SpellHelpDescriptor
	Icons      [36]*image.RGBA
	Sequences  [4][36]HelpSequence
}

func LoadSpellHelpArt(files fs.FS) (*SpellHelpArt, error) {
	data, err := readLimited(files, SpellHelpArtName, 2<<20)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	art := &SpellHelpArt{}
	if err := decoder.Decode(&art.Descriptor); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("spell help metadata has trailing data")
	}
	d := &art.Descriptor
	if d.Version != 1 || d.FramesPerSecond != 10 || d.Layout.Name != "spell-help" {
		return nil, fmt.Errorf("unsupported spell help presentation")
	}
	if err := d.Layout.Validate(); err != nil {
		return nil, err
	}
	for _, p := range []image.Point{d.IconPosition, d.DescriptionPosition} {
		if !p.In(image.Rect(0, 0, 320, 200)) {
			return nil, fmt.Errorf("spell help field is outside the display")
		}
	}
	loader := imageLoader{files: files, images: make(map[string]image.Image)}
	for slot, text := range d.Descriptions {
		if len(text) > 1024 {
			return nil, fmt.Errorf("spell help description is too long")
		}
		for _, glyph := range []byte(text) {
			if glyph != '\n' && (glyph < 32 || glyph > 126) {
				return nil, fmt.Errorf("spell help description contains invalid glyphs")
			}
		}
		if d.Icons[slot].Width != 32 || d.Icons[slot].Height < 1 || d.Icons[slot].Height > 40 {
			return nil, fmt.Errorf("spell help icon dimensions differ")
		}
		art.Icons[slot], err = loader.region(d.Icons[slot], false)
		if err != nil {
			return nil, err
		}
	}
	for land := range d.Sequences {
		for slot, descriptor := range d.Sequences[land] {
			if len(descriptor.Frames) < 1 || len(descriptor.Frames) > 256 || descriptor.LoopStart < 0 || descriptor.LoopStart >= len(descriptor.Frames) {
				return nil, fmt.Errorf("invalid spell help animation loop")
			}
			sequence := &art.Sequences[land][slot]
			sequence.LoopStart = descriptor.LoopStart
			sequence.Frames = make([]HelpFrame, len(descriptor.Frames))
			for i, frame := range descriptor.Frames {
				if !image.Rect(frame.Position.X, frame.Position.Y, frame.Position.X+frame.Image.Width, frame.Position.Y+frame.Image.Height).In(image.Rect(0, 0, 320, 200)) || len(frame.SoundCues) > 16 {
					return nil, fmt.Errorf("spell help frame is outside the display")
				}
				for _, cue := range frame.SoundCues {
					if cue < 1 || cue >= 133 {
						return nil, fmt.Errorf("invalid spell help sound cue")
					}
				}
				sequence.Frames[i].Image, err = loader.region(frame.Image, false)
				if err != nil {
					return nil, err
				}
				sequence.Frames[i].Position = frame.Position
				sequence.Frames[i].SoundCues = frame.SoundCues
			}
		}
	}
	return art, nil
}

func (a *SpellHelpArt) DrawBase(dst *image.RGBA, font *Font, slot int) {
	if a == nil || font == nil || dst == nil || slot < 0 || slot >= 36 {
		return
	}
	d := &a.Descriptor
	d.Layout.Draw(dst, font, nil, nil)
	if icon := a.Icons[slot]; icon != nil {
		draw.Draw(dst, icon.Bounds().Add(d.IconPosition), icon, image.Point{}, draw.Over)
	}
	font.Draw(dst, d.Descriptions[slot], d.DescriptionPosition.X, d.DescriptionPosition.Y, d.Layout.Palette)
}

func (a *SpellHelpArt) DrawFrame(dst *image.RGBA, land, slot, age int) {
	if a == nil || dst == nil || land < 0 || land >= 4 || slot < 0 || slot >= 36 {
		return
	}
	s := &a.Sequences[land][slot]
	index := s.FrameIndex(age)
	if index < 0 {
		return
	}
	f := s.Frames[index]
	if f.Image != nil {
		draw.Draw(dst, f.Image.Bounds().Add(f.Position), f.Image, image.Point{}, draw.Over)
	}
}
