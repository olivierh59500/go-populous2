package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// FollowerHeroRules translates the shared hero decisions at $12044/$1204e.
// Hero-specific motion artwork and hazard handling remain in their own native
// controllers; this is not the inherited Populous I knight behavior.
type FollowerHeroRules struct {
	Properties     [256]uint16
	Alternatives   [9][8][2]int16
	WallAnimations [256]uint16
	WallThresholds [2]uint32
}

func DecodeFollowerHeroRules(exe *amiga.Executable) (FollowerHeroRules, error) {
	var rules FollowerHeroRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native hero decision tables missing")
	}
	code := exe.Hunks[0].Data
	for index := range rules.Properties {
		rules.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	for index := range rules.Alternatives {
		offset := int(int8(code[0x20c86+index]))
		for direction := range rules.Alternatives[index] {
			at := 0x20c86 + 10 + offset + direction*4
			rules.Alternatives[index][direction] = [2]int16{int16(binary.BigEndian.Uint16(code[at:])), int16(binary.BigEndian.Uint16(code[at+2:]))}
		}
	}
	for index := range rules.WallAnimations {
		// The original wall stage is sign-extended, without doubling it.
		rules.WallAnimations[index] = binary.BigEndian.Uint16(code[0x20f1e+int(int8(uint8(index))):])
	}
	for index := range rules.WallThresholds {
		rules.WallThresholds[index] = binary.BigEndian.Uint32(code[0x20d5c+index*4:])
	}
	return rules, nil
}

type FollowerHeroCallbacks struct {
	Memory       FollowerCleanupMemory
	RaiseEnabled func() bool                                              // Original global word $f12.
	Raise        func(uint8, uint8) error                                 // Direct unpriced $d81e, not player sculpting.
	Contact      func(NativeRecordReference, NativeRecordReference) error // $12ade.
	Cleanup      func(NativeRecordReference, uint16) error                // Complete $124a2.
}

type FollowerHeroStep struct {
	Target                           NativeRecordReference
	Selected, Contact, Waiting, Dead bool
	CountPopulation, Redispatch      bool
	Timer                            uint16
}

func heroGodAddress(owner uint8) int {
	return 0xe76a + int(int16(uint16(uint32(uint16(int16(int8(owner))))*314)))
}

func heroByteDistance(a, b uint8) int16 {
	difference := int8(a - b)
	if difference < 0 {
		difference = -difference
	}
	return int16(difference)
}

// SelectTarget translates $14414. Preferred ties select the later pool slot.
// If every enemy is captive or claimed, the fallback is the last qualifying
// record, rather than the closest. Existing stale backlinks are not cleared.
func (rules FollowerHeroRules) SelectTarget(ref NativeRecordReference, m FollowerCleanupMemory) (NativeRecordReference, error) {
	if !winMemoryValid(m) {
		return 0, fmt.Errorf("native hero memory callbacks missing")
	}
	source := cleanupRecordAddress(ref)
	owner, err := m.Read8(source + 12)
	if err != nil {
		return 0, err
	}
	x, err := m.Read8(source + 6)
	if err != nil {
		return 0, err
	}
	y, err := m.Read8(source + 8)
	if err != nil {
		return 0, err
	}
	best, preferred, fallback := int16(0x7fff), 0, 0
	for address := 0x76f4; address < 0xc800; address += 52 {
		other, err := m.Read8(address + 12)
		if err != nil {
			return 0, err
		}
		if int8(other) <= 0 || other == owner {
			continue
		}
		population, err := m.Read32(address + 26)
		if err != nil {
			return 0, err
		}
		if int32(population) <= 0 {
			continue
		}
		tx, err := m.Read8(address + 6)
		if err != nil {
			return 0, err
		}
		ty, err := m.Read8(address + 8)
		if err != nil {
			return 0, err
		}
		distance := heroByteDistance(tx, x) + heroByteDistance(ty, y)
		if best < distance || address == source {
			continue
		}
		fallback = address
		flags, err := m.Read8(address + 13)
		if err != nil {
			return 0, err
		}
		backlink, err := m.Read16(address + 36)
		if err != nil {
			return 0, err
		}
		if flags&8 != 0 || backlink != 0 {
			continue
		}
		best, preferred = distance, address
	}
	if preferred == 0 {
		preferred = fallback
	}
	if preferred == 0 {
		return 0, nil
	}
	target := NativeRecordReference(uint16(preferred - 0x76c0))
	if err := m.Write16(source+34, uint16(target)); err != nil {
		return 0, err
	}
	if err := m.Write16(preferred+36, uint16(ref)); err != nil {
		return 0, err
	}
	if err := m.Write8(source+22, 0x26); err != nil {
		return 0, err
	}
	return target, nil
}

