package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"go-populous2/internal/populous2"
)

var nativeKeys = func() []struct {
	key ebiten.Key
	raw uint8
} {
	bindings := populous2.NativeDesktopKeyBindings()
	keys := make([]struct {
		key ebiten.Key
		raw uint8
	}, len(bindings))
	for i, binding := range bindings {
		if err := keys[i].key.UnmarshalText([]byte(binding.Name)); err != nil {
			panic(err)
		}
		keys[i].raw = binding.Raw
	}
	return keys
}()

func (g *NativeGame) pollKeys() error {
	previous := make([]populous2.NativeHostKeySample, len(nativeKeys))
	current := make([]populous2.NativeHostKeySample, len(nativeKeys))
	for i, entry := range nativeKeys {
		previous[i] = populous2.NativeHostKeySample{Raw: entry.raw, Down: g.keys[entry.key]}
		current[i] = populous2.NativeHostKeySample{Raw: entry.raw, Down: ebiten.IsKeyPressed(entry.key)}
	}
	wires, err := populous2.NativeHostKeyboardTransitions(previous, current)
	if err != nil {
		return err
	}
	for _, wire := range wires {
		if err := g.Host.Session.Presentation.Input.KeyboardInterrupt(wire); err != nil {
			return err
		}
	}
	for i, entry := range nativeKeys {
		g.keys[entry.key] = current[i].Down
	}
	return nil
}
