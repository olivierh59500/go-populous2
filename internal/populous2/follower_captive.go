package populous2

import "fmt"

type FollowerCaptiveRules struct{ Hero FollowerHeroRules }

type FollowerCaptiveCallbacks struct {
	Hero      FollowerHeroCallbacks
	Attrition FollowerAttritionCallbacks
}

type FollowerCaptiveStep struct {
	Target, Captor              NativeRecordReference
	Released, Rebound, Dead     bool
	Redispatch, CountPopulation bool
	Timer                       uint16
}

// Tick translates $120cc, raw state34. A1 retains the original word42 target
// even when backlink repair changes that word. The native post-Helen value35c
// therefore follows an overlapping record view for this first movement leg.
func (rules FollowerCaptiveRules) Tick(ref NativeRecordReference, cb FollowerCaptiveCallbacks) (FollowerCaptiveStep, error) {
	var step FollowerCaptiveStep
	m, source := cb.Hero.Memory, cleanupRecordAddress(ref)
	if !winMemoryValid(m) || cb.Attrition.Read == nil || cb.Attrition.Write == nil || cb.Attrition.Cleanup == nil {
		return step, fmt.Errorf("native captive callbacks missing")
	}
	state, err := m.Read8(source + 22)
	if err != nil {
		return step, err
	}
	if state != 0x34 {
		return step, fmt.Errorf("native captive state%x unsupported", state)
	}
	owner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	amount, err := m.Read32(heroGodAddress(owner) + 20)
	if err != nil {
		return step, err
	}
	attrition, err := ApplyFollowerAttrition(ref, amount, cb.Attrition)
	if err != nil {
		return step, err
	}
	if attrition.Died {
		step.Dead, step.Redispatch = true, true
		return step, nil
	}
	target, err := m.Read16(source + 42)
	if err != nil {
		return step, err
	}
	step.Target = NativeRecordReference(target)
	address := cleanupRecordAddress(step.Target)
	flags, err := m.Read8(address + 13)
	if err != nil {
		return step, err
	}
	other, err := m.Read8(address + 12)
	if err != nil {
		return step, err
	}
	valid := (flags&2 != 0 || flags&8 != 0) && int8(other) > 0
	if !valid {
		captor, err := m.Read16(source + 44)
		if err != nil {
			return step, err
		}
		step.Captor = NativeRecordReference(captor)
		back := cleanupRecordAddress(step.Captor)
		other, err := m.Read8(back + 12)
		if err != nil {
			return step, err
		}
		flags, err := m.Read8(back + 13)
		if err != nil {
			return step, err
		}
		if int8(other) <= 0 || flags&2 == 0 {
			flags, err := m.Read8(source + 13)
			if err != nil {
				return step, err
			}
			if err := m.Write8(source+13, flags&^8); err != nil {
				return step, err
			}
			if err := m.Write8(source+22, 2); err != nil {
				return step, err
			}
			step.Released, step.Redispatch = true, true
			return step, nil
		}
		head, err := m.Read16(back + 42)
		if err != nil {
			return step, err
		}
		if head != 0 {
			previous := cleanupRecordAddress(NativeRecordReference(head))
			other, err := m.Read8(previous + 12)
			if err != nil {
				return step, err
			}
			flags, err := m.Read8(previous + 13)
			if err != nil {
				return step, err
			}
			if int8(other) > 0 && flags&8 != 0 {
				if err := m.Write16(previous+42, uint16(ref)); err != nil {
					return step, err
				}
			}
		}
		if err := m.Write16(source+42, captor); err != nil {
			return step, err
		}
		if err := m.Write16(back+42, uint16(ref)); err != nil {
			return step, err
		}
		step.Rebound = true
	}
	// Keep A1's original address. Re-read its coordinate bytes after repair,
	// because writes to overlapping source/captor records can change them.
	x, err := m.Read8(address + 6)
	if err != nil {
		return step, err
	}
	y, err := m.Read8(address + 8)
	if err != nil {
		return step, err
	}
	step.Timer, err = rules.Hero.Plan(ref, x, y, cb.Hero)
	if err != nil {
		return step, err
	}
	if err := m.Write16(source+20, step.Timer); err != nil {
		return step, err
	}
	if err := m.Write8(source+23, 0x34); err != nil {
		return step, err
	}
	if err := m.Write8(source+22, 4); err != nil {
		return step, err
	}
	// Unlike hero chase, timer0 still enters state4; there is no contact call.
	step.CountPopulation = true
	return step, nil
}
