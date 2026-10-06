package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"go-populous2/internal/populous2"
)

var nativeKeys = []struct {
	key ebiten.Key
	raw uint8
}{
	{ebiten.KeyEscape, 0x45}, {ebiten.KeySpace, 0x40}, {ebiten.KeyEnter, 0x44}, {ebiten.KeyBackspace, 0x41}, {ebiten.KeyTab, 0x42},
	{ebiten.KeyArrowUp, 0x4c}, {ebiten.KeyArrowDown, 0x4d}, {ebiten.KeyArrowLeft, 0x4f}, {ebiten.KeyArrowRight, 0x4e},
	{ebiten.KeyShiftLeft, 0x60}, {ebiten.KeyShiftRight, 0x61}, {ebiten.KeyControlLeft, 0x63}, {ebiten.KeyAltLeft, 0x64},
	{ebiten.KeyA, 0x20}, {ebiten.KeyB, 0x35}, {ebiten.KeyC, 0x33}, {ebiten.KeyD, 0x22}, {ebiten.KeyE, 0x12}, {ebiten.KeyF, 0x23}, {ebiten.KeyG, 0x24}, {ebiten.KeyH, 0x25}, {ebiten.KeyI, 0x17}, {ebiten.KeyJ, 0x26}, {ebiten.KeyK, 0x27}, {ebiten.KeyL, 0x28}, {ebiten.KeyM, 0x37}, {ebiten.KeyN, 0x36}, {ebiten.KeyO, 0x18}, {ebiten.KeyP, 0x19}, {ebiten.KeyQ, 0x10}, {ebiten.KeyR, 0x13}, {ebiten.KeyS, 0x21}, {ebiten.KeyT, 0x14}, {ebiten.KeyU, 0x16}, {ebiten.KeyV, 0x34}, {ebiten.KeyW, 0x11}, {ebiten.KeyX, 0x32}, {ebiten.KeyY, 0x15}, {ebiten.KeyZ, 0x31},
	{ebiten.KeyDigit1, 0x01}, {ebiten.KeyDigit2, 0x02}, {ebiten.KeyDigit3, 0x03}, {ebiten.KeyDigit4, 0x04}, {ebiten.KeyDigit5, 0x05}, {ebiten.KeyDigit6, 0x06}, {ebiten.KeyDigit7, 0x07}, {ebiten.KeyDigit8, 0x08}, {ebiten.KeyDigit9, 0x09}, {ebiten.KeyDigit0, 0x0a},
	{ebiten.KeyF1, 0x50}, {ebiten.KeyF2, 0x51}, {ebiten.KeyF3, 0x52}, {ebiten.KeyF4, 0x53}, {ebiten.KeyF5, 0x54}, {ebiten.KeyF6, 0x55}, {ebiten.KeyF7, 0x56}, {ebiten.KeyF8, 0x57}, {ebiten.KeyF9, 0x58}, {ebiten.KeyF10, 0x59},
	{ebiten.KeyNumpad1, 0x1d}, {ebiten.KeyNumpad2, 0x1e}, {ebiten.KeyNumpad3, 0x1f}, {ebiten.KeyNumpad4, 0x2d}, {ebiten.KeyNumpad5, 0x2e}, {ebiten.KeyNumpad6, 0x2f}, {ebiten.KeyNumpad7, 0x3d}, {ebiten.KeyNumpad8, 0x3e}, {ebiten.KeyNumpad9, 0x3f},
}

func (g *NativeGame) pollKeys() error {
	for _, entry := range nativeKeys {
		down := ebiten.IsKeyPressed(entry.key)
		if down == g.keys[entry.key] {
			continue
		}
		wire, err := populous2.NativeKeyWire(entry.raw, down)
		if err != nil {
			return err
		}
		if err := g.Host.Session.Presentation.Input.KeyboardInterrupt(wire); err != nil {
			return err
		}
		g.keys[entry.key] = down
	}
	return nil
}
