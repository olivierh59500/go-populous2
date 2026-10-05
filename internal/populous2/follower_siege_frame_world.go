package populous2

func (w *World) nativeSiegeFrameCallbacks(frame *NativeFrameRegisterContext) NativeFollowerSiegeFrameCallbacks {
	return NativeFollowerSiegeFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame,
		Cleanup:    w.nativeHeroFrameCallbacks(frame).Cleanup,
		ReformTown: w.nativeEntryFrameCallbacks(frame, &NativeFollowerEntryFrameState{}).ReformTown,
	}
}
