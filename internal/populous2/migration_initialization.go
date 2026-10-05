package populous2

import (
	"bytes"
	"encoding/binary"
)

type nativeInitializationDefaults struct {
	Globals  NativeGlobalImage
	Commands [0x78]byte
}

func captureNativeInitializationDefaults(w *World) nativeInitializationDefaults {
	return nativeInitializationDefaults{Globals: w.NativeGlobals, Commands: w.NativeCommandBytes}
}

// migrateNativeInitialization fills only metadata absent from prototype Go
// saves. A valid native GAM profile word distinguishes actual saved images;
// their opaque bytes/templates must not be reconstructed from campaign data.
// Full startup cannot be replayed: it resets live modes, XP, bolts and script.
func (w *World) migrateNativeInitialization(version int, defaults nativeInitializationDefaults) error {
	if version >= 27 || binary.BigEndian.Uint16(w.NativeCommandBytes[0xeb42-0xeb18:]) != 0 {
		return nil
	}
	m := w.nativeCleanupMemory()
	for player := range 2 {
		god, _ := NativeDeityAddress(uint8(player + 1))
		at := god + 0x5a - NativeMagnetImageStart
		missing := true
		for _, value := range w.NativeGlobals.Bytes[at : at+58] {
			missing = missing && value == 0
		}
		if missing {
			copy(w.NativeGlobals.Bytes[at:at+58], defaults.Globals.Bytes[at:at+58])
			if err := m.Write16(god+0x66, w.Rules[player].Raw); err != nil {
				return err
			}
			// Compile only a genuinely missing choice list. Preserve the live
			// policy counter reset by the original compiler at God+26.
			first, _ := m.Read16(god + 0x94)
			second, _ := m.Read16(god + 0x96)
			policyMissing := first == 0 && second == 0
			for _, value := range w.NativeGlobals.Bytes[god+0x9c-NativeMagnetImageStart : god+0x11c-NativeMagnetImageStart] {
				policyMissing = policyMissing && value == 0
			}
			if policyMissing {
				counter, _ := m.Read16(god + 0x26)
				if err := w.NativeAI.CompileChoices(god, m); err != nil {
					return err
				}
				if err := m.Write16(god+0x26, counter); err != nil {
					return err
				}
			}
		}
		for _, field := range []int{0x18, 0x1a} {
			current, _ := m.Read16(god + field)
			if current != 0 {
				continue
			}
			offset := god + field - NativeMagnetImageStart
			value := binary.BigEndian.Uint16(defaults.Globals.Bytes[offset:])
			if field == 0x1a {
				value = 4
				if player+1 == int(w.NativeProfileSide) {
					value = 2
				}
			}
			if err := m.Write16(god+field, value); err != nil {
				return err
			}
		}
	}
	for _, field := range []struct {
		address int
		value   uint16
	}{{0xeb22, uint16(w.Level.Terrain)}, {0xeb2c, w.Rules[0].Raw}, {0xeb2e, w.Rules[1].Raw}, {0xeb42, uint16(w.NativeProfileSide)}, {0xeb44, w.NativeGameMode}, {0xeb46, uint16(w.Level.Number)}} {
		if err := m.Write16(field.address, field.value); err != nil {
			return err
		}
	}
	if err := m.Write32(0xeb24, w.Level.RandomSeed); err != nil {
		return err
	}
	if err := m.Write32(0xeb28, w.Core.RandomState()); err != nil {
		return err
	}
	name := w.NativeCommandBytes[0xeb30-0xeb18 : 0xeb40-0xeb18]
	if len(bytes.Trim(name, "\x00")) == 0 && w.Deity.Name != "" {
		// Older JSON profiles permitted longer names. Keep that live name;
		// only its newly introduced native display field has a bounded prefix.
		length := min(len(w.Deity.Name), len(name)-1)
		copy(name, []byte(w.Deity.Name[:length]))
		name[length] = 0
	}
	for _, address := range []int{0xeb56, 0xeb60, 0xeb5e, 0xeb68} {
		at := address - 0xeb18
		if w.NativeCommandBytes[at] == 0 {
			w.NativeCommandBytes[at] = defaults.Commands[at]
		}
	}
	pointer := w.NativeCommandBytes[0xeb6a-0xeb18 : 0xeb6e-0xeb18]
	if binary.BigEndian.Uint32(pointer) == 0 {
		copy(pointer, defaults.Commands[0xeb6a-0xeb18:0xeb6e-0xeb18])
	}
	return nil
}
