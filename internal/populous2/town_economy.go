package populous2

import "fmt"

type NativeTownEconomyRules struct {
	ManaAdd, PopulationAdd, PopulationLimit [TownStages]uint16
	EmigrationDivisor, WorkTicks            [TownStages]uint16
}

func DecodeNativeTownEconomyRules(land Landscape) (NativeTownEconomyRules, error) {
	var rules NativeTownEconomyRules
	for stage := range TownStages {
		rules.ManaAdd[stage], rules.PopulationAdd[stage] = uint16(land.ManaAdd[stage]), uint16(land.PopulationAdd[stage])
		rules.PopulationLimit[stage], rules.EmigrationDivisor[stage], rules.WorkTicks[stage] = uint16(land.PopulationLimit[stage]), uint16(land.EmigrationDivisor[stage]), uint16(land.WorkTicks[stage])
		if rules.EmigrationDivisor[stage] == 0 {
			return NativeTownEconomyRules{}, fmt.Errorf("native town emigration divisor is zero at stage%d", stage)
		}
	}
	return rules, nil
}

// NativeTownEconomyState carries caller-owned globals. PoolBlocked is $dc2,
// reset once at the start of $11252, rather than once per town. Selection is
// the raw follower reference represented by pointer $f36; Clock/Deadline are
// the wrapping long words at $f40/$dd8.
type NativeTownEconomyState struct {
	PoolBlocked     uint16
	Selected        NativeRecordReference
	Clock, Deadline uint32
}

type NativeTownLandRequest struct {
	Registers FollowerCleanupRegisters
}

type NativeTownEconomyCallbacks struct {
	Memory       FollowerCleanupMemory
	Prepass      func(NativeRecordReference) error
	EvaluateTown func(NativeRecordReference) (int, error)
	ClearFarms   func(NativeRecordReference, uint8) error
	Insert       func(NativeRecordReference) error
	Random       func() int
	// LandAI is the actual $131cc creator, including any record/map changes.
	// It is deliberately external; a generic terrain change is not equivalent.
	LandAI func(NativeTownLandRequest) error
}

type NativeTownEconomyStep struct {
	Handled, Prepassed, Worked, CurrentTotal, NeedsDecision bool
	PoolExhausted, SamePassEligible, RequestedLandAI        bool
	Born                                                    NativeRecordReference
	GodAddress                                              int
}

func townGodAddress(owner uint8) int {
	return 0xe76a + int(int16(uint16(uint32(uint16(int16(int8(owner))))*314)))
}

