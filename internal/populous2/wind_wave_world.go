package populous2

import "fmt"

// NativeEnvironmentPointerBase retains the original relocated BSS pointer
// basis for $15a94's comparison with the value in the neutral deity record.
// The Go memory image uses offsets; this fixed basis keeps that native alias
// deterministic without exposing host pointers.
const NativeEnvironmentPointerBase uint32 = 0x200000

func (w *World) hurricaneCallbacks() HurricaneCallbacks {
	return HurricaneCallbacks{Memory: w.nativeCleanupMemory(), Move: w.nativeTerrainCallbacks().Move, Cleanup: w.cleanupNativeFollower, Unlink: w.nativeRuntimeUnlink,
		WriteOverlay: func(index int, value uint8) error { w.NativeOverlays[index] = value; return nil }, PointerBase: NativeEnvironmentPointerBase}
}

func (w *World) castNativeHurricane(player, x, y int, direction uint16) bool {
	admitted := false
	err := w.runNativeFollowerCall(func() error {
		step, err := w.HurricaneRules.Create(uint8(player+1), uint8(x), uint8(y), direction, w.hurricaneCallbacks())
		if step.Admitted {
			location, _ := LocateNativeRecord(step.Reference)
			w.NativeEnvironment[location.Index] = NativeEnvironmentHurricane
		}
		admitted = step.Admitted
		return err
	})
	if err != nil {
		panic(err)
	}
	return admitted
}

func (w *World) tsunamiCallbacks() TsunamiCallbacks {
	return TsunamiCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink,
		Link: func(ref NativeRecordReference) error {
			if err := w.nativeRuntimeInsert(ref); err != nil {
				return err
			}
			location, _ := LocateNativeRecord(ref)
			w.NativeEnvironment[location.Index] = NativeEnvironmentTsunami
			return nil
		},
		Move: func(ref NativeRecordReference, x, y uint16) (bool, error) {
			changed, err := w.Occupancy.Grid.Move(ref, x, y, w.runtimeMemory().RecordAccess())
			if err == nil {
				w.hydrateNativeRuntimeGraph()
			}
			return changed, err
		},
		Lower: func(x, y uint8) error {
			before := w.Core.Alt
			if w.Core.DirectLowerTerrain(int(x), int(y)) {
				w.clearChangedGround(before)
				w.refreshChangedTerrain(before)
			}
			return nil
		}}
}

func (w *World) castNativeTsunami(player, x, y int) {
	err := w.runNativeFollowerCall(func() error {
		_, err := w.TsunamiRules.Create(uint16(player+1), uint8(x), uint8(y), w.tsunamiCallbacks())
		return err
	})
	if err != nil {
		panic(err)
	}
}

// Older saves held one provisional direction/radius effect. Preserve one
// active controller and its remaining wind lifetime without recasting, mana
// charges or random draws; waves adopt their original fixed movement bank.
func (w *World) migrateLegacyWindWaves() error {
	return w.runNativeFollowerCall(func() error {
		live := w.Effects[:0]
		m := w.nativeCleanupMemory()
		for _, effect := range w.Effects {
			if effect.Spell != Wind && effect.Spell != Tsunami {
				live = append(live, effect)
				continue
			}
			direction := uint16(0)
			if abs(effect.DX) >= abs(effect.DY) && effect.DX != 0 {
				direction = 2
				if effect.DX < 0 {
					direction = 6
				}
			} else if effect.DY > 0 {
				direction = 4
			}
			if effect.Spell == Wind {
				step, err := w.HurricaneRules.Create(uint8(effect.Player+1), uint8(effect.X), uint8(effect.Y), direction, w.hurricaneCallbacks())
				if err != nil {
					return err
				}
				if !step.Admitted {
					return fmt.Errorf("saved wind exceeds native shared pool")
				}
				location, _ := LocateNativeRecord(step.Reference)
				w.NativeEnvironment[location.Index] = NativeEnvironmentHurricane
				if err := m.Write16(cleanupRecordAddress(step.Reference)+24, uint16(max(1, effect.Life))); err != nil {
					return err
				}
				continue
			}
			a, err := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
			if err != nil {
				return err
			}
			if a == 0 {
				return fmt.Errorf("saved wave exceeds native shared pool")
			}
			index := direction / 2
			for _, field := range []struct {
				offset int
				value  uint8
			}{{0, 0x32}, {12, uint8(effect.Player + 1)}, {22, 0x2a}} {
				if err := m.Write8(a+field.offset, field.value); err != nil {
					return err
				}
			}
			for _, field := range []struct {
				offset int
				value  uint16
			}{{6, uint16(effect.X*256 + 128)}, {8, uint16(effect.Y*256 + 128)}, {10, w.TsunamiRules.Animations[index]}, {14, uint16(w.TsunamiRules.Vectors[index][0] * int16(w.TsunamiRules.Speed))}, {16, uint16(w.TsunamiRules.Vectors[index][1] * int16(w.TsunamiRules.Speed))}, {24, w.TsunamiRules.Life}, {26, index * 4}} {
				if err := m.Write16(a+field.offset, field.value); err != nil {
					return err
				}
			}
			if err := w.tsunamiCallbacks().Link(NativeRecordReference(a - 0x76c0)); err != nil {
				return err
			}
		}
		w.Effects = live
		return nil
	})
}
