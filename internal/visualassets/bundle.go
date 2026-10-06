// Package visualassets loads ordinary images and animation metadata used by
// the Go game. Import-time formats and the original program are not required.
package visualassets

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io/fs"
)

const (
	SchemaVersion  = 1
	LandscapeCount = 4
	CatalogName    = "visuals.json"
)

// Region selects artwork from a PNG atlas. Anchors are measured in pixels
// from its top-left corner to the actor's position on the terrain.
type Region struct {
	Atlas   string `json:"atlas"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	AnchorX int    `json:"anchor_x,omitempty"`
	AnchorY int    `json:"anchor_y,omitempty"`
}

type Sprite struct {
	Image            *image.RGBA
	AnchorX, AnchorY int
}

type SpriteLayer struct {
	Sprite int `json:"sprite"`
	X      int `json:"x,omitempty"`
	Y      int `json:"y,omitempty"`
}

// Frame describes artwork composition, not an executable instruction or
// memory location. The optional cue is a sound's position in the audio bank.
type Frame struct {
	Layers   []SpriteLayer `json:"layers"`
	SoundCue int           `json:"sound_cue,omitempty"`
}

type Animation struct {
	Frames []Frame `json:"frames"`
	Loop   bool    `json:"loop"`
}

type Landscape struct {
	Tiles     []Region        `json:"tiles"`
	Sprites   []Region        `json:"sprites"`
	Palette   [16]color.RGBA  `json:"palette"`
	MapColors [256]color.RGBA `json:"map_colors"`
}

type FontDescriptor struct {
	Atlas       string `json:"atlas"`
	FirstCode   int    `json:"first_code"`
	Count       int    `json:"count"`
	GlyphWidth  int    `json:"glyph_width"`
	GlyphHeight int    `json:"glyph_height"`
	Columns     int    `json:"columns"`
}

// Catalog is the portable import result. All paths are relative to its root;
// its fields contain only graphics, color palettes and animation composition.
type Catalog struct {
	Version        int                       `json:"version"`
	Landscapes     [LandscapeCount]Landscape `json:"landscapes"`
	Background     string                    `json:"background"`
	Startup        string                    `json:"startup"`
	StartupPalette [16]color.RGBA            `json:"startup_palette"`
	Ending         string                    `json:"ending,omitempty"`
	PortraitParts  [3][8]Region              `json:"portrait_parts"`
	Font           FontDescriptor            `json:"font"`
	Animations     map[string]Animation      `json:"animations,omitempty"`
	Towns          *TownArt                  `json:"towns,omitempty"`
	EndingSequence *EndingDescriptor         `json:"ending_sequence,omitempty"`
	TileRasters    [256]uint8                `json:"tile_rasters"`
}

type Bundle struct {
	Tiles                       [LandscapeCount][]*image.RGBA
	Sprites                     [LandscapeCount][]Sprite
	Palettes                    [LandscapeCount][16]color.RGBA
	MapColors                   [LandscapeCount][256]color.RGBA
	Background, Startup, Ending *image.RGBA
	StartupPalette              [16]color.RGBA
	PortraitParts               [3][8]*image.RGBA
	Font                        *Font
	Animations                  map[string]Animation
	Towns                       *TownArt
	EndingSequence              *EndingSequence
	TileRasters                 [256]uint8
}

// Portrait assembles independently selectable headpiece, eyes and mouth.
func (b *Bundle) Portrait(parts [3]uint8) (*image.RGBA, error) {
	if b == nil {
		return nil, fmt.Errorf("visual bundle missing")
	}
	portrait := image.NewRGBA(image.Rect(0, 0, 32, 96))
	for part := len(parts) - 1; part >= 0; part-- {
		if int(parts[part]) >= len(b.PortraitParts[part]) {
			return nil, fmt.Errorf("invalid portrait variant %d", parts[part])
		}
		strip := b.PortraitParts[part][parts[part]]
		if strip == nil {
			return nil, fmt.Errorf("portrait artwork missing")
		}
		y := part*16 + max(0, 16-strip.Bounds().Dy())
		draw.Draw(portrait, image.Rect(0, y, 32, y+strip.Bounds().Dy()), strip, image.Point{}, draw.Over)
	}
	return portrait, nil
}

// LoadFS reads a locally generated installation containing no game program.
func LoadFS(files fs.FS) (*Bundle, error) {
	return loadFS(files)
}
