package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

func nativeSessionRenderTestBindings(t *testing.T) NativeSessionRenderBindings {
	t.Helper()
	bundle := testBundle(t)
	rules, err := DecodeNativeActorRenderRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	sprites, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := DecodeNativeTileBitmapBank(bundle.Raw["block0.pak"])
	if err != nil {
		t.Fatal(err)
	}
	return NativeSessionRenderBindings{Rules: &rules, Sprites: sprites, Tiles: tiles}
}

func TestNativeSessionDebugTargetUsesCODEInsteadOfTerrain(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	bindings := nativeSessionRenderTestBindings(t)
	state := NativeMainRenderState{}
	background := make([]byte, 32000)
	if err := session.Begin(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	memory := session.Presentation.Memory(w.nativeCleanupMemory())
	_ = memory.Write32(0x22, 0xa00000)
	_ = memory.Write16(0xf0c, 8)
	_ = memory.Write16(0xf0e, 0)
	_ = memory.Write16(0x3b8, 1)
	_ = memory.Write16(0xeb44, 8)
	_ = memory.Write16(0x5f44, 20)
	_ = memory.Write16(0x5f46, 20)
	_ = memory.Write32(0x2e3a, 0x5a7c9dbf)
	_ = memory.Write32(0xf40, 0x12345678)
	target, _ := memory.Read32(0x1e)
	called := false
	bindings.DebugOverlay = func(frame *NativeFrameRegisterContext) error {
		called = true
		terrain, err := memory.Read32(0x2e3a)
		if err != nil {
			return err
		}
		if binary.BigEndian.Uint32(w.NativeAI.Code[0x2e3a:]) != target || terrain != 0x5a7c9dbf || frame.D[0] != 0x12345678 {
			t.Fatal("debug target was written to terrain or source clock registers differ")
		}
		return nil
	}
	session.Presentation.Input.setWord(0xa, 1)
	complete, err := session.Advance(NativeFrameSessionCallbacks{
		Render: session.RenderMain(bindings, &state),
		Bitmap: func(uint32) ([]byte, error) { return background, nil },
	})
	if err != nil || !complete || !called {
		t.Fatal("debug source child was not reached", complete, called, err)
	}
}

func TestNativeSessionConcreteRenderingUsesRealBuffersAcrossSwaps(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	bindings := nativeSessionRenderTestBindings(t)
	state := NativeMainRenderState{}
	background := make([]byte, 32000)
	for i := range background {
		background[i] = byte(i*53 + 17)
	}
	callbacks := NativeFrameSessionCallbacks{Bitmap: func(address uint32) ([]byte, error) {
		if address != 0xa00000 {
			return nil, fmt.Errorf("unexpected external bitmap%x", address)
		}
		return background, nil
	}}
	callbacks.Render = session.RenderMain(bindings, &state)
	for _, view := range []uint16{8, 16, 8} {
		if err := session.Begin(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
			t.Fatal(err)
		}
		memory := session.Presentation.Memory(w.nativeCleanupMemory())
		_ = memory.Write32(0x22, 0xa00000)
		_ = memory.Write16(0x3b8, 1) // This frame has no protection challenge.
		_ = memory.Write16(0xf0c, view)
		_ = memory.Write16(0xf0e, 0)
		_ = memory.Write16(0xeb44, 2)
		_ = memory.Write16(0x5f44, 20)
		_ = memory.Write16(0x5f46, 20)
		drawingAddress, _ := memory.Read32(0x1e)
		at, err := session.Presentation.chipAt(drawingAddress, 32000)
		if err != nil {
			t.Fatal(err)
		}
		bitmap := session.Presentation.Chip[at : at+32000]
		for i := range bitmap {
			bitmap[i] = 0x5a
		}
		before := append([]byte(nil), bitmap...)
		backdrop := append([]byte(nil), background...)
		session.Presentation.Input.setWord(0xa, 1)
		complete, err := session.Advance(callbacks)
		if err != nil || !complete {
			t.Fatal("concrete native session frame failed", view, complete, err)
		}
		if state.Step != 11 || session.RenderPhase != 2 || state.View != view || w.nativeCallDepth != 0 {
			t.Fatal("concrete renderer lifecycle differs", state.Step, session.RenderPhase, state.View, w.nativeCallDepth)
		}
		if bytes.Equal(bitmap, before) {
			t.Fatal("native drawing target was not updated", view)
		}
		if !bytes.Equal(background, backdrop) {
			t.Fatal("renderer mutated the persistent background")
		}
		if session.Presentation.Input.long(0x1a) != drawingAddress {
			t.Fatal("swap did not display the actual drawn buffer")
		}
	}
}

func TestNativeSessionConcreteRenderingRejectsDetachedRAM(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	bindings := nativeSessionRenderTestBindings(t)
	bindings.Window = func(uint32) (NativeBitmapWindow, error) {
		return NativeBitmapWindow{Bytes: make([]byte, 33024), BitmapOffset: 256}, nil
	}
	state := NativeMainRenderState{}
	if err := session.Begin(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	defer session.finish(nil)
	memory := session.Presentation.Memory(w.nativeCleanupMemory())
	bitmap, err := session.Presentation.BackBuffer()
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), bitmap...)
	phase := uint32(0)
	complete, err := session.RenderMain(bindings, &state)(memory, &session.Frame, &session.Image, bitmap, &phase)
	if err == nil || complete || phase != 0 || !bytes.Equal(bitmap, before) {
		t.Fatal("detached adjacent RAM was accepted or drawing began")
	}
}

func TestNativeSessionConcreteRenderingMissingModalRetainsPrefix(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	bindings := nativeSessionRenderTestBindings(t)
	state := NativeMainRenderState{}
	background := bytes.Repeat([]byte{0x47}, 32000)
	if err := session.Begin(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	memory := session.Presentation.Memory(w.nativeCleanupMemory())
	_ = memory.Write32(0x22, 0xa00000)
	_ = memory.Write16(0xf0c, 8)
	_ = memory.Write16(0xf0e, 1)
	bitmap, err := session.Presentation.BackBuffer()
	if err != nil {
		t.Fatal(err)
	}
	session.Presentation.Input.setWord(0xa, 1)
	complete, err := session.Advance(NativeFrameSessionCallbacks{
		Render: session.RenderMain(bindings, &state),
		Bitmap: func(uint32) ([]byte, error) { return background, nil },
	})
	if err == nil || complete || state.Step != 4 || session.RenderPhase != 1 || w.nativeCallDepth != 0 {
		t.Fatal("missing editor body was accepted or renderer prefix was lost", complete, err, state.Step)
	}
	if bitmap[31999] != background[31999] {
		t.Fatal("completed background copy was rolled back after the missing modal")
	}
	if err := session.Begin(w, NativeFrameRegisterContext{}); err == nil {
		t.Fatal("failed renderer was silently restarted")
	}
}
