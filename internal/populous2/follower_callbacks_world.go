package populous2

import "fmt"

// runNativeFollowerCall keeps original routines' intermediate byte writes
// authoritative until the complete call returns to the inherited adapter.
func (w *World) runNativeFollowerCall(call func() error) error {
	outer := w.nativeCallDepth == 0
	if outer {
		w.reconcileActorGraph()
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
	return call()
}

func (w *World) nativePackedHead(tile NativePackedTile) (NativeRecordReference, error) {
	x, y := int(uint8(tile)), int(uint8(uint16(tile)>>8))
	if !inside(x, y) {
		return 0, fmt.Errorf("native follower map head outside world")
	}
	return w.Occupancy.Grid.Cells[x+y*64].Head, nil
}

func (w *World) nativePackedTile(tile NativePackedTile) (uint8, error) {
	x, y := int(uint8(tile)), int(uint8(uint16(tile)>>8))
	if !inside(x, y) {
		return 0, fmt.Errorf("native follower terrain outside world")
	}
	return w.nativeTileAt(x, y), nil
}

func (w *World) nativeEntryCallbacks() FollowerEntryCallbacks {
	memory := w.runtimeMemory()
	return FollowerEntryCallbacks{
		Head: w.nativePackedHead,
		Node: func(ref NativeRecordReference) (FollowerEntryNode, error) {
			at := cleanupRecordAddress(ref)
			kind, err := memory.Read8(at)
			if err != nil {
				return FollowerEntryNode{}, err
			}
			owner, err := memory.Read8(at + 12)
			if err != nil {
				return FollowerEntryNode{}, err
			}
			next, err := memory.Read16(at + 2)
			return FollowerEntryNode{Kind: kind, Owner: owner, Next: NativeRecordReference(next)}, err
		},
		Read: w.readEntryRecord, Write: w.writeEntryRecord,
		ReadWord: func(ref NativeRecordReference, offset uint16) (uint16, error) {
			return memory.Read16(cleanupRecordAddress(ref) + int(offset))
		},
		ReadLong: func(ref NativeRecordReference, offset uint16) (uint32, error) {
			return memory.Read32(cleanupRecordAddress(ref) + int(offset))
		},
		WriteWord: func(ref NativeRecordReference, offset, value uint16) error {
			_, err := memory.Write16(cleanupRecordAddress(ref)+int(offset), value)
			return err
		},
		Tile: w.nativePackedTile,
		GodMode: func(owner uint8) (uint16, error) {
			deity, ok := NativeDeityAddress(owner)
			if !ok {
				return 0, fmt.Errorf("native follower deity owner outside records")
			}
			return memory.Read16(deity + 12)
		},
		Tick: func() uint16 { return uint16(w.Core.GameTurn) },
		SetLeader: func(owner uint8, ref NativeRecordReference) error {
			deity, ok := NativeDeityAddress(owner)
			if !ok {
				return fmt.Errorf("native follower leader owner outside records")
			}
			_, err := memory.Write16(deity+8, uint16(ref))
			return err
		},
		Selected: func() NativeRecordReference { return w.NativeSelected },
		Select:   func(ref NativeRecordReference) error { w.NativeSelected = ref; return nil },
		Founded: func(owner uint8) error {
			deity, ok := NativeDeityAddress(owner)
			if !ok {
				return fmt.Errorf("native founding owner outside records")
			}
			value, err := memory.Read16(deity + 0x44)
			if err == nil {
				_, err = memory.Write16(deity+0x44, value+1)
			}
			return err
		},
		Unlink: w.nativeRuntimeUnlink, EvaluateTown: w.evaluateNativeTown, ClearFarms: w.clearNativeFarms,
		ClearHeroLinks: func(ref NativeRecordReference) error { _, err := ClearEntryHeroLinks(&w.RecordImage, ref); return err },
		Sound: func(raw uint16) error {
			if raw%10 != 0 || raw/10 >= 133 {
				return fmt.Errorf("native follower sound outside sample bank")
			}
			w.effectSoundCues = append(w.effectSoundCues, int(raw/10))
			return nil
		},
	}
}

func (w *World) nativeTownCombatCallbacks() TownCombatCallbacks {
	return TownCombatCallbacks{Read: w.readEntryRecord, Write: w.writeEntryRecord, ClearFarms: w.clearNativeFarms, EvaluateTown: w.evaluateNativeTown, Cleanup: w.cleanupNativeFollower}
}

func (w *World) nativeDestroyTown(ref NativeRecordReference) error {
	_, err := w.TownCombat.Destroy(ref, w.nativeTownCombatCallbacks())
	return err
}

func (w *World) nativeReformTown(ref, originalA0 NativeRecordReference) error {
	_, err := w.TownCombat.Reform(ref, originalA0, uint16(w.Core.GameTurn), w.nativeTownCombatCallbacks())
	return err
}

func (w *World) nativeWinnerCallbacks() FollowerWinCallbacks {
	return FollowerWinCallbacks{Memory: w.nativeCleanupMemory(), Cleanup: w.cleanupNativeFollower, ClearFarms: w.clearNativeFarms, ReformTown: w.nativeReformTown, DestroyTown: w.nativeDestroyTown, PoolBlocked: func() bool { return w.NativeBirthBlocked }, Insert: w.nativeRuntimeInsert}
}

func (w *World) nativeCombatCallbacks(originalA0 NativeRecordReference) FollowerCombatCallbacks {
	return FollowerCombatCallbacks{Read: w.readEntryRecord, Write: w.writeEntryRecord, Random: w.random, Cleanup: w.cleanupNativeFollower, Win: func(winner, loser NativeRecordReference) error {
		_, err := w.FollowerWin.Win(winner, loser, originalA0, w.nativeWinnerCallbacks())
		return err
	}}
}

func (w *World) nativeAftermathCallbacks() FollowerAftermathCallbacks {
	return FollowerAftermathCallbacks{Read: w.readEntryRecord, Write: w.writeEntryRecord, Head: w.nativePackedHead, Tile: w.nativePackedTile, DestroyTown: w.nativeDestroyTown, Unlink: w.nativeRuntimeUnlink, Cleanup: w.cleanupNativeFollower,
		ClearLeader: func(ref NativeRecordReference) error {
			_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert})
			return err
		},
	}
}