// Probe translates $141a2, including mutation of hostile walls. Return values
// are native D0 words: zero admits, -1 requests terrain, -2 blocks, 1 is outside.
func (rules FollowerHeroRules) Probe(ref NativeRecordReference, tile NativePackedTile, dx, dy int16, m FollowerCleanupMemory) (int16, error) {
	if !winMemoryValid(m) {
		return 0, fmt.Errorf("native hero memory callbacks missing")
	}
	x := uint8(uint16(tile)) + uint8(dx)
	packed := uint16(tile)&0xff00 | uint16(x)
	packed += uint16(uint8(dy)) << 8
	if packed&0xc0c0 != 0 {
		return 1, nil
	}
	grid := 0xf44 + int(packed&0xff00) + int(uint8(packed))*4
	code, err := m.Read8(grid + 1)
	if err != nil {
		return 0, err
	}
	properties := rules.Properties[code]
	if properties&8 != 0 {
		return -1, nil
	}
	source := cleanupRecordAddress(ref)
	flags, err := m.Read8(source + 13)
	if err != nil {
		return 0, err
	}
	hero, err := m.Read16(source + 40)
	if err != nil {
		return 0, err
	}
	if properties&0x790 != 0 && flags&2 != 0 && hero == 0 {
		return -1, nil
	}
	next, err := m.Read16(grid + 2)
	if err != nil {
		return 0, err
	}
	seen := make(map[uint16]bool)
	for next != 0 {
		if seen[next] {
			return 0, fmt.Errorf("cyclic native hero movement chain")
		}
		seen[next] = true
		address := cleanupRecordAddress(NativeRecordReference(next))
		kind, err := m.Read8(address)
		if err != nil {
			return 0, err
		}
		if kind == 0x18 {
			return -2, nil
		}
		if kind == 0x1a {
			owner, err := m.Read8(source + 12)
			if err != nil {
				return 0, err
			}
			other, err := m.Read8(address + 12)
			if err != nil {
				return 0, err
			}
			if owner != other {
				xp, err := m.Read8(heroGodAddress(owner) + 0x54)
				if err != nil {
					return 0, err
				}
				population, err := m.Read32(source + 26)
				if err != nil {
					return 0, err
				}
				// MOVE.B XP replaces only MULU(owner,314)'s low byte. Its
				// retained upper bytes contribute to both native thresholds.
				xpWord := uint32(owner)*314&0xffffff00 | uint32(xp)
				threshold := xpWord*128 + rules.WallThresholds[1]
				if int32(threshold) < int32(population) {
					if flags&2 != 0 {
						if err := m.Write8(source+22, 0x2a); err != nil {
							return 0, err
						}
						animation := uint16(0x7cc)
						if owner != 1 {
							animation = 0x7d4
						}
						if err := m.Write16(source+10, animation); err != nil {
							return 0, err
						}
					}
					stage, err := m.Read8(address + 1)
					if err != nil {
						return 0, err
					}
					if err := m.Write8(address, 0x1c); err != nil {
						return 0, err
					}
					if stage&1 != 0 {
						// Native has already changed the wall kind when this
						// word read raises its 68000 address-error exception.
						return 0, fmt.Errorf("native wall animation address is odd")
					}
					if err := m.Write16(address+10, rules.WallAnimations[stage]); err != nil {
						return 0, err
					}
				} else if int32(xpWord*128+rules.WallThresholds[0]) >= int32(population) {
					return -2, nil
				}
			}
		}
		next, err = m.Read16(address + 2)
		if err != nil {
			return 0, err
		}
	}
	return 0, nil
}

func heroSignedStep(target, current uint8) int16 {
	if int8(target) > int8(current) {
		return 1
	}
	if int8(target) < int8(current) {
		return -1
	}
	return 0
}

// Plan translates $1452e. Failed probes try the native ordered vector table,
// excluding the exact reverse vector; final failure still centers the source
// and consumes a movement timer with zero velocity. Same-cell contact does not.
func (rules FollowerHeroRules) Plan(ref NativeRecordReference, targetX, targetY uint8, cb FollowerHeroCallbacks) (uint16, error) {
	m, source := cb.Memory, cleanupRecordAddress(ref)
	if !winMemoryValid(m) || cb.RaiseEnabled == nil || cb.Raise == nil {
		return 0, fmt.Errorf("native hero planner callbacks missing")
	}
	x, err := m.Read8(source + 6)
	if err != nil {
		return 0, err
	}
	y, err := m.Read8(source + 8)
	if err != nil {
		return 0, err
	}
	dx, dy := heroSignedStep(targetX, x), heroSignedStep(targetY, y)
	if dx == 0 && dy == 0 {
		return 0, nil
	}
	probe := func(a, b int16) (int16, error) {
		return rules.Probe(ref, NativePackedTile(uint16(y)<<8|uint16(x)), a, b, m)
	}
	result, err := probe(dx, dy)
	if err != nil {
		return 0, err
	}
	if result != 0 {
		if result == -1 {
			if cb.RaiseEnabled() {
				if err := cb.Raise(x+uint8(dx), y+uint8(dy)); err != nil {
					return 0, err
				}
			} else {
				owner, err := m.Read8(source + 12)
				if err != nil {
					return 0, err
				}
				god := heroGodAddress(owner)
				if err := m.Write16(god+0x32, uint16(ref)); err != nil {
					return 0, err
				}
				if err := m.Write16(god+0x34, uint16(y+uint8(dy))<<8|uint16(x+uint8(dx))); err != nil {
					return 0, err
				}
			}
		}
		found := false
		for _, direction := range rules.Alternatives[3*dx+dy+4] {
			if direction[0] == -dx && direction[1] == -dy {
				continue
			}
			result, err = probe(direction[0], direction[1])
			if err != nil {
				return 0, err
			}
			if result == 0 {
				dx, dy, found = direction[0], direction[1], true
				break
			}
		}
		if !found {
			result, err = probe(dx, dy)
			if err != nil {
				return 0, err
			}
			if result != 0 {
				dx, dy = 0, 0
			}
		}
	}
	speed, err := m.Read8(source + 18)
	if err != nil {
		return 0, err
	}
	if err := m.Write16(source+14, uint16(dx*int16(speed))); err != nil {
		return 0, err
	}
	if err := m.Write16(source+16, uint16(dy*int16(speed))); err != nil {
		return 0, err
	}
	if speed == 0 {
		// Native DIVU traps after both velocity writes, before centering.
		return 0, ErrFollowerZeroSpeed
	}
	if err := m.Write8(source+7, 128); err != nil {
		return 0, err
	}
	if err := m.Write8(source+9, 128); err != nil {
		return 0, err
	}
	return 256 / uint16(speed), nil
}

