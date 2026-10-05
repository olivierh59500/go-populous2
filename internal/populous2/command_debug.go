package populous2

import "fmt"

// commandInitialFollower is $10cbe, including the retained population upper
// word and template byte fields. Its enclosing MOVEM restores all eight data
// registers even when no pool slot is available.
func (w *World) commandInitialFollower(call NativeCommandCall) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	at, e := primitiveFreeRecord(m.m, 0x76f4, 0xc800, 52)
	if e != nil || at == 0 {
		return false, e
	}
	owner := uint16(c.D[2])
	god := primitiveDeityAddress(owner)
	packed := uint16(c.D[0])
	ref := uint16(at - 0x76c0)
	m.putByte(at, 2)
	m.putByte(at+12, uint8(owner))
	m.putWord(at+50, (ref/52)&14)
	m.putByte(at+6, uint8(packed)>>2)
	m.putByte(at+7, 128)
	m.putWord(at+8, packed&0xff00|128)
	m.putWord(at+10, 0)
	m.putByte(at+13, 0)
	m.putWord(at+14, 0)
	m.putWord(at+16, 0)
	m.putByte(at+24, 2)
	m.putByte(at+18, m.byte(god+0x5f))
	m.putByte(at+25, m.byte(god+0x61))
	m.putWord(at+28, m.word(god+0x5c))
	m.putByte(at+22, 2)
	m.putByte(at+23, 2)
	if m.err != nil {
		return false, m.err
	}
	return false, w.nativeRuntimeInsert(NativeRecordReference(ref))
}

// commandScenery is $db26/$dd1c: cycle a matching tree/boulder in the linked
// chain, or create the first free scenery record. Command88 creates/cycles a
// boulder; it is not a remove-tree action. The creator has no terrain filter.
func (w *World) commandScenery(call NativeCommandCall) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	rules := NativeCommandRules{Code: w.NativeAI.Code}
	x, y := uint8(c.D[0]), uint8(c.D[1])
	packed := uint16(y)<<8 | uint16(x)
	commandWord(c, 3, packed&0xff00|uint16(uint8(packed)*4))
	head := m.word(tsunamiGrid(packed) + 2)
	commandWord(c, 3, head)
	kind, table, ageAt := uint8(0x16), 0x20f42, 0x20f3d
	if call.Routine == 0xdd1c {
		kind, table, ageAt = 0x18, 0xddd2, 0xddcd
	}
	seen := map[uint16]bool{}
	for head != 0 {
		if seen[head] {
			return false, fmt.Errorf("cyclic native debug scenery chain")
		}
		seen[head] = true
		at := cleanupRecordAddress(NativeRecordReference(head))
		if m.byte(at) == kind {
			art := m.word(at + 10)
			commandWord(c, 3, art)
			for i := 0; i < 4; i++ {
				v, e := rules.word(table + i*2)
				if e != nil {
					return false, e
				}
				if v == art {
					next, e := rules.word(table + ((i+1)%4)*2)
					if e != nil {
						return false, e
					}
					m.putWord(at+10, next)
					return false, m.err
				}
			}
		}
		head = m.word(at + 2)
		commandWord(c, 3, head)
	}
	at, e := primitiveFreeRecord(m.m, 0x6bd0, 0x76c0, 14)
	if e != nil || at == 0 {
		return false, e
	}
	age, e := rules.byte(ageAt)
	if e != nil {
		return false, e
	}
	art, e := rules.word(table)
	if e != nil {
		return false, e
	}
	for _, v := range []struct {
		offset int
		value  uint8
	}{{12, 3}, {0, kind}, {6, x}, {8, y}, {1, age}, {7, 128}, {9, 128}} {
		m.putByte(at+v.offset, v.value)
	}
	m.putWord(at+10, art)
	commandWord(c, 1, m.word(tsunamiGrid(packed)+2))
	ref := NativeRecordReference(uint16(at - 0x76c0))
	c.D[0] = uint32(int32(at - 0x76c0))
	if m.err != nil {
		return false, m.err
	}
	return false, w.nativeRuntimeInsert(ref)
}

func (w *World) commandNeutralCreation(call NativeCommandCall) (bool, error) {
	c := call.Context
	selector := uint16(c.D[2])
	rules := NativeCommandRules{Code: w.NativeAI.Code}
	cb := NativeNeutralCallbacks{Memory: w.nativeCleanupMemory()}
	cb.Insert = func(ref NativeRecordReference) error {
		at := cleanupRecordAddress(ref)
		m := nativeWhirlwindMemory{m: cb.Memory}
		packed := uint16(m.byte(at+8))<<8 | uint16(m.byte(at+6))
		commandWord(c, 1, m.word(tsunamiGrid(packed)+2))
		if m.err != nil {
			return m.err
		}
		return w.nativeRuntimeInsert(ref)
	}
	step, e := CreateNativeNeutral(FollowerCleanupRegisters{D0: c.D[0], D1: c.D[1], D2: c.D[2]}, cb)
	if e != nil {
		return false, e
	}
	if len(step.Records) != 0 {
		offset, e := rules.word(0x1321c + int(int16(selector)))
		if e != nil {
			return false, e
		}
		commandWord(c, 2, offset)
	}
	c.D[0] = 0
	if step.Created {
		c.D[0] = 1
	}
	return !step.Created, nil
}

// commandRemove is $dff4. It skips marker records and removes one eligible
// record from the actual cell chain, including a retained dead follower.
func (w *World) commandRemove(call NativeCommandCall, trace func(NativeCommandCall)) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	packed := uint16(uint8(c.D[1]))<<8 | uint16(uint8(c.D[0]))
	commandWord(c, 1, packed&0xff00|uint16(uint8(packed)*4))
	head := m.word(tsunamiGrid(packed) + 2)
	commandWord(c, 0, head)
	seen := map[uint16]bool{}
	for head != 0 {
		if seen[head] {
			return false, fmt.Errorf("cyclic native remove chain")
		}
		seen[head] = true
		at := cleanupRecordAddress(NativeRecordReference(head))
		if at >= 0x76c0 && at < 0xc800 {
			return false, w.commandFollowerCleanup(NativeRecordReference(head), 0, call, trace)
		}
		if at < 0x76c0 || at < 0xe740 {
			m.putByte(at+12, 0)
			if m.err != nil {
				return false, m.err
			}
			return false, w.nativeRuntimeUnlink(NativeRecordReference(head))
		}
		head = m.word(at + 2)
		commandWord(c, 0, head)
	}
	return true, m.err
}
