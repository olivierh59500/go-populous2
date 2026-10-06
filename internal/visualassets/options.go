package visualassets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"io/fs"
)

const OptionsArtName = "options.json"

// OptionsArt describes the original single-page rules requester and its
// sixteen-position reaction indicator. Its fields contain no template grammar.
type OptionsArt struct {
	Version          int
	Layout           RequesterLayout
	SideNames        [2]string
	ReactionPosition image.Point
	ReactionGlyph    byte
}

func LoadOptionsArt(files fs.FS) (*OptionsArt, error) {
	data, err := readLimited(files, OptionsArtName, 1<<20)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	art := &OptionsArt{}
	if err := decoder.Decode(art); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("options metadata has trailing data")
	}
	if art.Version != 1 || art.Layout.Name != "options" || art.ReactionGlyph == 0 || art.ReactionPosition.X < 0 || art.ReactionPosition.X+128 > 320 || art.ReactionPosition.Y < 0 || art.ReactionPosition.Y+8 > 200 {
		return nil, fmt.Errorf("invalid options artwork metadata")
	}
	if err := art.Layout.Validate(); err != nil {
		return nil, err
	}
	for _, name := range art.SideNames {
		if len(name) > 31 {
			return nil, fmt.Errorf("options side label is too long")
		}
		for _, glyph := range []byte(name) {
			if glyph < 32 || glyph > 126 {
				return nil, fmt.Errorf("invalid options side glyph")
			}
		}
	}
	return art, nil
}

func (a *OptionsArt) DrawReaction(dst *image.RGBA, font *Font, reaction int) {
	if a == nil || font == nil || dst == nil {
		return
	}
	reaction = min(max(reaction, 0), 15)
	font.Draw(dst, string([]byte{a.ReactionGlyph}), a.ReactionPosition.X+reaction*8, a.ReactionPosition.Y, a.Layout.Palette)
}

// ActionAt keeps the movable thumb's two click regions in step with the
// displayed reaction value. Other requester actions keep their fixed bounds.
func (a *OptionsArt) ActionAt(x, y, reaction int) string {
	if a == nil {
		return ""
	}
	p := a.ReactionPosition
	if y >= p.Y && y < p.Y+8 && x >= p.X-8 && x < p.X+128 {
		if x < p.X+min(max(reaction, 0), 15)*8 {
			return "reaction-decrease"
		}
		return "reaction-increase"
	}
	return a.Layout.ActionAt(x, y)
}
