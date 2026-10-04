package populous2

import "fmt"

type FollowerLeaderCallbacks struct {
	Memory FollowerCleanupMemory
	// These are the original $125da/$125a0 marker primitives. The marker
	// is removed and prepended even when its target cell is unchanged; this
	// relocation does not add pressure to the cell header.
	Unlink func(NativeRecordReference) error
	Insert func(NativeRecordReference) error
}

type FollowerLeaderStep struct {
	Registers       FollowerCleanupRegisters
	GodAddress      int
	MarkerReference NativeRecordReference
	Owner           uint8
	Cleared         bool
}

// ClearFollowerLeader translates $140ae and its complete $13fe4 marker
// relocation. It changes no population or loss statistics. The original
// wrapper preserves D0 but leaves D2's sign-extended owner low word and, for
// a leader, D1's original unclamped Y byte visible to the caller.
func ClearFollowerLeader(reference NativeRecordReference, registers FollowerCleanupRegisters, cb FollowerLeaderCallbacks) (FollowerLeaderStep, error) {
	step := FollowerLeaderStep{Registers: registers}
	m := cb.Memory
	if m.Read8 == nil {
		return step, fmt.Errorf("native leader source memory missing")
	}
	source := cleanupRecordAddress(reference)
	owner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	step.Owner = owner
	ownerWord := uint16(int16(int8(owner)))
	step.Registers.D2 = registers.D2&0xffff0000 | uint32(ownerWord)
	step.GodAddress = 0xe76a + int(int16(uint16(uint32(ownerWord)*314)))
	flags, err := m.Read8(source + 13)
	if err != nil {
		return step, err
	}
	if flags&1 == 0 {
		return step, nil
	}
	if m.Read16 == nil || m.Write8 == nil || m.Write16 == nil || cb.Unlink == nil || cb.Insert == nil {
		return step, fmt.Errorf("native leader relocation callbacks missing")
	}
	if err := m.Write8(source+13, flags&^1); err != nil {
		return step, err
	}
	if err := m.Write16(step.GodAddress+8, 0); err != nil {
		return step, err
	}
	x, err := m.Read8(source + 6)
	if err != nil {
		return step, err
	}
	y, err := m.Read8(source + 8)
	if err != nil {
		return step, err
	}
	step.Registers.D1 = registers.D1&0xffffff00 | uint32(y)
	// MOVE.B owner,D2 preserves the high byte produced by EXT.W. The
	// register-preserving $13fe4 wrapper restores this unmultiplied value.
	markerOwner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	step.Registers.D2 = step.Registers.D2&0xffffff00 | uint32(markerOwner)
	markerGod := 0xe76a + int(int16(uint16(uint32(uint16(int16(int8(markerOwner))))*314)))
	marker, err := m.Read16(markerGod + 10)
	if err != nil {
		return step, err
	}
	step.MarkerReference = NativeRecordReference(marker)
	if err := cb.Unlink(step.MarkerReference); err != nil {
		return step, err
	}
	clamp := func(value uint8) uint8 {
		if int8(value) < 0 {
			return 0
		}
		return min(value, 63)
	}
	markerAddress := cleanupRecordAddress(step.MarkerReference)
	// Keep the four ordered byte writes from the original relocation, which
	// matters when callers expose aliases or record memory write traces.
	for _, field := range []struct {
		offset int
		value  uint8
	}{{6, clamp(x)}, {7, 128}, {8, clamp(y)}, {9, 128}} {
		if err := m.Write8(markerAddress+field.offset, field.value); err != nil {
			return step, err
		}
	}
	if err := cb.Insert(step.MarkerReference); err != nil {
		return step, err
	}
	step.Cleared = true
	return step, nil
}