// Decide is the bounded state24/state26 dispatch. State24 goes to $123b4
// population accounting. State26 returns to $112b8 immediately, rerunning the
// common prepass before its new movement/waiting/death state is dispatched.
func (rules FollowerHeroRules) Decide(ref NativeRecordReference, cb FollowerHeroCallbacks) (FollowerHeroStep, error) {
	var step FollowerHeroStep
	m, source := cb.Memory, cleanupRecordAddress(ref)
	if !winMemoryValid(m) || cb.Cleanup == nil || cb.Contact == nil {
		return step, fmt.Errorf("native hero decision callbacks missing")
	}
	state, err := m.Read8(source + 22)
	if err != nil {
		return step, err
	}
	if state == 0x24 {
		step.Target, err = rules.SelectTarget(ref, m)
		step.Selected, step.CountPopulation = step.Target != 0, true
		return step, err
	}
	if state != 0x26 {
		return step, fmt.Errorf("native hero decision state%x unsupported", state)
	}
	step.Redispatch = true
	owner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	population, err := m.Read32(source + 26)
	if err != nil {
		return step, err
	}
	attrition, err := m.Read32(heroGodAddress(owner) + 0x14)
	if err != nil {
		return step, err
	}
	population -= attrition
	if err := m.Write32(source+26, population); err != nil {
		return step, err
	}
	// The intervening MOVE.L clears SUB.L's overflow flag. BGT therefore
	// tests the signed, wrapped population that was actually stored.
	if int32(population) <= 0 {
		if err := cb.Cleanup(ref, 1); err != nil {
			return step, err
		}
		flags, err := m.Read8(source + 13)
		if err != nil {
			return step, err
		}
		animation, deathState := uint16(0x7f4), uint8(0x2c)
		if flags&2 != 0 {
			animation, deathState = 0x9d4, 0x40
		}
		if err := m.Write16(source+10, animation); err != nil {
			return step, err
		}
		if err := m.Write8(source+22, deathState); err != nil {
			return step, err
		}
		step.Dead = true
		return step, nil
	}
	target, err := m.Read16(source + 34)
	if err != nil {
		return step, err
	}
	valid := false
	if target != 0 {
		address := cleanupRecordAddress(NativeRecordReference(target))
		p, err := m.Read32(address + 26)
		if err != nil {
			return step, err
		}
		o, err := m.Read8(address + 12)
		if err != nil {
			return step, err
		}
		valid = int32(p) > 0 && int8(o) > 0 && o != owner
	}
	if !valid {
		selected, err := rules.SelectTarget(ref, m)
		if err != nil {
			return step, err
		}
		target, step.Selected = uint16(selected), selected != 0
	}
	step.Target = NativeRecordReference(target)
	if target == 0 {
		if err := m.Write16(source+20, 20); err != nil {
			return step, err
		}
		if err := m.Write8(source+23, 0x24); err != nil {
			return step, err
		}
		if err := m.Write8(source+22, 0x0a); err != nil {
			return step, err
		}
		if err := m.Write16(source+10, 0xccc); err != nil {
			return step, err
		}
		step.Waiting, step.Timer = true, 20
		return step, nil
	}
	address := cleanupRecordAddress(NativeRecordReference(target))
	tx, err := m.Read8(address + 6)
	if err != nil {
		return step, err
	}
	ty, err := m.Read8(address + 8)
	if err != nil {
		return step, err
	}
	step.Timer, err = rules.Plan(ref, tx, ty, cb)
	if err != nil {
		return step, err
	}
	if err := m.Write16(source+20, step.Timer); err != nil {
		return step, err
	}
	if step.Timer == 0 {
		step.Contact = true
		return step, cb.Contact(ref, NativeRecordReference(target))
	}
	if err := m.Write8(source+23, 0x26); err != nil {
		return step, err
	}
	return step, m.Write8(source+22, 4)
}
