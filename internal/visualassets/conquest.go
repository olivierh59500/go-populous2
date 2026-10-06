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

const ConquestArtName = "conquest.json"

type OpponentInfo struct {
	Name, Realm string
	Biography   [5]string
	FaceParts   [3]uint8
}

// ConquestDescriptor contains presentation data for the world briefing and
// its opponent page. Original template parsing occurs only during export.
type ConquestDescriptor struct {
	Version                int
	Layout, OpponentLayout RequesterLayout
	LandscapeNames         [4]string
	Opponents              [32]OpponentInfo
	SpeedLabels            [15]string
	AggressionLabels       [8]string
	Icons                  [36]Region
	FaceBackdrop           Region
	FaceBackdropPosition   image.Point
	FacePositions          [3]image.Point
}

type ConquestArt struct {
	Descriptor   ConquestDescriptor
	Icons        [36]*image.RGBA
	FaceBackdrop *image.RGBA
}

func (a *ConquestArt) OpponentValues(world, reaction, aggression int) map[string]string {
	d := &a.Descriptor
	stage := min(max(world/32, 0), len(d.Opponents)-1)
	info := d.Opponents[stage]
	if reaction < 0 || reaction >= len(d.SpeedLabels) {
		reaction = 12
	}
	behavior := min(max(aggression/5, 0), len(d.AggressionLabels)-1)
	values := map[string]string{"name": info.Name, "realm": info.Realm, "speed": d.SpeedLabels[reaction], "aggression": d.AggressionLabels[behavior]}
	for i, line := range info.Biography {
		values[fmt.Sprintf("biography-%d", i)] = line
	}
	return values
}

func LoadConquestArt(files fs.FS) (*ConquestArt, error) {
	data, err := readLimited(files, ConquestArtName, 1<<20)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	art := &ConquestArt{}
	if err := decoder.Decode(&art.Descriptor); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("conquest metadata has trailing data")
	}
	d := &art.Descriptor
	if d.Version != 1 || d.Layout.Name != "conquest" || d.OpponentLayout.Name != "opponent" {
		return nil, fmt.Errorf("invalid conquest presentation version or role")
	}
	if err := d.Layout.Validate(); err != nil {
		return nil, err
	}
	if err := d.OpponentLayout.Validate(); err != nil {
		return nil, err
	}
	validText := func(text string, limit int) bool {
		if len(text) > limit {
			return false
		}
		for _, c := range []byte(text) {
			if c < 32 || c > 126 {
				return false
			}
		}
		return true
	}
	for _, text := range d.LandscapeNames {
		if !validText(text, 20) {
			return nil, fmt.Errorf("invalid landscape label")
		}
	}
	for _, info := range d.Opponents {
		if !validText(info.Name, 15) || !validText(info.Realm, 30) {
			return nil, fmt.Errorf("invalid opponent label")
		}
		for _, text := range info.Biography {
			if !validText(text, 38) {
				return nil, fmt.Errorf("invalid opponent biography")
			}
		}
		for _, part := range info.FaceParts {
			if part > 7 {
				return nil, fmt.Errorf("invalid opponent portrait")
			}
		}
	}
	for _, text := range d.SpeedLabels {
		if !validText(text, 20) {
			return nil, fmt.Errorf("invalid speed label")
		}
	}
	for _, text := range d.AggressionLabels {
		if !validText(text, 20) {
			return nil, fmt.Errorf("invalid aggression label")
		}
	}
	loader := imageLoader{files: files, images: make(map[string]image.Image)}
	for slot, region := range d.Icons {
		if region.Width != 32 || region.Height < 1 || region.Height > 40 {
			return nil, fmt.Errorf("invalid conquest power icon %d", slot)
		}
		art.Icons[slot], err = loader.region(region, false)
		if err != nil {
			return nil, err
		}
	}
	if d.FaceBackdrop.Width != 48 || d.FaceBackdrop.Height != 64 {
		return nil, fmt.Errorf("invalid opponent portrait backdrop")
	}
	art.FaceBackdrop, err = loader.region(d.FaceBackdrop, false)
	if err != nil {
		return nil, err
	}
	if !image.Rect(d.FaceBackdropPosition.X, d.FaceBackdropPosition.Y, d.FaceBackdropPosition.X+48, d.FaceBackdropPosition.Y+64).In(image.Rect(0, 0, 320, 200)) {
		return nil, fmt.Errorf("opponent backdrop position outside display")
	}
	for _, p := range d.FacePositions {
		if p.X < 0 || p.Y < 0 || p.X+32 > 320 || p.Y+32 > 200 {
			return nil, fmt.Errorf("opponent portrait position outside display")
		}
	}
	return art, nil
}

// PowerAt is the original isometric icon lattice. The sixth column in each
// elemental row has no selectable power, even though its icon slot exists.
func (a *ConquestArt) PowerAt(x, y int) (int, bool) {
	if a == nil || x < 0 || y < 0 || x >= 320 || y >= 200 {
		return 0, false
	}
	dx, dy := (x-126)>>1, y+10
	column, row := (dx+dy)>>4, (dy-dx)>>4
	column = column - 5 + row
	if row < 0 || row >= 6 || column < 0 || column >= 5 {
		return 0, false
	}
	return (5-row)*6 + column, true
}

func (a *ConquestArt) DrawIcons(dst *image.RGBA, enabled [36]bool) {
	if a == nil || dst == nil {
		return
	}
	for slot, visible := range enabled {
		if !visible || a.Icons[slot] == nil {
			continue
		}
		p := image.Pt(30+(slot/6)*32+(slot%6)*16, 30+(slot%6)*8)
		draw.Draw(dst, a.Icons[slot].Bounds().Add(p), a.Icons[slot], image.Point{}, draw.Over)
	}
}

func (a *ConquestArt) DrawOpponentFace(dst *image.RGBA, stage int, portraits [3][8]*image.RGBA) {
	if a == nil || dst == nil || stage < 0 || stage >= len(a.Descriptor.Opponents) {
		return
	}
	d := &a.Descriptor
	if a.FaceBackdrop != nil {
		draw.Draw(dst, a.FaceBackdrop.Bounds().Add(d.FaceBackdropPosition), a.FaceBackdrop, image.Point{}, draw.Src)
	}
	for part := 2; part >= 0; part-- {
		img := portraits[part][d.Opponents[stage].FaceParts[part]]
		if img == nil {
			continue
		}
		p := d.FacePositions[part]
		p.Y += max(0, 16-img.Bounds().Dy())
		draw.Draw(dst, img.Bounds().Add(p), img, image.Point{}, draw.Over)
	}
}
