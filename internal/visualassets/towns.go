package visualassets

// TownArt contains sprite composition and visual meter scaling. Surrounding
// pieces are relative to their own terrain parcel, not the center's altitude.
type TownArt struct {
	Centers            [19]Frame    `json:"centers"`
	PopulationDivisors [19]uint16   `json:"population_divisors"`
	FlagSprites        [2][2]int    `json:"flag_sprites"`
	FlagHeight         int          `json:"flag_height"`
	Surroundings       [19][8]Frame `json:"surroundings"`
	Offsets            [8][2]int    `json:"offsets"`
}

// CenterLayers selects the animated faction flag and its population height.
// Returned layers belong to the caller and cannot mutate the imported bank.
func (t *TownArt) CenterLayers(stage, owner int, population uint32, tick uint64) []SpriteLayer {
	if t == nil || stage < 0 || stage >= len(t.Centers) || owner < 0 || owner > 1 || t.PopulationDivisors[stage] == 0 {
		return nil
	}
	layers := append([]SpriteLayer(nil), t.Centers[stage].Layers...)
	for i := range layers {
		layer := &layers[i]
		if layer.Sprite != t.FlagSprites[0][0] {
			continue
		}
		quotient := population / uint32(t.PopulationDivisors[stage])
		word := uint16(quotient)
		if quotient > 65535 {
			word = uint16(population)
		}
		height := min(int(int16(word)), t.FlagHeight)
		layer.Y = int(int16(uint16(layer.Y) + uint16(t.FlagHeight-height)))
		layer.Sprite = t.FlagSprites[owner][tick&1]
	}
	return layers
}
