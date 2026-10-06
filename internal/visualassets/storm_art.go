package visualassets

import "image"

// ShortenedSpriteDescriptor contains predecoded artwork for the original
// height-dependent strike images. A height selects an ordinary PNG region;
// no plane-stride reinterpretation or original program is needed at runtime.
type ShortenedSpriteDescriptor struct {
	Sprite  int      `json:"sprite"`
	Heights []Region `json:"heights"`
}
type ShortenedSpriteArt struct {
	Sprite  int
	Heights []*image.RGBA
}

func (b *Bundle) StormStrikeImage(land, sprite, height int) *image.RGBA {
	if b == nil || land < 0 || land >= LandscapeCount || height <= 0 {
		return nil
	}
	for _, art := range b.StormStrikeArt[land] {
		if art.Sprite == sprite && height <= len(art.Heights) {
			return art.Heights[height-1]
		}
	}
	return nil
}
