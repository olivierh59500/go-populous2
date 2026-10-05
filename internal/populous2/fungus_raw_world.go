package populous2

func (w *World) nativeFungusCallbacks() NativeFungusCallbacks {
	m := w.nativeCleanupMemory()
	write := m.Write8
	m.Write8 = func(address int, value uint8) error {
		if err := write(address, value); err != nil {
			return err
		}
		if address >= 0xf45 && address < 0x4f44 && (address-0xf44)%4 == 1 && value >= 145 && value <= 151 {
			w.Marks[(address-0xf44)/4].Spell = Fungus
		}
		return nil
	}
	return NativeFungusCallbacks{Memory: m}
}

// castFungus uses the raw creator, including planting before pool admission
// and reuse of the deity's retained pending reference. It never consumes RNG.
func (w *World) castFungus(player, x, y int) FungusPlacement {
	result := FungusPlacement{Slot: -1}
	err := w.runNativeFollowerCall(func() error {
		step, err := w.NativeFungus.Create(uint16(player+1), uint8(x), uint8(y), w.nativeFungusCallbacks())
		result.Planted, result.Reused = step.Planted, step.Reused
		if location, ok := LocateNativeRecord(step.Reference); ok && location.Pool == NativeEffectPool {
			result.Slot = location.Index
			if step.Created {
				w.NativeEnvironment[location.Index] = NativeEnvironmentNone
			}
		}
		return err
	})
	if err != nil {
		panic(err)
	}
	return result
}

func (w *World) tickRawFungus(index int) error {
	return w.runNativeFollowerCall(func() error {
		_, err := w.NativeFungus.Tick(nativeActorReference(NativeEffectPool, index), w.nativeFungusCallbacks())
		return err
	})
}
