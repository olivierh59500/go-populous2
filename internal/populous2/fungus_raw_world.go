package populous2

import (
	"encoding/binary"
	"fmt"
)

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
	return NativeFungusCallbacks{Memory: m, WriteCode8: func(at int, value uint8) error {
		if at < 0 || at >= len(w.NativeAI.Code) {
			return fmt.Errorf("native Fungus CODE byte%x unavailable", at)
		}
		w.NativeAI.Code[at] = value
		return nil
	}, WriteCode16: func(at int, value uint16) error {
		if at < 0 || at&1 != 0 || at+2 > len(w.NativeAI.Code) {
			return fmt.Errorf("native Fungus CODE word%x unavailable", at)
		}
		binary.BigEndian.PutUint16(w.NativeAI.Code[at:], value)
		return nil
	}}
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
