package visualassets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"io/fs"
)

const PointerFile = "pointers.json"

type PointerDescriptor struct {
	MapOutlines     [16][4][2]int       `json:"map_outlines"`
	Frames          map[string][]Region `json:"frames"`
	MapMarkerSprite int                 `json:"map_marker_sprite"`
}
type PointerArt struct {
	Frames          map[string][]Sprite
	MapMarkerSprite int
	MapOutlines     [16][4][2]int
}

func LoadPointers(files fs.FS) (*PointerArt, error) {
	data, err := readLimited(files, PointerFile, 65536)
	if err != nil {
		return nil, err
	}
	var descriptor PointerDescriptor
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&descriptor); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("pointer data has trailing content")
	}
	if len(descriptor.Frames) == 0 || len(descriptor.Frames) > 40 || descriptor.MapMarkerSprite < 0 || descriptor.MapMarkerSprite >= 4096 {
		return nil, fmt.Errorf("invalid pointer artwork catalog")
	}
	for _, vertices := range descriptor.MapOutlines {
		for _, point := range vertices {
			if point[0] < -64 || point[0] > 64 || point[1] < -64 || point[1] > 64 {
				return nil, fmt.Errorf("map pointer outline exceeds tile geometry")
			}
		}
	}
	loader := imageLoader{files: files, images: make(map[string]image.Image)}
	result := &PointerArt{Frames: make(map[string][]Sprite), MapMarkerSprite: descriptor.MapMarkerSprite, MapOutlines: descriptor.MapOutlines}
	for name, frames := range descriptor.Frames {
		if name == "" || len(name) > 48 || len(frames) < 1 || len(frames) > 4 {
			return nil, fmt.Errorf("invalid named pointer sequence")
		}
		for _, region := range frames {
			if region.Width != 16 || region.Height != 16 {
				return nil, fmt.Errorf("pointer frame must be16x16")
			}
			img, err := loader.region(region, false)
			if err != nil {
				return nil, err
			}
			result.Frames[name] = append(result.Frames[name], Sprite{Image: img, AnchorX: region.AnchorX, AnchorY: region.AnchorY})
		}
	}
	if len(result.Frames["normal"]) == 0 || len(result.Frames["forbidden"]) == 0 {
		return nil, fmt.Errorf("normal terrain pointer art missing")
	}
	return result, nil
}
