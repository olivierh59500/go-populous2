package visualassets

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"io/fs"
)

const SelectionPanelName = "selected-panel.json"

// SelectionPanel contains only presentation coordinates and decoded artwork.
// The selected group's state and population remain ordinary game data.
type SelectionPanel struct {
	Version          int            `json:"version"`
	ActorX           int            `json:"actor_x"`
	ActorY           int            `json:"actor_y"`
	WeaponX          int            `json:"weapon_x"`
	WeaponY          int            `json:"weapon_y"`
	HitX             int            `json:"hit_x"`
	HitY             int            `json:"hit_y"`
	HitWidth         int            `json:"hit_width"`
	HitHeight        int            `json:"hit_height"`
	PopulationPoints [8]image.Point `json:"population_points"`
	PopulationSprite int            `json:"population_sprite"`
	Weapons          []Frame        `json:"weapons"`
	TownHitHeights   [19]int        `json:"town_hit_heights"`
}

func (p *SelectionPanel) TownHitHeight(stage int) int {
	if p == nil || stage < 0 || stage >= len(p.TownHitHeights) {
		return 0
	}
	return p.TownHitHeights[stage]
}

func (p *SelectionPanel) HitRect() image.Rectangle {
	if p == nil {
		return image.Rectangle{}
	}
	return image.Rect(p.HitX, p.HitY, p.HitX+p.HitWidth, p.HitY+p.HitHeight)
}

// PopulationIndicators expresses each nonzero remaining hexadecimal place as
// a vertical position. The artwork primitive uses these as top-left points,
// unlike actor and weapon sprite layers, which use their imported anchors.
func (p *SelectionPanel) PopulationIndicators(population uint32) []image.Point {
	if p == nil || population == 0 {
		return nil
	}
	points := make([]image.Point, 0, 8)
	for _, anchor := range p.PopulationPoints {
		if population == 0 {
			break
		}
		points = append(points, image.Pt(anchor.X, anchor.Y-int(population&15)))
		population >>= 4
	}
	return points
}

func LoadSelectionPanel(files fs.FS) (*SelectionPanel, error) {
	f, err := files.Open(SelectionPanelName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var panel SelectionPanel
	if err := decoder.Decode(&panel); err != nil {
		return nil, fmt.Errorf("selected panel metadata: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("selected panel metadata has trailing data")
	}
	if panel.Version != 1 || panel.ActorX < 0 || panel.ActorX >= 320 || panel.ActorY < 0 || panel.ActorY >= 200 || panel.WeaponX < 0 || panel.WeaponX >= 320 || panel.WeaponY < 0 || panel.WeaponY >= 200 || panel.HitWidth <= 0 || panel.HitHeight <= 0 || panel.HitX < 0 || panel.HitY < 0 || panel.HitRect().Max.X > 320 || panel.HitRect().Max.Y > 200 || panel.PopulationSprite < 0 || panel.PopulationSprite > 4095 || len(panel.Weapons) != 19 {
		return nil, fmt.Errorf("selected panel metadata is invalid")
	}
	for _, point := range panel.PopulationPoints {
		if point.X < 0 || point.X >= 320 || point.Y < 0 || point.Y >= 200 {
			return nil, fmt.Errorf("selected panel population point is invalid")
		}
	}
	for _, height := range panel.TownHitHeights {
		if height < 0 || height > 200 {
			return nil, fmt.Errorf("selected panel town hit height is invalid")
		}
	}
	for _, frame := range panel.Weapons {
		if len(frame.Layers) == 0 || len(frame.Layers) > 64 {
			return nil, fmt.Errorf("selected panel weapon frame is invalid")
		}
		for _, layer := range frame.Layers {
			if layer.Sprite < 0 || layer.Sprite > 4095 || layer.X < -320 || layer.X > 320 || layer.Y < -200 || layer.Y > 200 {
				return nil, fmt.Errorf("selected panel weapon layer is invalid")
			}
		}
	}
	return &panel, nil
}
