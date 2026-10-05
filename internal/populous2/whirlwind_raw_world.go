package populous2

func (w *World) nativeWhirlwindCallbacks() NativeWhirlwindCallbacks {
	return NativeWhirlwindCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Move: w.tsunamiCallbacks().Move, Unlink: w.nativeRuntimeUnlink, ClearFarms: w.clearNativeFarms,
		Cleanup: func(ref NativeRecordReference, registers FollowerCleanupRegisters) (FollowerCleanupStep, error) {
			return CleanupFollower(ref, registers, FollowerCleanupCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
		}}
}

func (w *World) tickRawWhirlwind(index int) error {
	return w.runNativeFollowerCall(func() error {
		_, err := w.NativeWhirlwind.Tick(nativeActorReference(NativeEffectPool, index), w.nativeWhirlwindCallbacks())
		return err
	})
}
