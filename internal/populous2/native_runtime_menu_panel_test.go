package populous2

import (
	"bytes"
	"testing"
)

func TestNativeRuntimeMenuRestoresActualPanelAndDrawingTargets(t *testing.T) {
	var hosts [2]*NativeRuntimeHost
	var devices [2]*NativeAudioDevice
	var contexts [2]NativeFrameRegisterContext
	for side := range hosts {
		hosts[side] = nativeRuntimeHostTest(t)
		devices[side] = runtimeFilesPrelude(t, hosts[side])
		contexts[side] = runtimeFilesCampaign(t, hosts[side], devices[side])
	}
	h := hosts[0]
	frame, err := h.NewFrame(NativeRuntimeFrameBindings{Audio: NativeRuntimeAudioOperations{Command: devices[0].Command, MusicCommand: devices[0].MusicCommand, DirectCue: devices[0].DirectCue}, Menu: NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}})
	if err != nil {
		t.Fatal(err)
	}
	// The real menu return invokes1DA0 after181C0. This child paints its
	// original icons to22 and HUD to1E using the factory's genuine sprite sink.
	// Start at the independently verified return from181C0. The actual
	// MenuFrame closure owns the factory's default Sprite binding.
	frame.MenuState.Menu = NativeInGameFrameState{Started: true, PC: 0x45c2, Registers: contexts[0].D, Saved: contexts[0].D}
	phase := uint32(1)
	done, err := frame.Callbacks.Menu(h.Memory.BSS, &contexts[0], &h.Session.Image, &phase)
	if err != nil || !done || phase != 2 {
		t.Fatal("actual menu return failed", done, phase, err)
	}
	ref := hosts[1]
	rules, err := DecodeNativeProfilePanelFrameRules(ref.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(ref.Bundle, int(ref.World.Level.Terrain))
	if err != nil {
		t.Fatal(err)
	}
	_, err = rules.RestorePanel(NativeProfilePanelFrameCallbacks{Memory: ref.Memory.BSS, Frame: &contexts[1], Image: &ref.Session.Image, Sprite: bank.Paint, Bitmap: ref.Bitmap, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}

	for _, at := range []int{0x22, 0x1e} {
		a, err := h.Memory.BSS.Read32(at)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ref.Memory.BSS.Read32(at)
		if err != nil {
			t.Fatal(err)
		}
		x, err := h.Bitmap(a)
		if err != nil {
			t.Fatal(err)
		}
		y, err := ref.Bitmap(b)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(x, y) {
			t.Fatal("menu panel changed the native target", at)
		}
	}
}
