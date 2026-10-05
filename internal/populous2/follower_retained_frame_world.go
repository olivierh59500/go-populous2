package populous2

func (w *World) nativeRetainedFrameCallbacks(frame *NativeFrameRegisterContext, heroes *NativeFollowerHeroFrameRules) NativeFollowerRetainedFrameCallbacks {
	return NativeFollowerRetainedFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame,
		Attrition: w.nativeFollowerAttritionFrame,
		Plan: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
			return heroes.Plan(ref, w.nativeHeroFrameCallbacks(context))
		},
		ClearLeader: w.nativeAftermathFrameCallbacks(frame).ClearLeader,
		Unlink:      w.nativeRuntimeUnlink,
	}
}
