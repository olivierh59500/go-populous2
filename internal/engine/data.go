// Package engine implements the Populous II simulation in ordinary Go values.
// Its inputs are decoded campaign and landscape resources, never an executable.
package engine

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"strings"
)

const (
	MapSize          = 64
	CornerSize       = MapSize + 1
	TownStages       = 19
	FollowerCapacity = 400 // Slot zero is the empty occupancy marker.
	SimulationHz     = 12.5
)

// PlayerOptions gives names to the campaign fields needed by the simulation.
// Remaining options are retained as data for subsequent scenario features.
type PlayerOptions struct {
	Scenario                                     ScenarioOptions
	Groups, Population, Weapons, Mana, Attrition int
	MovementSpeed                                uint8
	Powers                                       [36]bool
	Extra                                        [5]uint16
}

type Level struct {
	Number             int
	Code               string
	Landscape          int
	Seed               uint32
	Players            [2]PlayerOptions
	WorldParameters    [60]byte
	OpponentText       string
	OpponentExperience [6]uint8
}

var worldSyllables = [...]string{
	"AA", "AB", "AC", "AD", "AK", "AF", "AG", "AT", "AM", "AL", "EM", "ME", "HE", "FE", "LE", "PE",
	"ET", "ER", "NE", "EG", "IT", "II", "SI", "PI", "TI", "IS", "IM", "IN", "WI", "JI", "SO", "DO",
	"MO", "LO", "HO", "OM", "OW", "OO", "WO", "OP", "UX", "UM", "UN", "UB", "UP", "UG", "UH", "QU",
	"SU", "TU", "TT", "NG", "CC", "MM", "MN", "VE", "NE", "UX", "LY", "DD", "LL", "GH", "TH", "LD",
}

func CodeForLevel(number int) string {
	if number < 0 {
		number = 0
	}
	code := uint16(uint32(uint16(number))*0x24a1+0x24df) & 0x7fff
	var result strings.Builder
	for code != 0 {
		result.WriteString(worldSyllables[code&63])
		code >>= 6
	}
	return result.String()
}

func DecodeLevelCode(input string) (int, bool) {
	input = strings.ToUpper(strings.TrimSpace(input))
	for number := 0; number < 1000; number++ {
		if CodeForLevel(number) == input {
			return number, true
		}
	}
	return 0, false
}

// DecodeCampaign reads the 200 campaign records, each describing five worlds.
func DecodeCampaign(data []byte) ([]Level, error) {
	const recordSize = 250
	if len(data) != 200*recordSize {
		return nil, fmt.Errorf("campaign size %d; expected 50000", len(data))
	}
	result := make([]Level, 1000)
	for i := range result {
		record := data[i/5*recordSize : (i/5+1)*recordSize]
		level := Level{Number: i, Code: CodeForLevel(i), Landscape: int(binary.BigEndian.Uint16(record[182:])), Seed: binary.BigEndian.Uint32(record[182:]) + uint32(i%5)*0x2d7}
		if level.Landscape > 3 {
			return nil, fmt.Errorf("world %d has invalid landscape %d", i, level.Landscape)
		}
		for owner := range level.Players {
			p := record[owner*58 : owner*58+58]
			options := &level.Players[owner]
			options.Groups = int(binary.BigEndian.Uint16(p))
			options.Population = int(binary.BigEndian.Uint16(p[2:]))
			options.MovementSpeed = uint8(binary.BigEndian.Uint16(p[4:]))
			options.Weapons = int(uint8(binary.BigEndian.Uint16(p[6:])))
			options.Mana = int(binary.BigEndian.Uint16(p[8:]))
			options.Attrition = int(binary.BigEndian.Uint16(p[10:]))
			options.Scenario = decodeScenario(binary.BigEndian.Uint16(p[12:]))
			for j := range options.Extra {
				options.Extra[j] = binary.BigEndian.Uint16(p[12+j*2:])
			}
			for j := range options.Powers {
				if p[22+j] > 1 {
					return nil, fmt.Errorf("world %d: invalid power flag", i)
				}
				options.Powers[j] = p[22+j] != 0
			}
		}
		copy(level.OpponentExperience[:], record[116:122])
		copy(level.WorldParameters[:], record[122:182])
		level.OpponentText = strings.TrimSpace(strings.SplitN(string(record[186:]), "\x00", 2)[0])
		result[i] = level
	}
	return result, nil
}

type Landscape struct {
	ManaAdd, PopulationAdd, PopulationLimit, EmigrationDivisor, Weapons, WorkTicks [TownStages]int
	Parameters                                                                     [3]int
	MapColor                                                                       [256]uint8
	Palettes                                                                       [2][16]color.RGBA
	LastWord                                                                       uint16
}

func DecodeLandscape(data []byte) (Landscape, error) {
	var land Landscape
	if len(data) != 556 {
		return land, fmt.Errorf("landscape size %d; expected 556", len(data))
	}
	offset := 0
	for _, table := range []*[TownStages]int{&land.ManaAdd, &land.PopulationAdd, &land.PopulationLimit, &land.EmigrationDivisor, &land.Weapons, &land.WorkTicks} {
		for i := range table {
			table[i] = int(int16(binary.BigEndian.Uint16(data[offset:])))
			offset += 2
		}
	}
	for i := range land.Parameters {
		land.Parameters[i] = int(int16(binary.BigEndian.Uint16(data[offset:])))
		offset += 2
	}
	copy(land.MapColor[:], data[offset:offset+256])
	offset += 256
	for p := range land.Palettes {
		for i := range land.Palettes[p] {
			c := binary.BigEndian.Uint16(data[offset:])
			offset += 2
			if c > 0xfff {
				return Landscape{}, fmt.Errorf("invalid palette colour %x", c)
			}
			land.Palettes[p][i] = color.RGBA{uint8(c>>8&15) * 17, uint8(c>>4&15) * 17, uint8(c&15) * 17, 255}
		}
	}
	land.LastWord = binary.BigEndian.Uint16(data[offset:])
	return land, nil
}

// ScenarioOptions controls the campaign's construction, visibility and
// terrain hazards. These booleans replace the resource's packed option word.
type ScenarioOptions struct {
	BuildAnywhere, SeaLevelOnly, ForbidEnemyTerrain, ForbidRaise, ForbidLower bool
	FatalWater, HideEnemy, DisableEmigration, HideDisasters, ShallowSwamps    bool
}

func decodeScenario(word uint16) ScenarioOptions {
	return ScenarioOptions{BuildAnywhere: word&1 != 0, SeaLevelOnly: word&2 != 0, ForbidEnemyTerrain: word&4 != 0, ForbidRaise: word&8 != 0, ForbidLower: word&16 != 0, FatalWater: word&32 != 0, HideEnemy: word&64 != 0, DisableEmigration: word&128 != 0, HideDisasters: word&256 != 0, ShallowSwamps: word&512 != 0}
}
