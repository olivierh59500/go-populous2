package populous2

// initializeNativeSceneData composes the original controls and template
// startup without replaying its destructive whole-BSS clear after allocation.
// Strategy execution remains a separate ordered stage of the game loop.
func (w *World) initializeNativeSceneData() error {
	m := w.nativeCleanupMemory()
	for _, field := range []struct {
		address int
		value   uint16
	}{{0xeb2c, w.Rules[0].Raw}, {0xeb2e, w.Rules[1].Raw}, {0xeb44, w.NativeGameMode}, {0xeb46, uint16(w.Level.Number)}} {
		if err := m.Write16(field.address, field.value); err != nil {
			return err
		}
	}
	if err := m.Write32(0xeb24, w.Level.RandomSeed); err != nil {
		return err
	}
	if err := m.Write32(0xeb28, w.Core.RandomState()); err != nil {
		return err
	}
	if err := w.initializeNativeAIControls(); err != nil {
		return err
	}
	experience, err := w.loadNativeAITemplates(&w.NativeAI, w.Level)
	if err != nil {
		return err
	}
	w.Experience = experience
	selected, err := m.Read16(0xeb42)
	if err != nil {
		return err
	}
	w.NativeProfileSide = uint8(selected)
	god, _ := NativeDeityAddress(w.NativeProfileSide)
	bolts, err := m.Read16(god + 0x58)
	if err != nil {
		return err
	}
	w.Deity.Bolts = bolts
	return nil
}
