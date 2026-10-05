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
		leader := uint16(0)
		if magnet.Carried > 0 && magnet.Carried <= NativeWorldFollowerCapacity {
			leader = uint16(nativeActorReference(NativeFollowerPool, magnet.Carried-1))
		}
		for _, field := range []struct {
			offset int
			value  uint16
		}{{8, leader}, {12, uint16(w.nativeFollowerMode(player))}} {
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
	read8 := func(address int) (uint8, error) {
		if address >= 0xf44 && address < 0x4f44 {
			value, _ := w.Occupancy.Byte(address - 0xf44)
			return value, nil
		}
		return memory.Read8(address)
	}
	return FollowerCleanupMemory{
		Read8: read8,
		Read16: func(address int) (uint16, error) {
			if address >= 0xf44 && address < 0x4f44-1 {
				high, _ := read8(address)
				low, _ := read8(address + 1)
				return uint16(high)<<8 | uint16(low), nil
			}
			return memory.Read16(address)
		},
		Read32: func(address int) (uint32, error) {
			if address >= 0xf44 && address < 0x4f44-3 {
				var value uint32
				for offset := range 4 {
					part, _ := read8(address + offset)
					value = value<<8 | uint32(part)
				}
				return value, nil
			}
			return memory.Read32(address)
		},
		Write8:  func(address int, value uint8) error { _, err := memory.Write8(address, value); return err },
		Write16: func(address int, value uint16) error { _, err := memory.Write16(address, value); return err },
		Write32: func(address int, value uint32) error { _, err := memory.Write32(address, value); return err },
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
