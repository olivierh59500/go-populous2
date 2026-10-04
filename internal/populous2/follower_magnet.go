package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type FollowerMagnetRules struct {
	Hero      FollowerHeroRules
	WaitTicks uint16
	Frames    map[int]AnimationFrame
	Loops     map[int]int
}

func DecodeFollowerMagnetRules(exe *amiga.Executable) (FollowerMagnetRules, error) {
	var rules FollowerMagnetRules
	var err error
	rules.Hero, err = DecodeFollowerHeroRules(exe)
	if err != nil {
		return rules, err
	}
	code := exe.Hunks[0].Data
	rules.WaitTicks = binary.BigEndian.Uint16(code[0x20aec:])
	rules.Frames, rules.Loops = make(map[int]AnimationFrame), make(map[int]int)
	for index := 0; index < 256; index++ {
		offset := 0x168 + index*4
		image := int16(binary.BigEndian.Uint16(code[0x23d1a+offset:]))
		if image < 0 {
			if index == 0 || offset+int(image) < 0x168 || offset+int(image) >= offset || image%4 != 0 {
				return FollowerMagnetRules{}, fmt.Errorf("invalid native magnet waiting loop")
			}
			rules.Loops[offset] = int(image)
			return rules, nil
		}
		layers, err := decodeImageLayers(code, uint16(image))
		if err != nil {
			return FollowerMagnetRules{}, err
		}
		rules.Frames[offset] = AnimationFrame{Layers: layers}
	}
	return FollowerMagnetRules{}, fmt.Errorf("unterminated native magnet waiting animation")
}

type FollowerMagnetCallbacks struct {
	Hero      FollowerHeroCallbacks
	Attrition FollowerAttritionCallbacks
	Merge     func(NativeRecordReference, NativeRecordReference) error // Complete $128fc.
}

type FollowerMagnetStep struct {
	Target                                            NativeRecordReference
	Search, Redispatch, CountPopulation, NextFollower bool
	ClaimedLeader, Merged, Dead                       bool
}

// Tick translates raw states $12/$3a at $11bb4/$11bec. Search jumps directly
// to $1131c without another common prepass; Redispatch goes through $112b8.
// Targets are original signed byte references, including the14-byte marker.

func (rules FollowerMagnetRules) Tick(ref NativeRecordReference, cb FollowerMagnetCallbacks) (FollowerMagnetStep, error) {
	if cb.Hero.Memory.Read8 == nil {
		return FollowerMagnetStep{}, fmt.Errorf("native magnet memory callback missing")
	}
	state, err := cb.Hero.Memory.Read8(cleanupRecordAddress(ref) + 22)
	if err != nil {
		return FollowerMagnetStep{}, err
	}
	if state != 0x12 && state != 0x3a {
		return FollowerMagnetStep{}, fmt.Errorf("native magnet state%x unsupported", state)
	}
	return rules.tick(ref, cb, state)
}

// Decide enters $11bb4 from ordinary search with the existing state retained.
// Native $1131c has already applied attrition before its mode16 branch, and
// $11bb4 applies it again. The caller must preserve that double subtraction.
func (rules FollowerMagnetRules) Decide(ref NativeRecordReference, cb FollowerMagnetCallbacks) (FollowerMagnetStep, error) {
	return rules.tick(ref, cb, 0x12)
}
func (rules FollowerMagnetRules) tick(ref NativeRecordReference, cb FollowerMagnetCallbacks, state uint8) (FollowerMagnetStep, error) {
	var step FollowerMagnetStep
	m, source := cb.Hero.Memory, cleanupRecordAddress(ref)
	if !winMemoryValid(m) || cb.Merge == nil || cb.Attrition.Read == nil || cb.Attrition.Write == nil || cb.Attrition.Cleanup == nil {
		return step, fmt.Errorf("native magnet callbacks missing")
	}
	if state == 0x3a {
		animation, err := m.Read16(source + 10)
		if err != nil {
			return step, err
		}
		next := uint16(animation + 4)
		if loop, ok := rules.Loops[int(next)]; ok {
			next += uint16(int16(loop))
		}
		if _, ok := rules.Frames[int(next)]; !ok {
			return step, fmt.Errorf("native magnet waiting animation outside decoded sequence")
		}
		if err := m.Write16(source+10, next); err != nil {
			return step, err
		}
	}
	owner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	god := heroGodAddress(owner)
	mode, err := m.Read16(god + 12)
	if err != nil {
		return step, err
	}
	if mode != 16 {
		if state == 0x3a {
			if err := m.Write16(source+10, 0); err != nil {
				return step, err
			}
		}
		step.Search = true
		return step, nil
	}
	attrition := func() error {
		amount, err := m.Read32(god + 0x14)
		if err != nil {
			return err
		}
		result, err := ApplyFollowerAttrition(ref, amount, cb.Attrition)
		step.Dead, step.Redispatch = result.Died, result.Died
		return err
	}
	if state == 0x3a {
		previous, err := m.Read16(source + 20)
		if err != nil {
			return step, err
		}
		if err := m.Write16(source+20, previous-1); err != nil {
			return step, err
		}
		// SUBI.W/BGT consumes overflow directly: -32768 also expires.
		if int16(previous) <= 1 {
			if err := m.Write16(source+20, rules.WaitTicks); err != nil {
				return step, err
			}
			if err := attrition(); err != nil || step.Dead {
				return step, err
			}
		}
		marker, err := m.Read16(god + 10)
		if err != nil {
			return step, err
		}
		step.Target = NativeRecordReference(marker)
		x, err := m.Read8(source + 6)
		if err != nil {
			return step, err
		}
		y, err := m.Read8(source + 8)
		if err != nil {
			return step, err
		}
		mx, err := m.Read8(cleanupRecordAddress(step.Target) + 6)
		if err != nil {
			return step, err
		}
		my, err := m.Read8(cleanupRecordAddress(step.Target) + 8)
		if err != nil {
			return step, err
		}
		if x == mx && y == my {
			step.CountPopulation = true
			return step, nil
		}
		if err := m.Write8(source+22, 0x12); err != nil {
			return step, err
		}
		if err := m.Write16(source+10, 0); err != nil {
			return step, err
		}
		step.Redispatch = true
		return step, nil
	}
	if err := attrition(); err != nil || step.Dead {
		return step, err
	}
	return rules.Follow(ref, cb)
}

