package populous2

func (w *World) beginNativeFollowerPass() {
	w.setNativeBirthBlockWord(0)
	w.syncNativeRuntimeBridge()
	m := w.runtimeMemory()
	for owner := uint8(1); owner <= 2; owner++ {
		god, _ := NativeDeityAddress(owner)
		for _, field := range [][2]int{{0, 0x40}, {4, 0x3c}} {
			value, _ := m.Read32(god + field[0])
			maximum, _ := m.Read32(god + field[1])
			if int32(value) > int32(maximum) {
				if _, err := m.Write32(god+field[1], value); err != nil {
					panic(err)
				}
			}
		}
		if _, err := m.Write32(god+4, 0); err != nil {
			panic(err)
		}
		for _, offset := range []int{0x1c, 0x24, 0x36, 0x2e, 0x32} {
			if _, err := m.Write16(god+offset, 0); err != nil {
				panic(err)
			}
		}
		if _, err := m.Write16(god+0x20, 0xffff); err != nil {
			panic(err)
		}
	}
}

func (w *World) nativePrimitiveCallbacks() NativePrimitiveCreatorCallbacks {
	return NativePrimitiveCreatorCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Link: w.nativeRuntimeInsert}
}

func (w *World) nativeNeutralCallbacks() NativeNeutralActorCallbacks {
	return NativeNeutralActorCallbacks{Memory: w.nativeCleanupMemory(), Move: func(ref NativeRecordReference, x, y uint16) error {
		_, err := w.Occupancy.Grid.Move(ref, x, y, w.runtimeMemory().RecordAccess())
		if err == nil {
			w.hydrateNativeRuntimeGraph()
		}
		return err
	}, Unlink: w.nativeRuntimeUnlink, Head: w.nativePackedHead, Tile: w.nativePackedTile, Random: w.random, Cleanup: w.cleanupNativeFollower,
		WriteTile: func(tile NativePackedTile, code uint8) error {
			return w.nativeCommonPrepassCallbacks().WriteTile(tile, code)
		},
		Lower: func(x, y uint8) error {
			before := w.Core.Alt
			if w.Core.DirectLowerTerrain(int(x), int(y)) {
				w.clearChangedGround(before)
				w.refreshChangedTerrain(before)
			}
			return nil
		},
		CreateWhirlwind: func(x, y uint8, owner uint16) error {
			_, err := w.PrimitiveCreators.CreateWhirlwind(owner, x, y, w.nativePrimitiveCallbacks())
			return err
		},
		PlantTree: func(x, y uint8, _ uint16) error {
			_, err := w.PrimitiveCreators.PlantTree(x, y, w.nativePrimitiveCallbacks())
			return err
		},
		CreateFireColumn: func(x, y uint8, owner uint16) error {
			_, err := w.PrimitiveCreators.CreateFireColumn(owner, x, y, w.nativePrimitiveCallbacks())
			return err
		},
	}
}

func (w *World) nativeTownEconomyTick(ref NativeRecordReference) (NativeTownEconomyStep, error) {
	blocked := w.nativeBirthBlockWord()
	state := NativeTownEconomyState{PoolBlocked: blocked, Selected: w.NativeSelected, Clock: uint32(w.Core.GameTurn), Deadline: w.NativeCreatureDeadline}
	step, err := w.TownEconomy.Tick(ref, &state, NativeTownEconomyCallbacks{Memory: w.nativeCleanupMemory(), EvaluateTown: w.evaluateNativeTown, ClearFarms: w.clearNativeFarms, Insert: w.nativeRuntimeInsert, Random: w.random,
		LandAI: func(request NativeTownLandRequest) error {
			_, err := CreateNativeNeutral(request.Registers, NativeNeutralCallbacks{Memory: w.nativeCleanupMemory(), Insert: w.nativeRuntimeInsert})
			return err
		}})
	w.setNativeBirthBlockWord(state.PoolBlocked)
	w.NativeSelected, w.NativeCreatureDeadline = state.Selected, state.Deadline
	return step, err
}
