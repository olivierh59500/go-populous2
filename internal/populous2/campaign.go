package populous2

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const CampaignWorlds = 1000
const CampaignRecordSize = 250

// PlayerOptions preserves the 58-byte per-player template copied by 0x10df2:
// eleven words, then six groups of six power flags (the last slot is reserved).
// Fields whose semantics are not yet established retain their on-disk values.
type PlayerOptions struct {
	Parameters [11]uint16
	Powers     [36]bool
}

// MovementSpeed is the byte copied from deity+$5f into follower+$12 during
// native allocation at CODE:$10d3c. The template starts at deity+$5a.
func (p PlayerOptions) MovementSpeed() uint8 { return uint8(p.Parameters[2]) }

func (p PlayerOptions) InitialGroups() int      { return int(p.Parameters[0]) }
func (p PlayerOptions) InitialPopulation() int  { return int(p.Parameters[1]) }
func (p PlayerOptions) SearchIntelligence() int { return int(uint8(p.Parameters[3])) }

// InitialMana and FollowerAttrition are the zero-extended template words
// copied to deity+$02 and deity+$16 at CODE:$10b6a/$10b70 (and $10c08/$10c0e).
// Mana is already in native ledger units; water and land use the same
// attrition longword.
func (p PlayerOptions) InitialMana() int       { return int(p.Parameters[4]) }
func (p PlayerOptions) FollowerAttrition() int { return int(p.Parameters[5]) }

type Level struct {
	Number             int
	Code               string
	Terrain            int
	Seed               uint16
	RandomSeed         uint32
	Players            [2]PlayerOptions
	WorldParameters    [60]byte
	OpponentText       string
	OpponentExperience [6]uint8
	Raw                [CampaignRecordSize]byte
}

var worldSyllables = [64]string{
	"AA", "AB", "AC", "AD", "AK", "AF", "AG", "AT", "AM", "AL", "EM", "ME", "HE", "FE", "LE", "PE",
	"ET", "ER", "NE", "EG", "IT", "II", "SI", "PI", "TI", "IS", "IM", "IN", "WI", "JI", "SO", "DO",
	"MO", "LO", "HO", "OM", "OW", "OO", "WO", "OP", "UX", "UM", "UN", "UB", "UP", "UG", "UH", "QU",
	"SU", "TU", "TT", "NG", "CC", "MM", "MN", "VE", "NE", "UX", "LY", "DD", "LL", "GH", "TH", "LD",
}

// CodeForLevel translates 0x103c6: the same 15-bit LCG as Populous 1, followed
// by Populous II's own table of two-letter syllables, least-significant first.
func CodeForLevel(number int) string {
	if number < 0 {
		number = 0
	}
	code := uint16(uint32(uint16(number*5))*0x24a1+0x24df) & 0x7fff
	var result strings.Builder
	for code != 0 {
		result.WriteString(worldSyllables[code&63])
		code >>= 6
	}
	return result.String()
}

func DecodeLevelCode(input string) (int, bool) {
	clean := strings.ToUpper(strings.TrimSpace(input))
	for number := 0; number < CampaignWorlds; number++ {
		if CodeForLevel(number) == clean {
			return number, true
		}
	}
	return 0, false
}

// DecodeCampaign translates 0x11044/0x10df2: 200 records, five worlds per record,
// with seed increment 0x2d7 for each subworld. This is not a list of 50-byte maps.
func DecodeCampaign(data []byte) ([]Level, error) {
	if len(data) != CampaignWorlds/5*CampaignRecordSize {
		return nil, fmt.Errorf("conquest data has %d bytes; expected 50000", len(data))
	}
	result := make([]Level, CampaignWorlds)
	for i := range result {
		record := data[(i/5)*CampaignRecordSize : (i/5+1)*CampaignRecordSize]
		level := Level{Number: i, Code: CodeForLevel(i), Terrain: int(binary.BigEndian.Uint16(record[182:184])), Seed: uint16(uint32(binary.BigEndian.Uint16(record[184:186])) + uint32(i%5)*0x2d7)}
		level.RandomSeed = binary.BigEndian.Uint32(record[182:186]) + uint32(i%5)*0x2d7
		if level.Terrain < 0 || level.Terrain > 3 {
			return nil, fmt.Errorf("world %d has unsupported landscape %d", i, level.Terrain)
		}
		for player := range level.Players {
			options := &level.Players[player]
			p := player * 58
			for n := range options.Parameters {
				options.Parameters[n] = binary.BigEndian.Uint16(record[p+n*2:])
			}
			for n := range options.Powers {
				v := record[p+22+n]
				if v > 1 {
					return nil, fmt.Errorf("world %d player %d power %d has nonboolean flag %d", i, player, n, v)
				}
				options.Powers[n] = v == 1
			}
		}
		copy(level.WorldParameters[:], record[122:182])
		// CODE:$10e58 copies this shared opponent profile to deity+$52.
		copy(level.OpponentExperience[:], record[116:122])
		text := record[186:]
		if end := strings.IndexByte(string(text), 0); end >= 0 {
			text = text[:end]
		}
		level.OpponentText = strings.TrimSpace(string(text))
		copy(level.Raw[:], record)
		result[i] = level
	}
	return result, nil
}
