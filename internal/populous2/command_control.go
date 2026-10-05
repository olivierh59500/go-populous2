package populous2

// commandMessage is $4f8e. Command114 edits the original11-character message
// buffer, with its exact shift/backspace/terminator writes and D0 counter.
// It neither sends a new external message nor substitutes a modal UI action.
func (w *World) commandMessage(call NativeCommandCall) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	key, owner := uint8(c.D[1]), uint8(c.D[2])
	if key != 32 && key != 8 && key != 13 && (int8(key) < 65 || int8(key) > 90) {
		return false, nil
	}
	m.putWord(0xf14, 250)
	base := 0xf16
	if owner != 1 {
		base = 0xf22
	}
	pointer := base
	c.D[0] = 10
	for {
		v := m.byte(pointer)
		pointer++
		if v == 0 {
			break
		}
		commandWord(c, 0, uint16(c.D[0])-1)
		if uint16(c.D[0]) == 0xffff {
			if key == 8 {
				m.putByte(pointer-2, 0)
				return false, m.err
			}
			for at := base; at < pointer; at++ {
				m.putByte(at, m.byte(at+1))
			}
			break
		}
	}
	if uint8(c.D[0]) == 10 {
		m.putByte(pointer-1, 0)
		if key == 8 {
			return false, m.err
		}
	}
	if key == 8 {
		m.putByte(pointer-2, 0)
	} else if int8(key) >= 32 {
		m.putByte(pointer-1, key)
		m.putByte(pointer, 0)
	}
	return false, m.err
}

// commandSwitchProfile is $111ae. Eight bytes swap XP and the bolt word,
// followed by control/identity and the complete transport word. Modes6/8
// retain both transport records. The saved pointer uses the runtime's explicit
// BSS-relative convention, matching the initialization adapter.
func (w *World) commandSwitchProfile(call NativeCommandCall) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	desired := uint16(c.D[0])
	old := m.word(0xeb42)
	commandWord(c, 1, old)
	destination, source := primitiveDeityAddress(desired), primitiveDeityAddress(old)
	c.D[2] = uint32(old) * 314
	for i := 0; i < 8; i++ {
		value := m.byte(destination + 0x52 + i)
		commandByte(c, 2, value)
		m.putByte(destination+0x52+i, m.byte(source+0x52+i))
		m.putByte(source+0x52+i, value)
	}
	for _, offset := range []int{0x1a, 0x18} {
		value := m.word(destination + offset)
		commandWord(c, 2, value)
		m.putWord(destination+offset, m.word(source+offset))
		m.putWord(source+offset, value)
	}
	c.D[2] = uint32(desired) * 10
	destinationCommand := 0xeb4c + int(int16(uint16(c.D[2])))
	if m.err == nil {
		m.err = m.m.Write32(0xeb6a, uint32(destinationCommand))
	}
	c.D[2] = uint32(old) * 10
	sourceCommand := 0xeb4c + int(int16(uint16(c.D[2])))
	transport := m.word(destinationCommand + 8)
	commandWord(c, 2, transport)
	if transport != 6 && transport != 8 {
		m.putWord(destinationCommand+8, m.word(sourceCommand+8))
		m.putWord(sourceCommand+8, transport)
	}
	m.putWord(0xeb42, desired)
	if m.err != nil {
		return false, m.err
	}
	w.NativeProfileSide = uint8(desired)
	for side := 0; side < 2; side++ {
		god := 0xe8a4 + side*314
		for i := 0; i < 6; i++ {
			w.Experience[side][i] = m.byte(god + 0x52 + i)
		}
	}
	w.Deity.Bolts = m.word(destination + 0x58)
	return false, m.err
}
