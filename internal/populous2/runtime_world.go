package populous2

import "fmt"

func (w *World) runtimeMemory() NativeRuntimeMemory {
	return NativeRuntimeMemory{Records: &w.RecordImage, Globals: &w.NativeGlobals}
}

func (w *World) initializeNativeRuntime() {
	w.reconcileActorGraph()
	for index, entry := range w.Occupancy.Magnets {
		if entry.Linked {
			w.unlinkActor(NativeMagnetPool, index)
		}
	}
	w.NativeGlobals = NativeGlobalImage{}
	w.refreshNativeRecordImage()
	memory := w.runtimeMemory()
	for player := range w.Core.Magnets {
		owner := uint8(player + 1)
		pos := w.Core.Magnets[player].GoTo
		if err := w.MagnetRules.Create(owner, uint8(pos%64), uint8(pos/64), memory, &w.Occupancy.Grid); err != nil {
			panic(err)
		}
		w.Occupancy.Magnets[owner].Linked = true
	}
	w.hydrateNativeRuntimeGraph()
	w.syncNativeRuntimeBridge()
}

// syncNativeRuntimeBridge patches the fields still controlled by the
// inherited engine. Original statistic and state-specific deity bytes survive.
// Native handlers hydrate their changes back into Core before this bridge runs.
func (w *World) syncNativeRuntimeBridge() {
	memory := w.runtimeMemory()
	for player, magnet := range w.Core.Magnets {
		owner := uint8(player + 1)
		deity, _ := NativeDeityAddress(owner)
		if _, err := memory.Write32(deity, uint32(magnet.Mana)); err != nil {
			panic(err)
		}
		attrition := w.Level.Players[player].FollowerAttrition()
		if w.Core.FollowerAttrition != nil {
			attrition = w.Core.FollowerAttrition(player, false)
		}
		if _, err := memory.Write32(deity+20, uint32(attrition)); err != nil {
			panic(err)
		}
		for element, experience := range w.Experience[player] {
			if _, err := memory.Write8(deity+0x52+element, experience); err != nil {
				panic(err)
			}
		}
		if owner == w.NativeProfileSide {
			if _, err := memory.Write16(deity+0x58, w.Deity.Bolts); err != nil {
				panic(err)
			}
		}
		leader := uint16(0)
		if magnet.Carried > 0 && magnet.Carried <= NativeWorldFollowerCapacity {
			leader = uint16(nativeActorReference(NativeFollowerPool, magnet.Carried-1))
		}
		for _, field := range []struct {
			offset int
			value  uint16
		}{{8, leader}, {12, uint16(w.nativeFollowerMode(player))}, {0x4a, w.Rules[player].Raw}} {
			if _, err := memory.Write16(deity+field.offset, field.value); err != nil {
				panic(err)
			}
		}
		// Controlled oracle worlds can deliberately omit the marker records.
		// Existing markers follow inherited position edits through the native
		// remove/insert operation instead of increasing movement pressure.
		if w.Occupancy.Magnets[owner].Linked {
			ref, _ := NativeMagnetReference(owner)
			record, _ := memory.RecordAccess().Record(ref)
			x, y := uint16((magnet.GoTo%64)*256+128), uint16((magnet.GoTo/64)*256+128)
			if record.X != x || record.Y != y {
				if err := w.MagnetRules.Relocate(owner, uint8(magnet.GoTo%64), uint8(magnet.GoTo/64), memory, &w.Occupancy.Grid); err != nil {
					panic(err)
				}
				w.hydrateNativeRuntimeGraph()
			}
		}
	}
}

func (w *World) nativeRuntimeUnlink(ref NativeRecordReference) error {
	entry, ok := w.Occupancy.entry(ref)
	if !ok {
		return fmt.Errorf("native runtime unlink requires an actor or marker entity")
	}
	if err := w.Occupancy.Grid.Remove(ref, w.runtimeMemory().RecordAccess()); err != nil {
		return err
	}
	entry.Linked = false
	w.hydrateNativeRuntimeGraph()
	return nil
}

func (w *World) nativeRuntimeInsert(ref NativeRecordReference) error {
	if location, ok := LocateNativeRecord(ref); ok && location.Pool == NativeEffectPool {
		w.NativeEnvironment[location.Index] = NativeEnvironmentNone
	}
	entry, ok := w.Occupancy.entry(ref)
	if !ok {
		return fmt.Errorf("native runtime insert requires an actor or marker entity")
	}
	record, ok := w.runtimeMemory().RecordAccess().Record(ref)
	if !ok {
		return fmt.Errorf("native runtime insert has no common record prefix")
	}
	index, err := nativeOccupancyIndex(record.X, record.Y)
	if err != nil {
		return err
	}
	if err := w.Occupancy.Grid.Insert(ref, index%64, index/64, w.runtimeMemory().RecordAccess()); err != nil {
		return err
	}
	entry.Linked = true
	w.hydrateNativeRuntimeGraph()
	return nil
}

