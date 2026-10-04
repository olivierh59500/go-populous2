package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type FollowerCombatRules struct {
	PopulationDivisor uint16
	Frames            map[int]AnimationFrame
	Loops             map[int]int
}

func DecodeFollowerCombatRules(exe *amiga.Executable) (FollowerCombatRules, error) {
	var rules FollowerCombatRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a+0x1e0 {
		return rules, fmt.Errorf("native combat tables missing")
	}
	code := exe.Hunks[0].Data
	rules.PopulationDivisor = binary.BigEndian.Uint16(code[0x20bb4:])
	if rules.PopulationDivisor == 0 {
		return rules, fmt.Errorf("native battle divisor is zero")
	}
	frames, err := DecodeAnimation(exe, 0x1c8)
	if err != nil {
		return rules, err
	}
	rules.Frames = make(map[int]AnimationFrame)
	rules.Loops = make(map[int]int)
	for i, frame := range frames {
		rules.Frames[0x1c8+i*4] = frame
	}
	end := 0x1c8 + len(frames)*4
	rules.Loops[end] = int(int16(binary.BigEndian.Uint16(code[0x23d1a+end:])))
	return rules, nil
}

type FollowerCombatCallbacks struct {
	Read   func(NativeRecordReference) (FollowerEntryActor, error)
	Write  func(NativeRecordReference, FollowerEntryActor) error
	Random func() int
	// Cleanup is the original $124a2 operation, including statistics, hero/
	// leader links and graph removal. Both-dead combat passes mode0 twice.
	Cleanup func(NativeRecordReference, uint16) error
	// Win is $1298c, not a generic population reward. It owns the original
	// mana/statistic tables, town reform/destruction, death images and Adonis
	// post-combat splitting. It must retain native raw record relationships.
	Win func(winner, loser NativeRecordReference) error
}

type FollowerCombatStep struct {
	CurrentTotal, NextFollower, RedispatchSearch bool
	Winner, Loser                                NativeRecordReference
}

// nativeCombatQuotient retains 68000 DIVU overflow: when the quotient cannot
// fit a word, D0 is unchanged and its original low word drives later MULU.
func nativeCombatQuotient(population int32, divisor uint16) uint16 {
	dividend := uint32(population)
	quotient := dividend / uint32(divisor)
	if quotient > 65535 {
		return uint16(dividend)
	}
	return uint16(quotient)
}

func (r *FollowerCombatRules) TickAggressor(reference NativeRecordReference, cb FollowerCombatCallbacks) (FollowerCombatStep, error) {
	var step FollowerCombatStep
	if r == nil || r.PopulationDivisor == 0 || cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native battle callbacks/rules missing")
	}
	actor, err := cb.Read(reference)
	if err != nil {
		return step, err
	}
	next := actor.Motion.Animation + 4
	if loop, ok := r.Loops[next]; ok {
		next += loop
	}
	if _, ok := r.Frames[next]; !ok {
		return step, fmt.Errorf("native aggressor animation outside sequence")
	}
	actor.Motion.Animation = next
	if err := cb.Write(reference, actor); err != nil {
		return step, err
	}
	opponentRef := NativeRecordReference(actor.Contact30)
	opponent, err := cb.Read(opponentRef)
	if err != nil {
		return step, err
	}
	if int8(opponent.Owner) <= 0 {
		actor.Motion.State, actor.Motion.Animation = 2, 0
		if err := cb.Write(reference, actor); err != nil {
			return step, err
		}
		step.CurrentTotal = true
		return step, nil
	}
	if cb.Random == nil {
		return step, fmt.Errorf("native battle RNG missing")
	}
	cb.Random()
	// $11b00 overwrites the preceding randomized/clamped D1 calculation.
	// Both damage amounts therefore use the aggressor's D0 quotient only.
	quotient := uint32(nativeCombatQuotient(actor.Motion.Population, r.PopulationDivisor))
	opponent.Motion.Population = int32(uint32(opponent.Motion.Population) - (uint32(actor.Weapon)*quotient + 10))
	actor.Motion.Population = int32(uint32(actor.Motion.Population) - (uint32(opponent.Weapon)*quotient + 10))
	if err := cb.Write(opponentRef, opponent); err != nil {
		return step, err
	}
	if err := cb.Write(reference, actor); err != nil {
		return step, err
	}
	if actor.Motion.Population <= 0 && opponent.Motion.Population <= 0 {
		if cb.Cleanup == nil {
			return step, fmt.Errorf("native mutual-death cleanup missing")
		}
		if err := cb.Cleanup(reference, 0); err != nil {
			return step, err
		}
		if err := cb.Cleanup(opponentRef, 0); err != nil {
			return step, err
		}
		step.NextFollower = true
		return step, nil
	}
	if actor.Motion.Population <= 0 {
		step.Winner, step.Loser = opponentRef, reference
		step.NextFollower = true
	} else if opponent.Motion.Population <= 0 {
		step.Winner, step.Loser = reference, opponentRef
		step.CurrentTotal = true
	} else {
		step.CurrentTotal = true
		return step, nil
	}
	if cb.Win == nil {
		return step, fmt.Errorf("native battle winner handler missing")
	}
	return step, cb.Win(step.Winner, step.Loser)
}

// TickDefender translates state16/$11b6a. Only a positive-owner enemy in
// state14 with the reciprocal Contact30 word keeps this record waiting.
// Failure immediately branches to ordinary search, unlike the aggressor's
// stale-opponent branch which ends the current update at population totals.
func (r *FollowerCombatRules) TickDefender(reference NativeRecordReference, cb FollowerCombatCallbacks) (FollowerCombatStep, error) {
	var step FollowerCombatStep
	if cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native defender callbacks missing")
	}
	actor, err := cb.Read(reference)
	if err != nil {
		return step, err
	}
	if actor.Contact30 != 0 {
		opponent, err := cb.Read(NativeRecordReference(actor.Contact30))
		if err != nil {
			return step, err
		}
		if int8(opponent.Owner) > 0 && opponent.Owner != actor.Owner && opponent.Motion.State == 14 && opponent.Contact30 == uint16(reference) {
			step.CurrentTotal = true
			return step, nil
		}
	}
	actor.Motion.Animation, actor.Motion.State = 0, 2
	if err := cb.Write(reference, actor); err != nil {
		return step, err
	}
	step.RedispatchSearch = true
	return step, nil
}
