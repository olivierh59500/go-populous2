package populous2

func (w *World) nativeHeroFrameCallbacks(frame *NativeFrameRegisterContext) NativeFollowerHeroFrameCallbacks {
	memory := w.nativeCleanupMemory()
	return NativeFollowerHeroFrameCallbacks{Memory: memory, Frame: frame,
		Raise: func(context *NativeFrameRegisterContext) error {
			command := context.CommandContext()
			_, err := w.commandDirectTerrain(NativeCommandCall{Routine: 0xd81e, Context: &command}, true)
			context.SetCommandContext(command)
			return err
		},
		Contact: func(source, target NativeRecordReference, context *NativeFrameRegisterContext) (FollowerContactStep, error) {
			return PrepareFollowerContactWithFrame(source, target, context, FollowerContactCallbacks{Memory: memory, ClearFarms: w.clearNativeFarms, Sound: w.nativeEntryCallbacks().Sound})
		},
		Cleanup: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
			_, err := CleanupFollowerWithFrame(ref, context, FollowerCleanupCallbacks{Memory: memory, Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
			return err
		},
	}
}

func (w *World) nativeFollowerAttritionFrame(ref NativeRecordReference, god int, frame *NativeFrameRegisterContext) (bool, error) {
	return ApplyFollowerAttritionWithFrame(ref, god, FollowerAttritionFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame, CleanupFrame: w.nativeHeroFrameCallbacks(frame).Cleanup})
}

func (w *World) nativeMagnetFrameCallbacks(frame *NativeFrameRegisterContext, heroes *NativeFollowerHeroFrameRules) NativeFollowerMagnetFrameCallbacks {
	return NativeFollowerMagnetFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame,
		Attrition: w.nativeFollowerAttritionFrame,
		Plan: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
			return heroes.Plan(ref, w.nativeHeroFrameCallbacks(context))
		},
		Merge: func(source, target NativeRecordReference, context *NativeFrameRegisterContext) error {
			return w.nativeEntryFrameCallbacks(context, &NativeFollowerEntryFrameState{}).Merge(source, target, context)
		},
	}
}
