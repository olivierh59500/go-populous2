package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativePrimitiveCreatorRules struct {
	WhirlwindLife, FireColumnLife            uint16
	WhirlwindSpeed, FireColumnSpeed, TreeAge uint8
	FireColumnJitter                         [9]int16
	TreeAnimations                           [4]uint16
}

func DecodeNativePrimitiveCreatorRules(exe *amiga.Executable) (NativePrimitiveCreatorRules, error) {
	var rules NativePrimitiveCreatorRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x20f4a {
		return rules, fmt.Errorf("native primitive creator tables missing")
	}
	code := exe.Hunks[0].Data
	rules.FireColumnLife = binary.BigEndian.Uint16(code[0x20d44:])
	rules.FireColumnSpeed = code[0x20d47]
	rules.WhirlwindLife = binary.BigEndian.Uint16(code[0x20d48:])
	rules.WhirlwindSpeed = code[0x20d4b]
	rules.TreeAge = code[0x20f3d]
	for index := range rules.FireColumnJitter {
		rules.FireColumnJitter[index] = int16(binary.BigEndian.Uint16(code[0x20d7a+index*2:]))
	}
	for index := range rules.TreeAnimations {
		rules.TreeAnimations[index] = binary.BigEndian.Uint16(code[0x20f42+index*2:])
	}
	return rules, nil
}

type NativePrimitiveCreatorCallbacks struct {
	Memory FollowerCleanupMemory
	Random func() uint16                     // Original $f622 result, 0..32767.
	Link   func(NativeRecordReference) error // $125a0; no pressure increment.
}

type NativePrimitiveCreation struct {
	Reference       NativeRecordReference
	Created, Cycled bool
	RandomDraws     int
}

func primitiveFreeRecord(memory FollowerCleanupMemory, start, end, stride int) (int, error) {
	for address := start; address < end; address += stride {
		owner, err := memory.Read8(address + 12)
		if err != nil {
			return 0, err
		}
		if owner == 0 {
			return address, nil
		}
	}
	return 0, nil
}

func primitiveDeityAddress(owner uint16) int { return 0xe76a + int(int16(uint16(uint32(owner)*314))) }

// CreateWhirlwind translates $15c3e. Its owner is the original D2 word, not a
// Go side index. Neutral owner3 gets no Air XP. No coordinate/terrain admission
// or RNG is added, and every recycled byte outside the native writes survives.
func (rules NativePrimitiveCreatorRules) CreateWhirlwind(owner uint16, x, y uint8, cb NativePrimitiveCreatorCallbacks) (NativePrimitiveCreation, error) {
	var step NativePrimitiveCreation
	m := cb.Memory
	if !winMemoryValid(m) || cb.Link == nil {
		return step, fmt.Errorf("native whirlwind creator callbacks missing")
	}
	address, err := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
	if err != nil || address == 0 {
		return step, err
	}
	for _, write := range []struct {
		offset int
		value  uint8
	}{{12, uint8(owner)}, {6, x}, {7, 128}, {8, y}, {9, 128}} {
		if err := m.Write8(address+write.offset, write.value); err != nil {
			return step, err
		}
	}
	if err := m.Write16(address+20, 1); err != nil {
		return step, err
	}
	if err := m.Write8(address, 0x20); err != nil {
		return step, err
	}
	if err := m.Write16(address+10, 0x4c8); err != nil {
		return step, err
	}
	if err := m.Write8(address+22, 8); err != nil {
		return step, err
	}
	experience := uint8(0)
	// CMP.B is signed; MULU still consumes the complete D2 word.
	if int8(uint8(owner)) <= 2 {
		experience, err = m.Read8(primitiveDeityAddress(owner) + 0x55)
		if err != nil {
			return step, err
		}
	}
	if err := m.Write16(address+24, rules.WhirlwindLife+uint16(experience)); err != nil {
		return step, err
	}
	if err := m.Write8(address+18, rules.WhirlwindSpeed); err != nil {
		return step, err
	}
	step.Reference = NativeRecordReference(uint16(address - 0x76c0))
	if err := cb.Link(step.Reference); err != nil {
		return step, err
	}
	step.Created = true
	return step, nil
}

