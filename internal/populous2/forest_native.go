package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type ForestNativeRules struct {
	Attempts, AgeMask               uint16
	InitialAge                      uint8
	Offsets                         [45]uint16
	Animations                      [4]uint16
	PlacementProperties, Properties [256]uint16
	Neighbors                       [4]uint16
	ImageWords                      []int16
}

func DecodeForestNativeRules(exe *amiga.Executable) (ForestNativeRules, error) {
	var rules ForestNativeRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native forest tables missing")
	}
	code := exe.Hunks[0].Data
	rules.Attempts = binary.BigEndian.Uint16(code[0x20f3e:])
	rules.AgeMask = binary.BigEndian.Uint16(code[0x20f3a:])
	rules.InitialAge = code[0x20f3d]
	if rules.Attempts == 0 {
		return ForestNativeRules{}, fmt.Errorf("native forest attempt divisor is zero")
	}
	for index := range rules.Offsets {
		rules.Offsets[index] = binary.BigEndian.Uint16(code[0x20f4a+index*2:])
	}
	for index := range rules.Animations {
		rules.Animations[index] = binary.BigEndian.Uint16(code[0x20f42+index*2:])
	}
	for index := range rules.Properties {
		rules.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
		rules.PlacementProperties[index] = binary.BigEndian.Uint16(code[0x33312+int(int8(uint8(index)))*2:])
	}
	for index := range rules.Neighbors {
		rules.Neighbors[index] = binary.BigEndian.Uint16(code[0x20bb6+index*2:])
	}
	for at := 0x23d1a; at < 0x26956; at += 2 {
		rules.ImageWords = append(rules.ImageWords, int16(binary.BigEndian.Uint16(code[at:])))
	}
	return rules, nil
}

type ForestNativeCallbacks struct {
	Frame        *NativeFrameRegisterContext
	Memory       FollowerCleanupMemory
	Random       func() uint16
	Link, Unlink func(NativeRecordReference) error
	DestroyTown  func(NativeRecordReference) error
}

type ForestNativePlacement struct {
	References            []NativeRecordReference
	Attempts, RandomDraws int
	Count                 uint16
	PoolFull              bool
}

// Plant translates $da0a. Original D2 is a signed word for the XP test, then
// unsigned MULU/ADDA.W can alias scenery for negative initialization owners.
// Tile properties use EXT.W of the tile byte, preserving the native high-code
// lookup before CODE:$33312. No exclusive legacy occupant check is added.
func (rules ForestNativeRules) Plant(owner uint16, origin NativePackedTile, cb ForestNativeCallbacks) (ForestNativePlacement, error) {
	step := ForestNativePlacement{References: []NativeRecordReference{}}
	m := cb.Memory
	if !winMemoryValid(m) || cb.Random == nil || cb.Link == nil {
		return step, fmt.Errorf("native forest placement callbacks missing")
	}
	bits := cb.Random()
	step.RandomDraws++
	variant := (bits % 8) & 0xfe
	count := bits % rules.Attempts
	if owner != 0 && int16(owner) <= 2 {
		xp, err := m.Read8(primitiveDeityAddress(owner) + 0x53)
		if err != nil {
			return step, err
		}
		count += uint16(xp >> 4)
	}
	for attempt := 0; attempt <= int(count); attempt++ {
		step.Attempts++
		bits = cb.Random()
		step.RandomDraws++
		packed := uint16(origin) + rules.Offsets[(bits%90)/2]
		if packed&0xc0c0 != 0 {
			continue
		}
		grid := tsunamiGrid(packed)
		tile, err := m.Read8(grid + 1)
		if err != nil {
			return step, err
		}
		if tile == 0 || rules.PlacementProperties[tile]&0x40 != 0 {
			continue
		}
		head, err := m.Read16(grid + 2)
		if err != nil {
			return step, err
		}
		if head != 0 {
			continue
		}
		address, err := primitiveFreeRecord(m, 0x6bd0, 0x76c0, 14)
		if err != nil {
			return step, err
		}
		if address == 0 {
			step.PoolFull = true
			return step, nil
		}
		if err := m.Write8(address+12, 3); err != nil {
			return step, err
		}
		if err := m.Write8(address, 0x16); err != nil {
			return step, err
		}
		if err := m.Write8(address+1, rules.InitialAge); err != nil {
			return step, err
		}
		if err := m.Write16(address+10, rules.Animations[variant/2]); err != nil {
			return step, err
		}
		bits = cb.Random()
		step.RandomDraws++
		if bits%90 == 0 {
			if err := m.Write16(address+10, rules.Animations[0]); err != nil {
				return step, err
			}
		}
		if err := m.Write8(address+6, uint8(packed)); err != nil {
			return step, err
		}
		if err := m.Write8(address+7, 128); err != nil {
			return step, err
		}
		if err := m.Write16(address+8, packed&0xff00|128); err != nil {
			return step, err
		}
		step.Count++
		ref := NativeRecordReference(uint16(address - 0x76c0))
		if err := cb.Link(ref); err != nil {
			return step, err
		}
		step.References = append(step.References, ref)
	}
	return step, nil
}

