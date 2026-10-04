package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// TownCombatRules decodes the nineteen original destruction sequences. The
// settlement evaluator and follower cleanup keep their own native boundaries.
type TownCombatRules struct {
	DeathAnimations [TownStages]uint16
	Frames          map[int]AnimationFrame
	Loops           map[int]int
}

func DecodeTownCombatRules(exe *amiga.Executable) (TownCombatRules, error) {
	var rules TownCombatRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a {
		return rules, fmt.Errorf("native town combat tables missing")
	}
	code := exe.Hunks[0].Data
	rules.Frames, rules.Loops = make(map[int]AnimationFrame), make(map[int]int)
	for stage := range rules.DeathAnimations {
		pointer := binary.BigEndian.Uint16(code[0x20b3a+stage*2:])
		rules.DeathAnimations[stage] = pointer
		if _, decoded := rules.Frames[int(pointer)]; decoded {
			continue
		}
		frames, err := DecodeAnimation(exe, int(pointer))
		if err != nil {
			return TownCombatRules{}, err
		}
		for index, frame := range frames {
			rules.Frames[int(pointer)+index*4] = frame
		}
		end := int(pointer) + len(frames)*4
		rules.Loops[end] = int(int16(binary.BigEndian.Uint16(code[0x23d1a+end:])))
	}
	return rules, nil
}

type TownCombatCallbacks struct {
	Read  func(NativeRecordReference) (FollowerEntryActor, error)
	Write func(NativeRecordReference, FollowerEntryActor) error
	// ClearFarms and EvaluateTown are the full $135ca/$13352 operations,
	// including overlays, competing towns and their original raw links.
	ClearFarms   func(NativeRecordReference, uint8) error
	EvaluateTown func(NativeRecordReference) (int, error)
	// Cleanup is $124a2, including deity statistics, hero/leader links and
	// mode1 retention. A no-op callback is not a complete destruction port.
	Cleanup func(NativeRecordReference, uint16) error
}

type TownCombatStep struct {
	Guarded, Destroyed, Reformed, Walking bool
	AnimationReference                    NativeRecordReference
}

// Destroy translates $16184. State$30 is protected even with positive owner
// and population. Otherwise the source keeps its allocation and occupancy
// through cleanup mode1; the later state$28 animation owns final removal.
func (r *TownCombatRules) Destroy(reference NativeRecordReference, cb TownCombatCallbacks) (TownCombatStep, error) {
	var step TownCombatStep
	if r == nil || cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native town destruction callbacks missing")
	}
	actor, err := cb.Read(reference)
	if err != nil {
		return step, err
	}
	if actor.Motion.State == 0x30 {
		step.Guarded = true
		return step, nil
	}
	if int(actor.Byte1) >= len(r.DeathAnimations) || cb.ClearFarms == nil || cb.Cleanup == nil {
		return step, fmt.Errorf("native town destruction stage or operation missing")
	}
	actor.Motion.Timer = 0
	if err := cb.Write(reference, actor); err != nil {
		return step, err
	}
	if err := cb.ClearFarms(reference, 95); err != nil {
		return step, err
	}
	// Reload after external native operations instead of overwriting any raw
	// fields that their adapters may have changed.
	actor, err = cb.Read(reference)
	if err != nil {
		return step, err
	}
	actor.Motion.State = 0x28
	actor.Motion.Animation = int(r.DeathAnimations[actor.Byte1])
	actor.Motion.Population = 0
	if err := cb.Write(reference, actor); err != nil {
		return step, err
	}
	if err := cb.Cleanup(reference, 1); err != nil {
		return step, err
	}
	step.Destroyed = true
	return step, nil
}

// Reform translates $12bd8. The caller supplies the incoming A0 reference:
// the hero fallback clears A0+$a while changing A2's kind/state. Nonheroes
// replace A0 with the source before evaluation, so their fallback clears the
// source animation. Work+$14, population, owner and unrelated links survive.
func (r *TownCombatRules) Reform(reference, originalA0 NativeRecordReference, clock uint16, cb TownCombatCallbacks) (TownCombatStep, error) {
	step := TownCombatStep{AnimationReference: originalA0}
	if cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native town reform callbacks missing")
	}
	actor, err := cb.Read(reference)
	if err != nil {
		return step, err
	}
	if actor.Motion.Flags&2 == 0 {
		if cb.EvaluateTown == nil || cb.ClearFarms == nil {
			return step, fmt.Errorf("native town evaluator operations missing")
		}
		actor.Motion.State, actor.Motion.Kind = 6, 4
		actor.Motion.X = int16(uint16(actor.Motion.X)&0xff00 | 0x80)
		actor.Motion.Y = int16(uint16(actor.Motion.Y)&0xff00 | 0x80)
		actor.Byte19, actor.Founded46, actor.Byte1 = 0, clock, 0
		if err := cb.Write(reference, actor); err != nil {
			return step, err
		}
		step.AnimationReference = reference
		stage, err := cb.EvaluateTown(reference)
		if err != nil {
			return step, err
		}
		if int16(uint16(stage)) > 0 {
			step.Reformed = true
			return step, nil
		}
		if err := cb.ClearFarms(reference, 15); err != nil {
			return step, err
		}
	}
	actor, err = cb.Read(reference)
	if err != nil {
		return step, err
	}
	actor.Motion.State, actor.Motion.Kind = 2, 2
	if err := cb.Write(reference, actor); err != nil {
		return step, err
	}
	animationActor, err := cb.Read(step.AnimationReference)
	if err != nil {
		return step, err
	}
	animationActor.Motion.Animation = 0
	if err := cb.Write(step.AnimationReference, animationActor); err != nil {
		return step, err
	}
	step.Walking = true
	return step, nil
}
