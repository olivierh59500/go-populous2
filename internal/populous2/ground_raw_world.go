package populous2

func (w *World) nativeGroundCallbacks(id SpellID, player int) NativeGroundCallbacks {
	m := w.nativeCleanupMemory()
	write := m.Write8
	m.Write8 = func(address int, value uint8) error {
		if err := write(address, value); err != nil {
			return err
		}
		if address >= 0xf45 && address < 0x4f44 && (address-0xf44)%4 == 1 && (value == 143 || value == 168) {
			pos := (address - 0xf44) / 4
			w.Marks[pos].Spell, w.Marks[pos].Player = id, player
		}
		return nil
	}
	return NativeGroundCallbacks{Memory: m, Random: func() uint16 { return uint16(w.random()) }, Prepass: w.nativeCommonPrepassCallbacks(), Terrain: w.nativeTerrainCallbacks(), Aftermath: w.nativeAftermathCallbacks()}
}

// castGroundEffect runs the raw font/swamp creator. Retained prepass/terrain
// handlers own entry, immunity, conversion and mortality in the follower pass.
func (w *World) castGroundEffect(player int, id SpellID, x, y int) bool {
	err := w.runNativeFollowerCall(func() error {
		_, err := w.NativeGround.Create(id, uint16(player+1), uint8(x), uint8(y), w.nativeGroundCallbacks(id, player))
		return err
	})
	if err != nil {
		panic(err)
	}
	return true
}
