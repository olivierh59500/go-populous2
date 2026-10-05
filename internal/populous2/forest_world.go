package populous2

import legacy "go-populous2/internal/legacy"

func (w *World) nativeForestCallbacks() ForestNativeCallbacks {
	return ForestNativeCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Link: w.nativeRuntimeInsert, Unlink: w.nativeRuntimeUnlink, DestroyTown: w.nativeDestroyTown}
}

func (w *World) castNativeForest(player, x, y int) bool {
	created := false
	err := w.runNativeFollowerCall(func() error {
		step, err := w.ForestNative.Cast(uint8(player+1), uint8(x), uint8(y), w.nativeForestCallbacks())
		created = step.Count != 0
		return err
	})
	if err != nil {
		panic(err)
	}
	return created
}

func (w *World) nativeRenewCallbacks(player int) RenewNativeCallbacks {
	m := w.nativeCleanupMemory()
	write := m.Write8
	m.Write8 = func(address int, value uint8) error {
		if err := write(address, value); err != nil {
			return err
		}
		if address >= 0xf45 && address < 0x4f44 && (address-0xf44)%4 == 1 && value == 245 {
			pos := (address - 0xf44) / 4
			w.Marks[pos].Spell, w.Marks[pos].Player = Flowers, player
			if w.Core.MapBlk[pos] == legacy.BadLand {
				w.Core.MapBlk[pos] = legacy.FlatBlock
			}
		}
		return nil
	}
	return RenewNativeCallbacks{Memory: m, Random: func() uint16 { return uint16(w.random()) }}
}

func (w *World) castNativeRenew(player, x, y int) {
	err := w.runNativeFollowerCall(func() error {
		_, err := w.RenewNative.Create(uint16(player+1), uint8(x), uint8(y), w.nativeRenewCallbacks(player))
		return err
	})
	if err != nil {
		panic(err)
	}
}

// tickScenery preserves the original owner-byte scan and signed aging/burning
// across the complete shared pool. Inner town/fire callbacks see raw writes
// until the complete pass has returned to the inherited presentation bridge.
func (w *World) tickScenery() {
	err := w.runNativeFollowerCall(func() error {
		for index := range SceneryCapacity {
			if _, err := w.ForestNative.Tick(nativeActorReference(NativeSceneryPool, index), uint16(w.Core.GameTurn), w.nativeForestCallbacks()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}
