package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// FungusHazardRules describes the fungus branch of the native follower
// terrain prepass at $12ec2..$12f1e. It does not replace the caller's ordered
// water/font/swamp checks, follower scheduling, or death-animation playback.
type FungusHazardRules struct {
	HeroDeath         [6]int
	OrdinaryAnimation int
	RawSoundArgument  uint16
	SoundCue          int
	DeathKind         uint8
	DeathState        uint8
	CenterFraction    uint8
	CleanupMode       uint8
	Frames            map[int]AnimationFrame
	SequenceLengths   map[int]int
}

func DecodeFungusHazardRules(exe *amiga.Executable) (FungusHazardRules, error) {
	var rules FungusHazardRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x20a48+12 {
		return rules, fmt.Errorf("native fungus hazard tables missing")
	}
	code := exe.Hunks[0].Data
	// Verify the bounded branch literals before reading their operands.
	for _, expected := range []struct {
		at   int
		word uint16
	}{
		{0x12eec, 0x317c}, {0x12ef2, 0x303c}, {0x12efc, 0x117c},
		{0x12f02, 0x117c}, {0x12f08, 0x117c}, {0x12f16, 0x7001},
	} {
		if expected.at+4 > len(code) || binary.BigEndian.Uint16(code[expected.at:]) != expected.word {
			return FungusHazardRules{}, fmt.Errorf("unsupported native fungus hazard branch")
		}
	}
	rules.OrdinaryAnimation = int(binary.BigEndian.Uint16(code[0x12eec+2:]))
	rules.RawSoundArgument = binary.BigEndian.Uint16(code[0x12ef2+2:])
	if rules.RawSoundArgument%10 != 0 || rules.RawSoundArgument/10 >= 133 {
		return FungusHazardRules{}, fmt.Errorf("invalid native fungus hazard sound offset")
	}
	// $184f6 adds the raw offset to its ten-byte descriptor table.
	rules.SoundCue = int(rules.RawSoundArgument / 10)
	rules.DeathKind = uint8(binary.BigEndian.Uint16(code[0x12efc+2:]))
	rules.DeathState = uint8(binary.BigEndian.Uint16(code[0x12f02+2:]))
	rules.CenterFraction = uint8(binary.BigEndian.Uint16(code[0x12f08+2:]))
	rules.CleanupMode = uint8(binary.BigEndian.Uint16(code[0x12f16:]) & 0xff)
	rules.Frames = make(map[int]AnimationFrame)
	rules.SequenceLengths = make(map[int]int)
	starts := []int{rules.OrdinaryAnimation}
	for index := range rules.HeroDeath {
		rules.HeroDeath[index] = int(binary.BigEndian.Uint16(code[0x20a48+index*2:]))
		if rules.HeroDeath[index] != 0 {
			starts = append(starts, rules.HeroDeath[index])
		}
	}
	for _, start := range starts {
		if _, ok := rules.SequenceLengths[start]; ok {
			continue
		}
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FungusHazardRules{}, err
		}
		rules.SequenceLengths[start] = len(frames)
		for index, frame := range frames {
			rules.Frames[start+index*4] = frame
		}
	}
	return rules, nil
}

type FungusHazardDecision struct {
	Applies, Immune  bool
	Animation        int
	Kind, State      uint8
	CenterFraction   uint8
	CleanupMode      uint8
	RawSoundArgument uint16
	SoundCue         int
}

// Enter reports the native fungus transition without deleting its actor.
// Fresh tile145 has bit0 and is harmless; mature146..150 have bit4 without
// bit0. Hero indices follow heroIDs; zero animation is elemental immunity.
// As in the common prepass, the caller filters inactive/neutral-owner actors
// and gives higher-priority terrain hazards their original precedence.
// The caller must preserve the death record/occupancy until its animation ends.
func (rules *FungusHazardRules) Enter(properties uint16, heroic bool, hero int) FungusHazardDecision {
	var decision FungusHazardDecision
	if rules == nil || properties&0x10 == 0 || properties&1 != 0 {
		return decision
	}
	animation := rules.OrdinaryAnimation
	if heroic {
		if hero < 0 || hero >= len(rules.HeroDeath) {
			return decision
		}
		animation = rules.HeroDeath[hero]
		if animation == 0 {
			decision.Immune = true
			return decision
		}
	}
	return FungusHazardDecision{Applies: true, Animation: animation, Kind: rules.DeathKind, State: rules.DeathState, CenterFraction: rules.CenterFraction, CleanupMode: rules.CleanupMode, RawSoundArgument: rules.RawSoundArgument, SoundCue: rules.SoundCue}
}
