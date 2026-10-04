package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// SpriteLayer preserves one entry of the composite image table at CODE:$26956.
// Its signed offsets apply before the descriptor's own image anchor.
type SpriteLayer struct {
	Sprite int
	X, Y   int
}

type HeroArt struct {
	Directions [8][][]SpriteLayer
}

type HeroRules struct {
	Art [6]HeroArt
}

var heroIDs = [6]SpellID{Perseus, Adonis, Heracles, Odysseus, Achilles, Helen}

func heroIndex(id SpellID) int {
	for i, hero := range heroIDs {
		if id == hero {
			return i
		}
	}
	return -1
}

// DecodeHeroRules follows the walking-animation pointers selected by the
// native conversion routine at $142d4. Animation words address six-byte
// composite-image entries, not sprite IDs, so dividing them by twelve would
// select the wrong images and lose additional layers.
func DecodeHeroRules(exe *amiga.Executable) (HeroRules, error) {
	var result HeroRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x26956 {
		return result, fmt.Errorf("Populous II hero animation tables missing")
	}
	code := exe.Hunks[0].Data
	for hero := range result.Art {
		start := int(binary.BigEndian.Uint16(code[0x20a00+hero*2:]))
		for direction := range result.Art[hero].Directions {
			at := 0x23d1a + start
			terminated := false
			for step := 0; step < 128; step++ {
				if at+4 > len(code) {
					return HeroRules{}, fmt.Errorf("hero %d animation exceeds CODE", hero)
				}
				word := int16(binary.BigEndian.Uint16(code[at:]))
				at += 4
				if word < 0 {
					terminated = true
					break
				}
				layers, err := decodeImageLayers(code, uint16(word))
				if err != nil {
					return HeroRules{}, fmt.Errorf("hero %d direction %d: %w", hero, direction, err)
				}
				result.Art[hero].Directions[direction] = append(result.Art[hero].Directions[direction], layers)
			}
			if !terminated || len(result.Art[hero].Directions[direction]) == 0 {
				return HeroRules{}, fmt.Errorf("empty hero animation")
			}
			start = at - 0x23d1a
		}
	}
	return result, nil
}

func decodeImageLayers(code []byte, image uint16) ([]SpriteLayer, error) {
	layers := make([]SpriteLayer, 0, 3)
	seen := make(map[uint16]bool)
	for {
		if seen[image] || len(layers) >= 256 {
			return nil, fmt.Errorf("cyclic composite image %d", image)
		}
		seen[image] = true
		at := 0x26956 + int(image)*2
		if at+6 > len(code) {
			return nil, fmt.Errorf("composite image %d exceeds CODE", image)
		}
		sprite := int(binary.BigEndian.Uint16(code[at+2:]))
		if sprite%12 != 0 || sprite/12 >= 830 {
			return nil, fmt.Errorf("invalid sprite descriptor offset %d", sprite)
		}
		layers = append(layers, SpriteLayer{Sprite: sprite / 12, X: int(int8(code[at])), Y: int(int8(code[at+1]))})
		image = binary.BigEndian.Uint16(code[at+4:])
		if image == 0 {
			return layers, nil
		}
	}
}

func (rules HeroRules) Layers(id SpellID, direction, frame int) []SpriteLayer {
	i := heroIndex(id)
	if i < 0 || direction < 0 || direction >= 8 || frame < 0 {
		return nil
	}
	frames := rules.Art[i].Directions[direction]
	if len(frames) == 0 {
		return nil
	}
	return frames[frame%len(frames)]
}

// HeroAttributes translates creation at $14360-$14408. Heracles doubles the
// leader's population. Every hero gains elemental experience >>3 in movement
// speed; Odysseus also doubles the leader's original speed. No weapons bonus
// is applied by this routine.
func HeroAttributes(id SpellID, population int, speed uint8, experience [6]uint8) (int, uint8, bool) {
	i := heroIndex(id)
	if i < 0 || population <= 0 {
		return population, speed, false
	}
	if id == Heracles {
		population *= 2
	}
	bonus := int(experience[i] >> 3)
	if id == Odysseus {
		bonus += int(speed)
	}
	return population, uint8(min(255, int(speed)+bonus)), true
}