// Cast includes $179be/$179ec's native metric update. Word44 is named by its
// offset here: no independent mana/popularity formula is inferred from it.
func (rules ForestNativeRules) Cast(owner uint8, x, y uint8, cb ForestNativeCallbacks) (ForestNativePlacement, error) {
	step, err := rules.Plant(uint16(owner), NativePackedTile(uint16(y)<<8|uint16(x)), cb)
	if err != nil || step.Count == 0 {
		return step, err
	}
	god := primitiveDeityAddress(uint16(owner))
	metric, err := cb.Memory.Read16(god + 0x44)
	if err != nil {
		return step, err
	}
	return step, cb.Memory.Write16(god+0x44, metric+step.Count)
}

// BurnNeighbors is complete $173b0. It visits only four orthogonal cells,
// kills nonhero kind2 followers without immediate cleanup, invokes the actual
// town destruction helper, and converts neighboring trees to kind1e/artf10.
func (rules ForestNativeRules) BurnNeighbors(ref NativeRecordReference, cb ForestNativeCallbacks) error {
	m := cb.Memory
	if !winMemoryValid(m) || cb.DestroyTown == nil {
		return fmt.Errorf("native forest burning callbacks missing")
	}
	address := cleanupRecordAddress(ref)
	x, err := m.Read8(address + 6)
	if err != nil {
		return err
	}
	y, err := m.Read8(address + 8)
	if err != nil {
		return err
	}
	origin := uint16(y)<<8 | uint16(x)
	for _, offset := range rules.Neighbors {
		packed := origin + offset
		if packed&0xc0c0 != 0 {
			continue
		}
		head, err := m.Read16(tsunamiGrid(packed) + 2)
		if err != nil {
			return err
		}
		seen := map[uint16]bool{}
		for head != 0 {
			if seen[head] {
				return fmt.Errorf("cyclic native forest burning chain")
			}
			seen[head] = true
			victim := cleanupRecordAddress(NativeRecordReference(head))
			kind, err := m.Read8(victim)
			if err != nil {
				return err
			}
			if kind == 2 {
				flags, err := m.Read8(victim + 13)
				if err != nil {
					return err
				}
				if flags&2 == 0 {
					if err := m.Write8(victim, 6); err != nil {
						return err
					}
					if err := m.Write8(victim+22, 8); err != nil {
						return err
					}
					if err := m.Write16(victim+10, 0x178); err != nil {
						return err
					}
					if err := m.Write32(victim+26, 0); err != nil {
						return err
					}
				}
			}
			if kind == 4 {
				if err := cb.DestroyTown(NativeRecordReference(head)); err != nil {
					return err
				}
			}
			if kind == 0x16 {
				if err := m.Write8(victim, 0x1e); err != nil {
					return err
				}
				if err := m.Write16(victim+10, 0xf10); err != nil {
					return err
				}
			}
			head, err = m.Read16(victim + 2)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type ForestNativeStep struct{ Removed, SpreadFire bool }

// Tick translates one record in the original $de36 pool pass. Normal ages
// stop at0; burial/removal uses wrapping signed bytes and reaching24 removes
// before recovery. Kind1e keeps the original temporary memory/register quirks.
func (rules ForestNativeRules) Tick(ref NativeRecordReference, clock uint16, cb ForestNativeCallbacks) (ForestNativeStep, error) {
	var step ForestNativeStep
	m := cb.Memory
	if !winMemoryValid(m) || cb.Unlink == nil {
		return step, fmt.Errorf("native scenery tick callbacks missing")
	}
	address := cleanupRecordAddress(ref)
	owner, err := m.Read8(address + 12)
	if err != nil {
		return step, err
	}
	if owner == 0 {
		return step, nil
	}
	remove := func() error {
		if err := m.Write8(address+12, 0); err != nil {
			return err
		}
		step.Removed = true
		return cb.Unlink(ref)
	}
	kind, err := m.Read8(address)
	if err != nil {
		return step, err
	}
	age, err := m.Read8(address + 1)
	if err != nil {
		return step, err
	}
	if kind == 0x1e {
		if cb.Frame != nil {
			cb.Frame.Byte(0, age)
		}
		if age != 0 {
			if int8(age) > 0 {
				if err := m.Write8(address+1, uint8(-int8(age))); err != nil {
					return step, err
				}
				if err := m.Write16(address+10, 0xae0); err != nil {
					return step, err
				}
			}
			magnitude := uint8(-int8(age)) + 1
			if cb.Frame != nil {
				cb.Frame.Byte(0, magnitude)
			}
			if magnitude == rules.InitialAge {
				return step, remove()
			}
			if cb.Frame != nil {
				cb.Frame.Byte(0, uint8(-int8(magnitude)))
			}
			return step, m.Write8(address+1, uint8(-int8(magnitude)))
		}
		animation, err := m.Read16(address + 10)
		if err != nil {
			return step, err
		}
		next := uint16(animation + 4)
		if cb.Frame != nil {
			cb.Frame.Word(0, next)
		}
		if next&1 != 0 || int(next)/2 >= len(rules.ImageWords) {
			return step, fmt.Errorf("native tree burning animation outside bank")
		}
		if rules.ImageWords[next/2] < 0 {
			if err := m.Write8(address+1, 255); err != nil {
				return step, err
			}
			if err := m.Write16(address+10, 0xad8); err != nil {
				return step, err
			}
			step.SpreadFire = true
			return step, rules.BurnNeighbors(ref, cb)
		}
		return step, m.Write16(address+10, next)
	}
	grid, err := stormCell(m, address)
	if err != nil {
		return step, err
	}
	property := func() (uint16, error) {
		tile, err := m.Read8(grid + 1)
		if err != nil {
			return 0, err
		}
		value := rules.Properties[tile] & 0x88
		if cb.Frame != nil {
			cb.Frame.D[0] = uint32(value)
		}
		return value, nil
	}
	if cb.Frame != nil {
		cb.Frame.Word(1, uint16(grid-0xf44))
		cb.Frame.Byte(0, age)
	}
	if int8(age) < 0 {
		magnitude := uint8(-int8(age)) + 1
		if cb.Frame != nil {
			cb.Frame.Byte(0, magnitude)
		}
		if magnitude == rules.InitialAge {
			return step, remove()
		}
		age = uint8(-int8(magnitude))
		if cb.Frame != nil {
			cb.Frame.Byte(0, age)
		}
		if err := m.Write8(address+1, age); err != nil {
			return step, err
		}
		properties, err := property()
		if err != nil {
			return step, err
		}
		if properties&0x88 == 0 {
			return step, m.Write8(address+1, uint8(-int8(age)))
		}
		return step, nil
	}
	positiveAge := int8(age) > 0
	if cb.Frame != nil && positiveAge {
		cb.Frame.Word(2, clock&rules.AgeMask)
	}
	if positiveAge && clock&rules.AgeMask == 0 {
		age--
		if err := m.Write8(address+1, age); err != nil {
			return step, err
		}
	}
	properties, err := property()
	if err != nil {
		return step, err
	}
	if properties&0x88 != 0 {
		age++
		return step, m.Write8(address+1, uint8(-int8(age)))
	}
	return step, nil
}
