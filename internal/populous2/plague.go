package populous2

// castPlague runs original $1730e against the linked mixed-actor cell list.
// CommonPrepass owns the phase and mortality; actual merge/birth controllers
// transfer infection. Co-location alone is not a native propagation rule.
func (w *World) castPlague(player, pos int) bool {
	applied := false
	err := w.runNativeFollowerCall(func() error {
		step, err := w.PlagueRules.Create(uint16(player+1), uint8(pos%64), uint8(pos/64), PlagueCallbacks{Memory: w.nativeCleanupMemory(), Cleanup: w.cleanupNativeFollower})
		applied = step.Admitted
		return err
	})
	if err != nil {
		panic(err)
	}
	return applied
}

func (w *World) PlagueFollowerFrame(index int) (AnimationFrame, bool) {
	if index < 0 || index >= len(w.Core.Peeps) {
		return AnimationFrame{}, false
	}
	a, err := w.RecordImage.ReadFollowerEntry(nativeActorReference(NativeFollowerPool, index))
	if err != nil || a.Owner == 0 || a.Owner == 3 || a.Motion.Flags&16 == 0 {
		return AnimationFrame{}, false
	}
	frame, ok := w.PlagueRules.Frames[int(a.Extra48)]
	return frame, ok
}

func (w *World) castNativeArmageddon() {
	err := w.runNativeFollowerCall(func() error {
		_, err := w.ArmageddonRules.Cast(ArmageddonCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Cleanup: w.cleanupNativeFollower,
			Convert: func(ref NativeRecordReference, hero uint16) error {
				return w.HeroArt.Convert(ref, heroIDs[hero/2], HeroCreationCallbacks{Memory: w.nativeCleanupMemory(), ClearLeader: w.clearNativeLeader, ClearFarms: w.clearNativeFarms, Sound: w.nativeEntryCallbacks().Sound})
			}})
		return err
	})
	if err != nil {
		panic(err)
	}
}