func (w *World) nativeCommonPrepassCallbacks() CommonPrepassCallbacks {
	return CommonPrepassCallbacks{Read: w.readEntryRecord, Write: w.writeEntryRecord, Tile: w.nativePackedTile,
		WriteTile: func(tile NativePackedTile, code uint8) error {
			x, y := int(uint8(tile)), int(uint8(uint16(tile)>>8))
			if !inside(x, y) {
				return fmt.Errorf("native prepass terrain write outside world")
			}
			w.writeNativeTownTile(x, y, code)
			return nil
		},
		Scenario: func(owner uint8) (uint16, error) {
			if owner < 1 || owner > 2 {
				return 0, fmt.Errorf("native prepass scenario owner outside game")
			}
			return w.Rules[owner-1].Raw, nil
		},
		ClearFarms: w.clearNativeFarms, Cleanup: w.cleanupNativeFollower, ClearLeader: w.clearNativeLeader, Unlink: w.nativeRuntimeUnlink,
		Sound: func(raw uint16) error {
			w.HazardSerial++
			w.LastHazardCue = int(raw / 10)
			return w.nativeEntryCallbacks().Sound(raw)
		},
	}
}

func (w *World) clearNativeLeader(ref NativeRecordReference) error {
	_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert})
	return err
}

func (w *World) nativeHeroCallbacks() FollowerHeroCallbacks {
	return FollowerHeroCallbacks{Memory: w.nativeCleanupMemory(), Cleanup: w.cleanupNativeFollower,
		RaiseEnabled: func() bool { return w.NativeRaiseEnabled != 0 },
		Raise: func(x, y uint8) error {
			if !inside(int(x), int(y)) {
				return fmt.Errorf("native hero direct raising outside world")
			}
			before := w.Core.Alt
			if w.Core.DirectRaiseTerrain(int(x), int(y)) {
				w.clearChangedGround(before)
				w.refreshChangedTerrain(before)
			}
			return nil
		},
		Contact: func(source, target NativeRecordReference) error {
			_, err := PrepareFollowerContact(source, target, FollowerContactCallbacks{Memory: w.nativeCleanupMemory(), ClearFarms: w.clearNativeFarms, Sound: w.nativeEntryCallbacks().Sound})
			return err
		},
	}
}

func (w *World) nativeTerrainCallbacks() FollowerTerrainCallbacks {
	return FollowerTerrainCallbacks{Read: w.readEntryRecord, Write: w.writeEntryRecord, Tile: w.nativePackedTile, Scenario: w.nativeCommonPrepassCallbacks().Scenario,
		Attrition: func(owner uint8) (uint32, error) {
			deity, ok := NativeDeityAddress(owner)
			if !ok {
				return 0, fmt.Errorf("native water attrition owner outside deities")
			}
			return w.runtimeMemory().Read32(deity + 20)
		},
		SetWaterReference: func(owner uint8, ref NativeRecordReference) error {
			deity, ok := NativeDeityAddress(owner)
			if !ok {
				return fmt.Errorf("native water reference owner outside deities")
			}
			_, err := w.runtimeMemory().Write16(deity+0x36, uint16(ref))
			return err
		},
		Cleanup: w.cleanupNativeFollower, ClearLeader: w.clearNativeLeader,
		Move: func(ref NativeRecordReference, x, y uint16) error {
			_, err := w.Occupancy.Grid.Move(ref, x, y, w.runtimeMemory().RecordAccess())
			if err == nil {
				w.hydrateNativeRuntimeGraph()
			}
			return err
		},
	}
}

func (w *World) nativeMagnetCallbacks() FollowerMagnetCallbacks {
	return FollowerMagnetCallbacks{Hero: w.nativeHeroCallbacks(), Attrition: FollowerAttritionCallbacks{Read: w.readEntryRecord, Write: w.writeEntryRecord, Cleanup: w.cleanupNativeFollower},
		Merge: func(source, target NativeRecordReference) error {
			return w.FollowerEntry.merge(source, target, w.nativeEntryCallbacks())
		}}
}