// Tick translates state6/$11738 through the economic body at $117de. Its
// support failure immediately redispatches ordinary search; an allocated
// higher slot is eligible for the same increasing-address follower pass.
func (r *NativeTownEconomyRules) Tick(reference NativeRecordReference, state *NativeTownEconomyState, cb NativeTownEconomyCallbacks) (NativeTownEconomyStep, error) {
	var step NativeTownEconomyStep
	if r == nil || state == nil || !winMemoryValid(cb.Memory) || cb.EvaluateTown == nil || cb.ClearFarms == nil {
		return step, fmt.Errorf("native town economy callbacks missing")
	}
	if cb.Prepass != nil {
		if err := cb.Prepass(reference); err != nil {
			return step, err
		}
		step.Prepassed = true
	}
	m, source := cb.Memory, cleanupRecordAddress(reference)
	actorState, err := m.Read8(source + 22)
	if err != nil {
		return step, err
	}
	if actorState != 6 {
		return step, nil
	}
	step.Handled = true
	oldStage, err := m.Read8(source + 1)
	if err != nil {
		return step, err
	}
	stageWord, err := cb.EvaluateTown(reference)
	if err != nil {
		return step, err
	}
	if int16(uint16(stageWord)) <= 0 {
		if err := m.Write8(source, 2); err != nil {
			return step, err
		}
		if err := m.Write8(source+22, 2); err != nil {
			return step, err
		}
		if err := m.Write16(source+10, 0); err != nil {
			return step, err
		}
		if err := cb.ClearFarms(reference, 15); err != nil {
			return step, err
		}
		step.NeedsDecision = true
		return step, nil
	}
	if stageWord >= TownStages {
		return step, fmt.Errorf("native town economy stage outside table")
	}
	owner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	step.GodAddress = townGodAddress(owner)
	god := step.GodAddress
	storedStage, err := m.Read8(source + 1)
	if err != nil {
		return step, err
	}
	if oldStage == 0 || int8(storedStage) < 17 {
		previousStage, err := m.Read8(source + 19)
		if err != nil {
			return step, err
		}
		if storedStage != previousStage {
			if err := m.Write16(god+0x2e, uint16(reference)); err != nil {
				return step, err
			}
		}
	}
	population, err := m.Read32(source + 26)
	if err != nil {
		return step, err
	}
	best, err := m.Read16(god + 0x1c)
	if err != nil {
		return step, err
	}
	if int16(uint16(population)) > int16(best) {
		if err := m.Write16(god+0x1c, uint16(population)); err != nil {
			return step, err
		}
		if err := m.Write16(god+0x1e, uint16(reference)); err != nil {
			return step, err
		}
	}
	founded, err := m.Read16(source + 46)
	if err != nil {
		return step, err
	}
	oldest, err := m.Read16(god + 0x20)
	if err != nil {
		return step, err
	}
	if founded <= oldest {
		if err := m.Write16(god+0x20, founded); err != nil {
			return step, err
		}
		if err := m.Write16(god+0x22, uint16(reference)); err != nil {
			return step, err
		}
	}
	count, err := m.Read16(god + 0x24)
	if err != nil {
		return step, err
	}
	if err := m.Write16(god+0x24, count+1); err != nil {
		return step, err
	}
	step.CurrentTotal = true
	work, err := m.Read16(source + 20)
	if err != nil {
		return step, err
	}
	work++
	if err := m.Write16(source+20, work); err != nil {
		return step, err
	}
	stage := int(stageWord)
	if r.WorkTicks[stage] > work {
		return step, nil
	}
	if err := m.Write16(source+20, 0); err != nil {
		return step, err
	}
	step.Worked = true
	flags, err := m.Read8(source + 13)
	if err != nil {
		return step, err
	}
	if flags&16 == 0 {
		mana, err := m.Read32(god)
		if err != nil {
			return step, err
		}
		if err := m.Write32(god, mana+uint32(r.ManaAdd[stage])); err != nil {
			return step, err
		}
	}
	newPopulation := population + uint32(r.PopulationAdd[stage])
	forced := flags&4 != 0
	if err := m.Write8(source+13, flags&^4); err != nil {
		return step, err
	}
	if !forced && int32(newPopulation) <= int32(r.PopulationLimit[stage]) || state.PoolBlocked != 0 {
		return step, m.Write32(source+26, newPopulation)
	}
	if r.EmigrationDivisor[stage] == 0 {
		return step, fmt.Errorf("native town division by zero")
	}
	emigrant := uint32(nativeCombatQuotient(int32(newPopulation), r.EmigrationDivisor[stage]))
	if err := m.Write32(source+26, population-emigrant); err != nil {
		return step, err
	}
	slot := -1
	for at := 0x76f4; at < 0xc800; at += 52 {
		owner, err := m.Read8(at + 12)
		if err != nil {
			return step, err
		}
		if owner == 0 {
			slot = at
			break
		}
	}
	if slot < 0 {
		if err := m.Write32(source+26, population); err != nil {
			return step, err
		}
		state.PoolBlocked = 1
		step.PoolExhausted = true
		return step, nil
	}
	if cb.Insert == nil {
		return step, fmt.Errorf("native newborn insertion missing")
	}
	for offset := 0; offset < 52; offset += 2 {
		if err := m.Write16(slot+offset, 0); err != nil {
			return step, err
		}
	}
	step.Born = NativeRecordReference(uint16(slot - 0x76c0))
	step.SamePassEligible = uint16(step.Born) > uint16(reference)
	if state.Selected == reference {
		state.Selected = step.Born
	}
	if err := m.Write32(slot+26, emigrant); err != nil {
		return step, err
	}
	if err := m.Write8(slot, 2); err != nil {
		return step, err
	}
	if err := m.Write8(slot+22, 2); err != nil {
		return step, err
	}
	owner, err = m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	if err := m.Write8(slot+12, owner); err != nil {
		return step, err
	}
	xy, err := m.Read32(source + 6)
	if err != nil {
		return step, err
	}
	if err := m.Write32(slot+6, xy); err != nil {
		return step, err
	}
	if err := m.Write32(slot+14, 0); err != nil {
		return step, err
	}
	speed, err := m.Read8(source + 18)
	if err != nil {
		return step, err
	}
	if err := m.Write8(slot+18, speed); err != nil {
		return step, err
	}
	if err := m.Write16(slot+50, uint16((slot-0x76c0)/52)&14); err != nil {
		return step, err
	}
	storedStage, err = m.Read8(source + 1)
	if err != nil {
		return step, err
	}
	if err := m.Write8(slot+25, storedStage); err != nil {
		return step, err
	}
	if err := m.Write8(slot+24, storedStage*2); err != nil {
		return step, err
	}
	if err := m.Write16(slot+10, 0); err != nil {
		return step, err
	}
	if err := m.Write8(slot+1, 0); err != nil {
		return step, err
	}
	flags, err = m.Read8(source + 13)
	if err != nil {
		return step, err
	}
	if err := m.Write8(slot+13, flags); err != nil {
		return step, err
	}
	extra, err := m.Read16(source + 48)
	if err != nil {
		return step, err
	}
	if err := m.Write16(slot+48, extra); err != nil {
		return step, err
	}
	if err := m.Write8(source+13, flags&0xfc); err != nil {
		return step, err
	}
	childFlags := flags & 0x13
	if err := m.Write8(slot+13, childFlags); err != nil {
		return step, err
	}
	if childFlags&1 != 0 {
		if err := m.Write16(god+8, uint16(step.Born)); err != nil {
			return step, err
		}
	} else if childFlags&2 != 0 {
		hero, err := m.Read16(source + 40)
		if err != nil {
			return step, err
		}
		if err := m.Write16(slot+40, hero); err != nil {
			return step, err
		}
		variant, err := m.Read16(source + 50)
		if err != nil {
			return step, err
		}
		childVariant, err := m.Read16(slot + 50)
		if err != nil {
			return step, err
		}
		if err := m.Write16(source+50, childVariant); err != nil {
			return step, err
		}
		if err := m.Write16(slot+50, variant); err != nil {
			return step, err
		}
		if err := m.Write16(source+40, 0); err != nil {
			return step, err
		}
	}
	if uint16(step.Born) == 0x32c8 && int32(state.Clock) >= int32(state.Deadline) {
		if cb.Random == nil || cb.LandAI == nil {
			return step, fmt.Errorf("native rare birth callbacks missing")
		}
		state.Deadline = state.Clock + 50
		cb.Random() // Its apparent X value is overwritten by the second draw.
		random := uint16(cb.Random())
		kind := ((random % 12) &^ 1) + 2
		request := NativeTownLandRequest{Registers: FollowerCleanupRegisters{D0: uint32(random/12)<<16 | uint32(kind), D1: uint32(uint16(int16(int8(owner))))*314&0xffff0000 | uint32(random&63), D2: population&0xffff0000 | uint32(kind)}}
		if err := cb.LandAI(request); err != nil {
			return step, err
		}
		step.RequestedLandAI = true
	}
	return step, cb.Insert(step.Born)
}
