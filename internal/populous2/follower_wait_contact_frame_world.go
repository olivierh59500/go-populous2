package populous2

func (w *World) nativeWaitContactFrameCallbacks(frame *NativeFrameRegisterContext) NativeFollowerWaitContactFrameCallbacks {
	return NativeFollowerWaitContactFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame,
		Contact: w.nativeHeroFrameCallbacks(frame).Contact,
		Merge:   w.nativeEntryFrameCallbacks(frame, &NativeFollowerEntryFrameState{}).Merge,
	}
}