func (w *World) nativeCleanupMemory() FollowerCleanupMemory {
	memory := w.runtimeMemory()
	syncRNG := func() {
		var value uint32
		for i := range 4 {
			value = value<<8 | uint32(w.NativeCommandBytes[0xeb28-0xeb18+i])
		}
		w.Core.SetRandomState(value)
	}
	read8 := func(address int) (uint8, error) {
		switch {
		case address >= 0xf40 && address < 0xf44:
			return uint8(w.NativeClock >> uint(24-(address-0xf40)*8)), nil
		case address == 0xf12:
			return uint8(w.NativeRaiseEnabled >> 8), nil
		case address == 0xf13:
			return uint8(w.NativeRaiseEnabled), nil
		case address >= 0xdc4 && address < 0xf44:
			return w.NativeControlBytes[address-0xdc4], nil
		case address >= 0xf44 && address < 0x4f44:
			value, _ := w.Occupancy.Byte(address - 0xf44)
			return value, nil
		case address >= 0x4f44 && address < 0x5f44:
			return w.NativeOverlays[address-0x4f44], nil
		case address >= 0x5f44 && address < 0x5f50:
			return w.NativeViewBytes[address-0x5f44], nil
		case address >= 0xeb18 && address < 0xeb90:
			return w.NativeCommandBytes[address-0xeb18], nil
		case address >= 0xeb90 && address < 0x11280:
			return w.NativeRedrawBytes[address-0xeb90], nil
		default:
			return memory.Read8(address)
		}
	}
	write8 := func(address int, value uint8) error {
		switch {
		case address >= 0xf40 && address < 0xf44:
			shift := uint(24 - (address-0xf40)*8)
			w.NativeClock = w.NativeClock&^(uint32(255)<<shift) | uint32(value)<<shift
		case address == 0xf12:
			w.NativeRaiseEnabled = uint16(value)<<8 | w.NativeRaiseEnabled&255
		case address == 0xf13:
			w.NativeRaiseEnabled = w.NativeRaiseEnabled&0xff00 | uint16(value)
		case address >= 0xdc4 && address < 0xf44:
			w.NativeControlBytes[address-0xdc4] = value
		case address >= 0xf44 && address < 0x4f44:
			offset, pos := address-0xf44, (address-0xf44)/4
			switch offset & 3 {
			case 0:
				w.Occupancy.Grid.Cells[pos].Header = value
			case 1:
				w.writeNativeTownTile(pos%64, pos/64, value)
			case 2:
				w.Occupancy.Grid.Cells[pos].Head = NativeRecordReference(value)<<8 | w.Occupancy.Grid.Cells[pos].Head&255
			case 3:
				w.Occupancy.Grid.Cells[pos].Head = w.Occupancy.Grid.Cells[pos].Head&0xff00 | NativeRecordReference(value)
			}
		case address >= 0x4f44 && address < 0x5f44:
			w.NativeOverlays[address-0x4f44] = value
		case address >= 0x5f44 && address < 0x5f50:
			w.NativeViewBytes[address-0x5f44] = value
		case address >= 0xeb18 && address < 0xeb90:
			w.NativeCommandBytes[address-0xeb18] = value
			if address >= 0xeb28 && address < 0xeb2c {
				syncRNG()
			}
		case address >= 0xeb90 && address < 0x11280:
			w.NativeRedrawBytes[address-0xeb90] = value
		default:
			_, err := memory.Write8(address, value)
			return err
		}
		return nil
	}
	// Word/long accesses share byte routing across all retained seams. Native
	// aliases can straddle grid, overlay, camera, pool and deity boundaries.
	read := func(address, width int) (uint32, error) {
		var value uint32
		for offset := range width {
			part, err := read8(address + offset)
			if err != nil {
				return 0, err
			}
			value = value<<8 | uint32(part)
		}
		return value, nil
	}
	write := func(address, width int, value uint32) error {
		for offset := range width {
			if err := write8(address+offset, uint8(value>>uint(8*(width-offset-1)))); err != nil {
				return err
			}
		}
		return nil
	}
	return FollowerCleanupMemory{
		Read8: read8, Write8: write8,
		Read16:  func(address int) (uint16, error) { value, err := read(address, 2); return uint16(value), err },
		Read32:  func(address int) (uint32, error) { return read(address, 4) },
		Write16: func(address int, value uint16) error { return write(address, 2, uint32(value)) },
		Write32: func(address int, value uint32) error { return write(address, 4, value) },
	}
}

func (w *World) cleanupNativeFollower(ref NativeRecordReference, mode uint16) error {
	outer := w.nativeCallDepth == 0
	if outer {
		w.refreshNativeRecordImage()
		w.syncNativeRuntimeBridge()
	}
	w.nativeCallDepth++
	defer func() {
		w.nativeCallDepth--
		if outer {
			w.hydrateNativeRuntimeRecords()
		}
	}()
	_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
	return err
}
