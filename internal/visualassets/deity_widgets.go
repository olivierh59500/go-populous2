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

const DeityWidgetsName = "deity-widgets.json"

type DeityWidgetsDescriptor struct {
	Bars          [24]Region     `json:"bars"`
	FaceBackdrop  Region         `json:"face_backdrop"`
	FacePositions [3]image.Point `json:"face_positions"`
	PortraitParts [3][8]Region   `json:"portrait_parts"`
}

type DeityWidgets struct {
	Bars          [24]*image.RGBA
	FaceBackdrop  *image.RGBA
	FacePositions [3]image.Point
	PortraitParts [3][8]*image.RGBA
}

func LoadDeityWidgets(files fs.FS) (*DeityWidgets, error) {
	data, err := readLimited(files, DeityWidgetsName, 1<<16)
	if err != nil {
		return nil, err
	}
	var descriptor DeityWidgetsDescriptor
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&descriptor); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("deity widget metadata has trailing data")
	}
	loader := imageLoader{files: files, images: make(map[string]image.Image)}
	widgets := &DeityWidgets{FacePositions: descriptor.FacePositions}
	for i, region := range descriptor.Bars {
		if region.Width != 16 || region.Height != 32 {
			return nil, fmt.Errorf("deity bar must be 16x32")
		}
		widgets.Bars[i], err = loader.region(region, false)
		if err != nil {
			return nil, err
		}
	}
	if descriptor.FaceBackdrop.Width != 48 || descriptor.FaceBackdrop.Height != 64 {
		return nil, fmt.Errorf("deity face backdrop must be 48x64")
	}
	widgets.FaceBackdrop, err = loader.region(descriptor.FaceBackdrop, false)
	if err != nil {
		return nil, err
	}
	for _, p := range widgets.FacePositions {
		if p.X < 0 || p.Y < 0 || p.X+32 > 320 || p.Y+64 > 200 {
			return nil, fmt.Errorf("deity face position is invalid")
		}
	}
	for part := range descriptor.PortraitParts {
		for variant, region := range descriptor.PortraitParts[part] {
			if region.Width != 32 || region.Height < 1 || region.Height > 64 {
				return nil, fmt.Errorf("deity portrait dimensions are invalid")
			}
			widgets.PortraitParts[part][variant], err = loader.region(region, false)
			if err != nil {
				return nil, err
			}
		}
	}
	return widgets, nil
}

// Draw preserves the original two-nibble experience strips and opaque face
// backing. Portrait variants retain their independently imported dimensions.
func (w *DeityWidgets) Draw(dst *image.RGBA, experience [6]uint8, parts [3]uint8, portraits [3][8]*image.RGBA) {
	if w == nil || dst == nil {
		return
	}
	if w.FaceBackdrop != nil {
		draw.Draw(dst, image.Rect(240, 40, 288, 104), w.FaceBackdrop, image.Point{}, draw.Src)
	}
	for category, value := range experience {
		x, y := 32+(category%3)*48, 32+(category/3)*40
		for half, height := range []int{int(value>>4) * 2, int(value&15) * 2} {
			bright, grey := w.Bars[category*2+half], w.Bars[category*2+half+12]
			if bright == nil || grey == nil {
				continue
			}
			for row := 0; row < 32; row++ {
				img := grey
				if row >= 32-height {
					img = bright
				}
				draw.Draw(dst, image.Rect(x+half*16, y+row, x+half*16+16, y+row+1), img, image.Pt(0, row), draw.Src)
			}
		}
	}
	for part := 2; part >= 0; part-- {
		if parts[part] > 7 {
			continue
		}
		img := w.PortraitParts[part][parts[part]]
		if img == nil {
			img = portraits[part][parts[part]]
		}
		if img == nil {
			continue
		}
		p := w.FacePositions[part]
		p.Y += max(0, 16-img.Bounds().Dy())
		draw.Draw(dst, img.Bounds().Add(p), img, image.Point{}, draw.Over)
	}
}
