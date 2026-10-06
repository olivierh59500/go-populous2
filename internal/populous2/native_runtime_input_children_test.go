package populous2

import "testing"

func TestNativeRuntimeInputCreatesDelayedTerrainCommandWithActualChildren(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	for _, p := range []nativeHeroPatch{{0xf0c, 2, 23}, {0x138, 2, 192}, {0x13a, 2, 184}, {0x140, 2, 1}, {0xeb18, 2, 2}, {0xeb42, 2, 1}, {0xeb6a, 4, 0x20eb56}, {0xe8ef, 1, 1}, {0xe8a4, 4, 1000000}} {
		renderFramePatch(h.Memory.BSS, p)
	}
	if err := h.Memory.BSS.Write8(0xe8ef, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write16(0x5f44, 24); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write16(0x5f46, 25); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4096; i++ {
		if err := h.Memory.BSS.Write8(0xf44+i*4, 3); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write8(0xf45+i*4, 15); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Memory.Code.Write32(0xe458, 0x00c00048); err != nil {
		t.Fatal(err)
	}
	children, err := h.NewInputChildren(NativeGameplayHUDHostCallbacks{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	state := NativeGameplayInputState{}
	callback, err := h.GameplayInput(&state, NativeStartupResetFrameCallbacks{Call: children.Call})
	if err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	phase := uint32(0)
	complete, err := callback(h.Memory.BSS, &frame, &phase)
	if err != nil || !complete || phase != 2 {
		t.Fatal("real input terrain operation did not complete", complete, phase, err)
	}
	command, err := h.Memory.BSS.Read8(0xeb57)
	if err != nil || command != 2 {
		t.Fatal("terrain click did not produce native delayed command", command, err)
	}
	if value, err := h.Memory.BSS.Read16(0x140); err != nil || value != 0 {
		t.Fatal("native click latch did not clear", value, err)
	}
}
