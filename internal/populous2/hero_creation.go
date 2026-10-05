package populous2

import "fmt"

type HeroCreationCallbacks struct {
	Memory      FollowerCleanupMemory
	ClearLeader func(NativeRecordReference) error
	ClearFarms  func(NativeRecordReference, uint8) error
	Sound       func(uint16) error
}

type HeroCreationStep struct {
	Reference NativeRecordReference
	Created   bool
}

// Create translates $142d4. The deity's raw leader reference is the only
// admission test; population and the leader flag are not checked here.
func (rules HeroRules) Create(id SpellID, ownerWord uint16, cb HeroCreationCallbacks) (HeroCreationStep, error) {
	var step HeroCreationStep
	if heroIndex(id) < 0 || cb.Memory.Read16 == nil {
		return step, fmt.Errorf("native hero creation input missing")
	}
	owner := uint16(int16(int8(uint8(ownerWord))))
	god := 0xe76a + int(int16(uint16(uint32(owner)*314)))
	ref, err := cb.Memory.Read16(god + 8)
	if err != nil || ref == 0 {
		return step, err
	}
	step.Reference = NativeRecordReference(ref)
	err = rules.Convert(step.Reference, id, cb)
	step.Created = err == nil
	return step, err
}

// Convert translates the direct $142fe/$14302 entry, also used by Adonis.
// It retains motion, targets, timers and unrelated flags. Population doubling
// wraps as a 68000 long; movement speed alone saturates at 255.
func (rules HeroRules) Convert(ref NativeRecordReference, id SpellID, cb HeroCreationCallbacks) error {
	i := heroIndex(id)
	m := cb.Memory
	if i < 0 || !winMemoryValid(m) || cb.ClearLeader == nil || cb.ClearFarms == nil || cb.Sound == nil {
		return fmt.Errorf("native hero conversion callbacks missing")
	}
	a := cleanupRecordAddress(ref)
	owner, err := m.Read8(a + 12)
	if err != nil {
		return err
	}
	god := heroGodAddress(owner)
	if err := cb.ClearLeader(ref); err != nil {
		return err
	}
	kind, err := m.Read8(a)
	if err != nil {
		return err
	}
	if kind == 4 {
		if err := cb.ClearFarms(ref, 15); err != nil {
			return err
		}
	}
	flags, err := m.Read8(a + 13)
	if err != nil {
		return err
	}
	if err := m.Write8(a+13, flags|2); err != nil {
		return err
	}
	if err := m.Write8(a+22, 0x24); err != nil {
		return err
	}
	if err := m.Write8(a, 2); err != nil {
		return err
	}
	for _, field := range []struct {
		offset int
		value  uint16
	}{{50, 0}, {10, 0}, {40, uint16(i * 2)}, {50, rules.Variants[i]}} {
		if err := m.Write16(a+field.offset, field.value); err != nil {
			return err
		}
	}
	if err := cb.Sound(rules.Sounds[i]); err != nil {
		return err
	}
	if id == Heracles {
		population, err := m.Read32(a + 26)
		if err != nil {
			return err
		}
		if err := m.Write32(a+26, population+population); err != nil {
			return err
		}
	}
	experience, err := m.Read8(god + 0x52 + i)
	if err != nil {
		return err
	}
	bonus := uint16(experience >> 3)
	if id == Odysseus {
		speed, err := m.Read8(a + 18)
		if err != nil {
			return err
		}
		bonus += uint16(speed)
	}
	speed, err := m.Read8(a + 18)
	if err != nil {
		return err
	}
	return m.Write8(a+18, uint8(min(uint16(255), uint16(speed)+bonus)))
}
