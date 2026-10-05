package populous2

func (w *World) nativeFireColumnCallbacks() NativeFireColumnCallbacks {
	return NativeFireColumnCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Move: w.tsunamiCallbacks().Move, Unlink: w.nativeRuntimeUnlink, DestroyTown: w.nativeDestroyTown}
}

func (w *World) tickRawFireColumn(index int) error {
	return w.runNativeFollowerCall(func() error {
		_, err := w.NativeFireColumn.Tick(nativeActorReference(NativeEffectPool, index), w.nativeFireColumnCallbacks())
		return err
	})
}