// CreateFireColumn is $15b7c, rather than the Fungus creator $15fda. Jitter
// consumes RNG before bounds/pool checks. Success consumes a second unused
// draw and permits water or occupied tiles. Owner3 bypasses Fire XP lookup.
func (rules NativePrimitiveCreatorRules) CreateFireColumn(owner uint16, x, y uint8, cb NativePrimitiveCreatorCallbacks) (NativePrimitiveCreation, error) {
	var step NativePrimitiveCreation
	m := cb.Memory
	if !winMemoryValid(m) || cb.Link == nil || cb.Random == nil {
		return step, fmt.Errorf("native fire column creator callbacks missing")
	}
	bits := cb.Random()
	step.RandomDraws++
	packed := uint16(y)<<8 | uint16(x)
	packed += uint16(rules.FireColumnJitter[(bits%18)/2])
	if packed&0xc0c0 != 0 {
		return step, nil
	}
	address, err := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
	if err != nil || address == 0 {
		return step, err
	}
	if err := m.Write8(address+12, uint8(owner)); err != nil {
		return step, err
	}
	if err := m.Write8(address+6, uint8(packed)); err != nil {
		return step, err
	}
	if err := m.Write8(address+7, 128); err != nil {
		return step, err
	}
	if err := m.Write16(address+8, packed); err != nil {
		return step, err
	}
	if err := m.Write8(address+9, 128); err != nil {
		return step, err
	}
	cb.Random()
	step.RandomDraws++
	if err := m.Write8(address+18, 16); err != nil {
		return step, err
	}
	if err := m.Write16(address+20, 1); err != nil {
		return step, err
	}
	if err := m.Write8(address, 0x22); err != nil {
		return step, err
	}
	if err := m.Write16(address+10, 0x1a0); err != nil {
		return step, err
	}
	if err := m.Write8(address+22, 2); err != nil {
		return step, err
	}
	experience := uint8(0)
	// This caller uses CMP.W, unlike Whirlwind's low-byte comparison.
	if int16(owner) <= 2 {
		experience, err = m.Read8(primitiveDeityAddress(owner) + 0x56)
		if err != nil {
			return step, err
		}
	}
	if err := m.Write16(address+24, rules.FireColumnLife+uint16(experience)); err != nil {
		return step, err
	}
	if err := m.Write8(address+18, rules.FireColumnSpeed); err != nil {
		return step, err
	}
	step.Reference = NativeRecordReference(uint16(address - 0x76c0))
	if err := cb.Link(step.Reference); err != nil {
		return step, err
	}
	step.Created = true
	return step, nil
}

// PlantTree translates $db26: cycle the first linked kind16 tree with one of
// the four recognized animations, or allocate one14-byte neutral record.
// It does not plant a random cluster or require an empty/owned/land tile.
func (rules NativePrimitiveCreatorRules) PlantTree(x, y uint8, cb NativePrimitiveCreatorCallbacks) (NativePrimitiveCreation, error) {
	var step NativePrimitiveCreation
	m := cb.Memory
	if !winMemoryValid(m) || cb.Link == nil {
		return step, fmt.Errorf("native single-tree creator callbacks missing")
	}
	// Native byte additions wrap X*4, and ADDA.W sign-extends the result.
	grid := 0xf44 + int(int16(uint16(y)<<8|uint16(uint8(x<<2))))
	head, err := m.Read16(grid + 2)
	if err != nil {
		return step, err
	}
	seen := make(map[uint16]bool)
	for head != 0 {
		if seen[head] {
			return step, fmt.Errorf("cyclic native tree admission chain")
		}
		seen[head] = true
		address := cleanupRecordAddress(NativeRecordReference(head))
		kind, err := m.Read8(address)
		if err != nil {
			return step, err
		}
		if kind == 0x16 {
			animation, err := m.Read16(address + 10)
			if err != nil {
				return step, err
			}
			for index, value := range rules.TreeAnimations {
				if animation == value {
					if err := m.Write16(address+10, rules.TreeAnimations[(index+1)%len(rules.TreeAnimations)]); err != nil {
						return step, err
					}
					step.Reference, step.Cycled = NativeRecordReference(head), true
					return step, nil
				}
			}
		}
		head, err = m.Read16(address + 2)
		if err != nil {
			return step, err
		}
	}
	address, err := primitiveFreeRecord(m, 0x6bd0, 0x76c0, 14)
	if err != nil || address == 0 {
		return step, err
	}
	for _, write := range []struct {
		offset int
		value  uint8
	}{{12, 3}, {0, 0x16}, {6, x}, {8, y}, {1, rules.TreeAge}} {
		if err := m.Write8(address+write.offset, write.value); err != nil {
			return step, err
		}
	}
	if err := m.Write16(address+10, rules.TreeAnimations[0]); err != nil {
		return step, err
	}
	if err := m.Write8(address+7, 128); err != nil {
		return step, err
	}
	if err := m.Write8(address+9, 128); err != nil {
		return step, err
	}
	step.Reference = NativeRecordReference(uint16(address - 0x76c0))
	if err := cb.Link(step.Reference); err != nil {
		return step, err
	}
	step.Created = true
	return step, nil
}
