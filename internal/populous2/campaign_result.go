package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type CampaignResultRules struct {
	ScoreBase                              uint32
	PowerWeight, BoltDivisor, WorldDivisor uint16
	BoltCap, WorldStepCap, LastWorld       uint16
}

func DecodeCampaignResultRules(exe *amiga.Executable) (CampaignResultRules, error) {
	var r CampaignResultRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3aa8 {
		return r, fmt.Errorf("native campaign result constants missing")
	}
	b := exe.Hunks[0].Data
	r.ScoreBase = binary.BigEndian.Uint32(b[0x3980:])
	r.PowerWeight = binary.BigEndian.Uint16(b[0x39b8:])
	r.BoltDivisor = binary.BigEndian.Uint16(b[0x3a32:])
	r.WorldDivisor = binary.BigEndian.Uint16(b[0x3a56:])
	r.BoltCap = binary.BigEndian.Uint16(b[0x3a40:])
	r.WorldStepCap = binary.BigEndian.Uint16(b[0x3a5e:])
	r.LastWorld = binary.BigEndian.Uint16(b[0x3a76:])
	if r.BoltDivisor == 0 || r.WorldDivisor == 0 {
		return CampaignResultRules{}, fmt.Errorf("native campaign result divisor zero")
	}
	return r, nil
}

type CampaignResultStatistics struct {
	Population, PeakPopulation, PeakMana                                                 uint32
	Identity, Metric, LeaderLosses, BattleWins, ScenarioOptions, WeightedPowerUse, Bolts uint16
}

func ReadCampaignResultStatistics(owner uint8, m FollowerCleanupMemory) (CampaignResultStatistics, error) {
	var s CampaignResultStatistics
	if owner > 2 || m.Read16 == nil || m.Read32 == nil {
		return s, fmt.Errorf("native campaign statistics memory missing")
	}
	a := 0xe76a + int(owner)*314
	for _, f := range []struct {
		Offset int
		Dest   *uint32
	}{{4, &s.Population}, {0x3c, &s.PeakPopulation}, {0x40, &s.PeakMana}} {
		v, err := m.Read32(a + f.Offset)
		if err != nil {
			return s, err
		}
		*f.Dest = v
	}
	for _, f := range []struct {
		Offset int
		Dest   *uint16
	}{{0x18, &s.Identity}, {0x44, &s.Metric}, {0x46, &s.LeaderLosses}, {0x48, &s.BattleWins}, {0x4a, &s.ScenarioOptions}, {0x138, &s.WeightedPowerUse}, {0x58, &s.Bolts}} {
		v, err := m.Read16(a + f.Offset)
		if err != nil {
			return s, err
		}
		*f.Dest = v
	}
	return s, nil
}

// Detect mirrors $12470/$1247a: editor mode8 has no automatic result, and
// exact zero population is tested in side1/side2 order. Identity+$18 follows
// profile/control swaps; it cannot be reconstructed from the physical side.
func (r CampaignResultRules) Detect(mode uint16, sides [3]CampaignResultStatistics) (uint16, bool) {
	if mode == 8 {
		return 0, false
	}
	for side := 1; side <= 2; side++ {
		if sides[side].Population == 0 {
			return sides[side].Identity, true
		}
	}
	return 0, false
}

type CampaignScore struct {
	Value          uint16
	DivideOverflow bool
	RatioWord      uint16
}

// Score translates $397e..$39c2. The raw long+$48 combines battle wins and
// scenario options; the opposing options word is the DIVU divisor. ADDI.W1
// preserves the product's high word without propagating a low-word carry.
// Original quotient overflow preserves the dividend, whose low word remains
// the ratio contribution. A zero divisor is an explicit native arithmetic
// fault, not a guessed nonzero replacement. Displayed peak statistics and
// the displayed metric clamp do not participate in this score.
func (r CampaignResultRules) Score(ticks uint32, local, opponent CampaignResultStatistics) (CampaignScore, error) {
	var out CampaignScore
	if opponent.ScenarioOptions == 0 {
		return out, fmt.Errorf("native campaign score DIVU by zero")
	}
	difference := uint32(uint16(local.BattleWins - opponent.BattleWins))
	product := difference * (uint32(local.BattleWins)<<16 | uint32(local.ScenarioOptions))
	dividend := product&0xffff0000 | uint32(uint16(product)+1)
	quotient := dividend / uint32(opponent.ScenarioOptions)
	if quotient > 0xffff {
		out.DivideOverflow = true
		out.RatioWord = uint16(dividend)
	} else {
		out.RatioWord = uint16(quotient)
	}
	out.Value = uint16(r.ScoreBase+ticks) + out.RatioWord + uint16(uint32(local.WeightedPowerUse)*uint32(r.PowerWeight))
	return out, nil
}

type CampaignProgress struct {
	Campaign, LocalVictory, Complete                bool
	BoltAward, WorldStep, PresentedWorld, NextWorld uint16
	OpenDeity                                       bool
}

// Progress translates $3a28..$3aac and the bounded $b244 completion guard.
// Bolts are awarded in campaign mode2 on either outcome. A local elimination
// advances one world except at999. An opponent elimination advances by the
// score-derived step, caps at999 from below, and can pass999 when starting
// there. The completion screen then resets the world to0. XP allocation and
// password generation remain the existing deity-screen operations.
func (r CampaignResultRules) Progress(mode, selected, eliminated, world, score uint16, cbMemory FollowerCleanupMemory) (CampaignProgress, error) {
	p := CampaignProgress{LocalVictory: selected != eliminated, PresentedWorld: world, NextWorld: world}
	if mode != 2 {
		return p, nil
	}
	if selected > 2 || cbMemory.Read16 == nil || cbMemory.Write16 == nil {
		return p, fmt.Errorf("native campaign profile memory missing")
	}
	p.Campaign = true
	p.BoltAward = min(score/r.BoltDivisor, r.BoltCap)
	a := 0xe76a + int(selected)*314 + 0x58
	bolts, err := cbMemory.Read16(a)
	if err != nil {
		return p, err
	}
	if err := cbMemory.Write16(a, bolts+p.BoltAward); err != nil {
		return p, err
	}
	p.WorldStep = min(score/r.WorldDivisor+1, r.WorldStepCap)
	if !p.LocalVictory {
		if world != r.LastWorld {
			p.NextWorld = world + 1
			p.PresentedWorld = p.NextWorld
		}
		return p, nil
	}
	next := world + p.WorldStep
	if int16(next) > int16(r.LastWorld) && world != r.LastWorld {
		next = r.LastWorld
	}
	p.PresentedWorld, p.NextWorld, p.OpenDeity = next, next, true
	if int16(next) >= int16(r.LastWorld+1) {
		p.Complete = true
		p.NextWorld = 0
	}
	return p, nil
}
