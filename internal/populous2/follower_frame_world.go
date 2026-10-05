package populous2

// nativeCommonPrepassFrameCallbacks borrows actual BSS and caller registers.
// Packed tile displacements retain the source's signed word and wrapping X
// byte, including supported aliases outside the normal 64x64 grid.
func (w *World) nativeCommonPrepassFrameCallbacks(frame *NativeFrameRegisterContext) CommonPrepassCallbacks {
	cb := w.nativeCommonPrepassCallbacks()
	cb.Frame = frame
	memory := w.nativeCleanupMemory()
	tileAddress := func(tile NativePackedTile) int {
		return 0xf45 + int(int16(uint16(tile)&0xff00|uint16(uint8(tile)*4)))
	}
	cb.Tile = func(tile NativePackedTile) (uint8, error) { return memory.Read8(tileAddress(tile)) }
	cb.WriteTile = func(tile NativePackedTile, value uint8) error { return memory.Write8(tileAddress(tile), value) }
	cb.Scenario = func(owner uint8) (uint16, error) {
		address := 0xeb2c
		if owner != 1 {
			address = 0xeb2e
		}
		return memory.Read16(address)
	}
	cb.CleanupFrame = func(ref NativeRecordReference, _ uint16, context *NativeFrameRegisterContext) error {
		_, err := CleanupFollowerWithFrame(ref, context, FollowerCleanupCallbacks{Memory: memory, Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
		return err
	}
	cb.ClearLeaderFrame = func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
		_, err := ClearFollowerLeaderWithFrame(ref, context, FollowerLeaderCallbacks{Memory: memory, Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert})
		return err
	}
	return cb
}

func (w *World) nativeAftermathFrameCallbacks(frame *NativeFrameRegisterContext) NativeFollowerAftermathFrameCallbacks {
	memory := w.nativeCleanupMemory()
	return NativeFollowerAftermathFrameCallbacks{Memory: memory, Frame: frame,
		Cleanup: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
			_, err := CleanupFollowerWithFrame(ref, context, FollowerCleanupCallbacks{Memory: memory, Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
			return err
		},
		ClearLeader: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
			_, err := ClearFollowerLeaderWithFrame(ref, context, FollowerLeaderCallbacks{Memory: memory, Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert})
			return err
		},
		Unlink: w.nativeRuntimeUnlink, DestroyTown: w.nativeDestroyTown,
	}
}

func (w *World) nativeWinnerFrameCallbacks() NativeFollowerWinFrameCallbacks {
	memory := w.nativeCleanupMemory()
	return NativeFollowerWinFrameCallbacks{Memory: memory,
		Cleanup: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
			_, err := CleanupFollowerWithFrame(ref, context, FollowerCleanupCallbacks{Memory: memory, Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
			return err
		},
		ClearFarms: w.clearNativeFarms, ReformTown: w.nativeReformTown, DestroyTown: w.nativeDestroyTown,
		PoolBlocked: func() bool { return w.nativeBirthBlockWord() != 0 }, Insert: w.nativeRuntimeInsert,
	}
}

func (w *World) nativeEntryFrameCallbacks(frame *NativeFrameRegisterContext, state *NativeFollowerEntryFrameState) NativeFollowerEntryFrameCallbacks {
	memory := w.nativeCleanupMemory()
	return NativeFollowerEntryFrameCallbacks{Memory: memory, Frame: frame, State: state,
		Merge: func(source, target NativeRecordReference, context *NativeFrameRegisterContext) error {
			cb := w.nativeEntryCallbacks()
			cb.Selected = func() NativeRecordReference {
				pointer, err := memory.Read32(0xf36)
				if err != nil || pointer == 0 {
					return 0
				}
				return NativeRecordReference(uint16(pointer - uint32(frame.AddressBase) - 0x76c0))
			}
			cb.Select = func(ref NativeRecordReference) error {
				w.NativeSelected = ref
				return memory.Write32(0xf36, frame.AddressBase+uint32(cleanupRecordAddress(ref)))
			}
			return w.FollowerEntry.MergeWithFrame(source, target, context, cb)
		},
		Battle: func(source, target NativeRecordReference, context *NativeFrameRegisterContext) error {
			_, err := PrepareFollowerContactWithFrame(source, target, context, FollowerContactCallbacks{Memory: memory, ClearFarms: w.clearNativeFarms, Sound: w.nativeEntryCallbacks().Sound})
			return err
		},
		ReformTown: func(ref NativeRecordReference) error {
			clock, err := memory.Read16(0xf42)
			if err != nil {
				return err
			}
			cb := w.nativeTownCombatCallbacks()
			cb.EvaluateTown = func(ref NativeRecordReference) (int, error) {
				value, err := w.TownEvaluator.Evaluate(ref, clock, w.nativeTownCallbacks())
				return int(value), err
			}
			_, err = w.TownCombat.Reform(ref, ref, clock, cb)
			return err
		},
	}
}
