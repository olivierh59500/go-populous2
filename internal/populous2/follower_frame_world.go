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
