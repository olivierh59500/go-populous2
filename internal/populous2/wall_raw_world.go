package populous2

import (
	"encoding/binary"
	"fmt"
)

// nativeWallCallbacks uses the original mixed actor graph and retained record
// image. SourceD7 is supplied by the original command register continuation;
// the creator replaces its low mask byte only, so zero is not an implied value.
func (w *World) nativeWallCallbacks(sourceD7 uint16) NativeWallCallbacks {
	memory := w.nativeCleanupMemory()
	entry := w.nativeEntryCallbacks()
	// Original entry uses signed owner-byte multiplication and ADDA.W.
	// Neutral owner3 therefore reaches the retained command region at$eb18,
	// rather than a fourth fabricated deity or an unsupported scalar player.
	entry.GodMode = func(owner uint8) (uint16, error) { return memory.Read16(heroGodAddress(owner) + 12) }
	entry.SetLeader = func(owner uint8, ref NativeRecordReference) error {
		return memory.Write16(heroGodAddress(owner)+8, uint16(ref))
	}
	entry.Founded = func(owner uint8) error {
		god := heroGodAddress(owner)
		value, err := memory.Read16(god + 0x44)
		if err != nil {
			return err
		}
		return memory.Write16(god+0x44, value+1)
	}
	callbacks := NativeWallCallbacks{
		Memory: memory, SourceD7: sourceD7,
		Link: w.nativeRuntimeInsert, Unlink: w.nativeRuntimeUnlink,
		Move: func(ref NativeRecordReference, x, y uint16) (bool, error) {
			changed, err := w.Occupancy.Grid.Move(ref, x, y, w.runtimeMemory().RecordAccess())
			if err == nil {
				w.hydrateNativeRuntimeGraph()
			}
			return changed, err
		},
		Enter: func(ref NativeRecordReference) error {
			_, err := w.FollowerEntry.Enter(ref, entry)
			return err
		},
	}
	if w.nativeSharedCode != nil {
		callbacks.WriteNeighbor = func(index int, ref NativeRecordReference) error {
			if index < 0 || index >= 4 || len(w.NativeAI.Code) < 0x1648c {
				return fmt.Errorf("native wall CODE neighbor slot%d unavailable", index)
			}
			value := uint32(0)
			if ref != 0 {
				// 16332's relocated LEA supplies the actual base76C0;
				// its following ADDA.W uses the signed raw reference.
				base := binary.BigEndian.Uint32(w.NativeAI.Code[0x16334:])
				value = base + uint32(int32(int16(ref)))
			}
			binary.BigEndian.PutUint32(w.NativeAI.Code[0x1647c+index*4:], value)
			return nil
		}
	}
	return callbacks
}

// castNativeWall is the $17800 command boundary. A failed disconnected cast
// can still update the deity's retained head; the raw scope hydrates that
// change even when the creator returns false. Mana/weighted cast statistics
// remain at the caller's command-admission boundary.
func (w *World) castNativeWall(player, x, y int, sourceD7 uint16, rules *NativeWallRules, placement *NativeWallPlacementState) bool {
	created := false
	err := w.runNativeFollowerCall(func() error {
		step, err := rules.Create(uint16(player+1), uint8(x), uint8(y), placement, w.nativeWallCallbacks(sourceD7))
		if err != nil {
			return err
		}
		created = step.Created
		if !created {
			return nil
		}
		memory := w.nativeCleanupMemory()
		god := heroGodAddress(uint8(player + 1))
		metric, err := memory.Read16(god + 0x44)
		if err != nil {
			return err
		}
		return memory.Write16(god+0x44, metric+1)
	})
	if err != nil {
		panic(err)
	}
	return created
}

// tickNativeWallPool is the single original $161cc pass, after FX and before
// scenery. Generic WallState.Tick must not run in addition to this controller.
func (w *World) tickNativeWallPool(rules *NativeWallRules) error {
	return w.runNativeFollowerCall(func() error {
		_, err := rules.TickPool(w.nativeWallCallbacks(0))
		return err
	})
}
