package populous2

import "fmt"

type NativeNeutralCallbacks struct {
	Memory FollowerCleanupMemory
	Insert func(NativeRecordReference) error
}

type NativeNeutralCreation struct {
	Created bool
	Records []NativeRecordReference
}

// CreateNativeNeutral translates $131cc. It scans owner bytes, retains all
// unassigned fields and uses D0/D1's low coordinate bytes. D2's low word is
// the even native selector, including selector zero's allocated failure path.
func CreateNativeNeutral(registers FollowerCleanupRegisters, cb NativeNeutralCallbacks) (NativeNeutralCreation, error) {
	var step NativeNeutralCreation
	m := cb.Memory
	if !winMemoryValid(m) || cb.Insert == nil {
		return step, fmt.Errorf("native neutral creator callbacks missing")
	}
	selector := uint16(registers.D2)
	if selector&1 != 0 || selector > 12 {
		return step, fmt.Errorf("native neutral selector outside bounded table")
	}
	find := func(start int) (int, error) {
		for address := start; address < 0xc800; address += 52 {
			owner, err := m.Read8(address + 12)
			if err != nil {
				return 0, err
			}
			if owner == 0 {
				return address, nil
			}
		}
		return 0, nil
	}
	address, err := find(0x76f4)
	if err != nil || address == 0 {
		return step, err
	}
	for _, field := range []struct {
		offset int
		value  uint8
	}{{12, 3}, {0, 0x3c}, {22, 0x44}} {
		if err := m.Write8(address+field.offset, field.value); err != nil {
			return step, err
		}
	}
	for _, offset := range []int{26, 6, 14} {
		if err := m.Write32(address+offset, 0); err != nil {
			return step, err
		}
	}
	if err := m.Write8(address+6, uint8(registers.D0)); err != nil {
		return step, err
	}
	if err := m.Write8(address+8, uint8(registers.D1)); err != nil {
		return step, err
	}
	if err := m.Write16(address+40, selector); err != nil {
		return step, err
	}
	ref := NativeRecordReference(uint16(address - 0x76c0))
	step.Records = append(step.Records, ref)
	if selector == 0 {
		return step, nil // Native jumps to its failure return after allocation.
	}
	animation := [7]uint16{0, 0x2cc, 0x53c, 0xa98, 0xab4, 0x2bfc, 0x2c18}[selector/2]
	if err := m.Write16(address+10, animation); err != nil {
		return step, err
	}
	writeByte := func(offset int, value uint8) error { return m.Write8(address+offset, value) }
	writeWord := func(offset int, value uint16) error { return m.Write16(address+offset, value) }
	switch selector {
	case 2:
		if err := writeByte(9, 128); err != nil {
			return step, err
		}
		if err := writeByte(18, 16); err != nil {
			return step, err
		}
		if err := writeWord(14, 16); err != nil {
			return step, err
		}
	case 4:
		if err := writeByte(7, 128); err != nil {
			return step, err
		}
		if err := writeByte(9, 127); err != nil {
			return step, err
		}
		if err := writeWord(18, 32); err != nil {
			return step, err
		}
		if err := writeWord(16, 0xffe0); err != nil {
			return step, err
		}
	case 6:
		if err := writeByte(7, 128); err != nil {
			return step, err
		}
		if err := writeByte(18, 48); err != nil {
			return step, err
		}
		if err := writeWord(16, 48); err != nil {
			return step, err
		}
	case 8:
		if err := writeByte(9, 128); err != nil {
			return step, err
		}
		if err := writeByte(18, 32); err != nil {
			return step, err
		}
		if err := writeWord(14, 0xffe0); err != nil {
			return step, err
		}
	case 10:
		if err := writeByte(9, 128); err != nil {
			return step, err
		}
		if err := writeByte(18, 48); err != nil {
			return step, err
		}
		if err := writeWord(14, 48); err != nil {
			return step, err
		}
	case 12:
		if err := writeByte(9, 128); err != nil {
			return step, err
		}
		if err := writeByte(7, 128); err != nil {
			return step, err
		}
		if err := writeByte(18, 32); err != nil {
			return step, err
		}
		if err := writeWord(14, 32); err != nil {
			return step, err
		}
		if err := writeWord(16, 32); err != nil {
			return step, err
		}
	}
	if err := cb.Insert(ref); err != nil {
		return step, err
	}
	step.Created = true
	if selector == 4 && int8(uint8(registers.D1)) < 63 {
		second, err := find(address)
		if err != nil || second == 0 {
			return step, err
		}
		// The original copy increments A4 through the complete first record.
		// Its second allocation is scanned from that first actor's address.
		for offset := 0; offset < 52; offset += 2 {
			value, err := m.Read16(address + offset)
			if err != nil {
				return step, err
			}
			if err := m.Write16(second+offset, value); err != nil {
				return step, err
			}
		}
		if err := m.Write16(second+10, 0x550); err != nil {
			return step, err
		}
		y, err := m.Read8(second + 8)
		if err != nil {
			return step, err
		}
		if err := m.Write8(second+8, y+1); err != nil {
			return step, err
		}
		secondRef := NativeRecordReference(uint16(second - 0x76c0))
		step.Records = append(step.Records, secondRef)
		if err := cb.Insert(secondRef); err != nil {
			return step, err
		}
	}
	return step, nil
}