// Follow is the bare $140f0 call, without mode checking or attrition.
func (rules FollowerMagnetRules) Follow(ref NativeRecordReference, cb FollowerMagnetCallbacks) (FollowerMagnetStep, error) {
	var step FollowerMagnetStep
	m, source := cb.Hero.Memory, cleanupRecordAddress(ref)
	if !winMemoryValid(m) || cb.Merge == nil {
		return step, fmt.Errorf("native magnet following callbacks missing")
	}
	owner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	god := heroGodAddress(owner)

	flags, err := m.Read8(source + 13)
	if err != nil {
		return step, err
	}
	marker, err := m.Read16(god + 10)
	if err != nil {
		return step, err
	}
	target := marker
	if flags&1 == 0 {
		leader, err := m.Read16(god + 8)
		if err != nil {
			return step, err
		}
		if leader != 0 {
			target = leader
		}
	}
	step.Target = NativeRecordReference(target)
	address := cleanupRecordAddress(step.Target)
	tx, err := m.Read8(address + 6)
	if err != nil {
		return step, err
	}
	ty, err := m.Read8(address + 8)
	if err != nil {
		return step, err
	}
	timer, err := rules.Hero.Plan(ref, tx, ty, cb.Hero)
	if err != nil {
		return step, err
	}
	if err := m.Write16(source+20, timer); err != nil {
		return step, err
	}
	if timer != 0 {
		if err := m.Write8(source+23, 0x12); err != nil {
			return step, err
		}
		if err := m.Write8(source+22, 4); err != nil {
			return step, err
		}
	} else {
		x, err := m.Read8(address + 7)
		if err != nil {
			return step, err
		}
		if err := m.Write8(source+7, x); err != nil {
			return step, err
		}
		y, err := m.Read8(address + 9)
		if err != nil {
			return step, err
		}
		if err := m.Write8(source+9, y); err != nil {
			return step, err
		}
		leader, err := m.Read16(god + 8)
		if err != nil {
			return step, err
		}
		if leader == 0 {
			if err := m.Write16(god+8, uint16(ref)); err != nil {
				return step, err
			}
			flags, err := m.Read8(source + 13)
			if err != nil {
				return step, err
			}
			if err := m.Write8(source+13, flags|1); err != nil {
				return step, err
			}
			step.ClaimedLeader = true
		}
		flags, err := m.Read8(source + 13)
		if err != nil {
			return step, err
		}
		if flags&1 == 0 {
			leader, err := m.Read16(god + 8)
			if err != nil {
				return step, err
			}
			if err := cb.Merge(ref, NativeRecordReference(leader)); err != nil {
				return step, err
			}
			step.Merged = true
		} else {
			if err := m.Write16(source+10, 0x168); err != nil {
				return step, err
			}
			if err := m.Write8(source+22, 0x3a); err != nil {
				return step, err
			}
			if err := m.Write16(source+20, rules.WaitTicks); err != nil {
				return step, err
			}
		}
	}
	owner, err = m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	step.CountPopulation, step.NextFollower = owner != 0, owner == 0
	return step, nil
}
