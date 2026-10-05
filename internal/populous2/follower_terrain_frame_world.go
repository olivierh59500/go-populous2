package populous2

func (w *World) nativeTerrainFrameCallbacks(frame *NativeFrameRegisterContext) NativeFollowerTerrainFrameCallbacks {
	aftermath := w.nativeAftermathFrameCallbacks(frame)
	return NativeFollowerTerrainFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame,
		Cleanup: aftermath.Cleanup, ClearLeader: aftermath.ClearLeader, Unlink: w.nativeRuntimeUnlink,
		Move: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
			command := context.CommandContext()
			err := w.commandMove(cleanupRecordAddress(ref), &command)
			context.SetCommandContext(command)
			return err
		},
	}
}
