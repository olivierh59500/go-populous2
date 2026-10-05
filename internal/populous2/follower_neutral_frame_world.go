package populous2

func (w *World) nativeNeutralFrameCallbacks(frame *NativeFrameRegisterContext, bindings NativeCommandWorldBindings) NativeFollowerNeutralFrameCallbacks {
	return NativeFollowerNeutralFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame,
		Random:  func() uint16 { return uint16(w.random()) },
		Unlink:  w.nativeRuntimeUnlink,
		Cleanup: w.nativeHeroFrameCallbacks(frame).Cleanup,
		Move:    w.nativeTerrainFrameCallbacks(frame).Move,
		Call: func(routine uint32, context *NativeFrameRegisterContext) error {
			command := context.CommandContext()
			var err error
			if routine == 0xd7f0 {
				_, err = w.commandDirectTerrain(NativeCommandCall{Routine: int(routine), Context: &command}, false)
			} else {
				_, err = w.nativeNormalCommandCallbacks(bindings).Call(NativeCommandCall{Routine: int(routine), Context: &command})
			}
			context.SetCommandContext(command)
			return err
		},
	}
}
