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

func TestNativeSessionRetainsActualEditorAcrossKeyboardWaits(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	bindings := nativeSessionRenderTestBindings(t)
	editorRules, err := DecodeNativeEditorFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	code := fileFrameRelocatedCode(t)
	codeMemory := commandFrameBacking(code)
	bindings.Code = codeMemory
	state, editor := NativeMainRenderState{}, NativeEditorFrameState{}
	if err := session.Begin(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if session.world != nil {
			session.finish(nil)
		}
	}()
	memory := session.Presentation.Memory(w.nativeCleanupMemory())
	for _, patch := range []nativeHeroPatch{{0x22, 4, 0xa00000}, {0x3b8, 2, 1}, {0xf0c, 2, 8}, {0xf0e, 2, 1}, {0xeb44, 2, 8}, {0xdde, 2, 123}, {0xde0, 2, 46}, {0xde2, 1, 20}, {0xde3, 1, 30}, {0x140, 2, 1}, {0x134, 2, 204}, {0x136, 2, 44}} {
		renderFramePatch(memory, patch)
	}
	background := make([]byte, 32000)
	callbacks := NativeFrameSessionCallbacks{Bitmap: func(address uint32) ([]byte, error) {
		if address != 0xa00000 {
			return nil, fmt.Errorf("unknown bitmap%x", address)
		}
		return background, nil
	}}
	bindings.PaintingAdvance = func(frame *NativeFrameRegisterContext) (bool, error) {
		step, err := editor.Advance(&editorRules, NativeEditorFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: codeMemory, Memory: memory, CodeBase: 0x100000, Frame: frame, Presentation: session.Presentation, Bitmap: session.bitmapAt, Sound: func(uint16, *NativeFrameRegisterContext) error { return nil }}, Image: &session.Image, Sprite: bindings.Sprites.Paint})
		return step.Complete, err
	}
	bindings.DebugOverlay = func(frame *NativeFrameRegisterContext) error {
		_, err := RenderNativeDebugFrame(NativeDebugFrameCallbacks{Code: codeMemory, CodeBase: 0x100000, Frame: frame, FormatAddress: 0x100000 + 0xef6, Bitmap: func(address uint32) (NativeBitmapWindow, error) {
			at, err := session.Presentation.chipAt(address, 32000)
			return NativeBitmapWindow{Bytes: session.Presentation.Chip, BitmapOffset: at}, err
		}})
		return err
	}
	callbacks.Render = session.RenderMain(bindings, &state)
	session.Presentation.Input.setWord(0xa, 1)
	complete, err := session.Advance(callbacks)
	if err != nil || complete || !editor.Started || state.Step != 4 || !state.painting || w.nativeCallDepth != 1 {
		t.Fatal("real editor entry was not retained", complete, err, state.Step)
	}
	for step := 0; step < 16 && !complete; step++ {
		if _, err := session.Presentation.VBlank(NativeMouseSample{}, memory, &session.Frame); err != nil {
			t.Fatal(err)
		}
		if step == 2 {
			for _, wire := range []byte{121, 120} {
				if err := session.Presentation.Input.KeyboardInterrupt(wire); err != nil {
					t.Fatal(err)
				}
			}
		}
		complete, err = session.Advance(callbacks)
		if err != nil {
			t.Fatal(err)
		}
		if !complete && (session.Phase != NativeFrameSessionRender || state.Step != 4 || !state.painting || w.nativeCallDepth != 1) {
			t.Fatal("editor wait lost raw World or source rendering position")
		}
	}
	if !complete || !editor.Complete || state.Step != 11 || session.RenderPhase != 2 || w.nativeCallDepth != 0 {
		t.Fatal("real editor did not complete its retained main frame", complete, editor.PC, state.Step)
	}
}

func TestNativeSessionRetainsSelectedActorParentAcrossChildWait(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	bindings := nativeSessionRenderTestBindings(t)
	state := NativeMainRenderState{}
	if err := session.Begin(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if session.world != nil {
			session.finish(nil)
		}
	}()
	memory := session.Presentation.Memory(w.nativeCleanupMemory())
	at := 0x76f4
	for _, patch := range []nativeHeroPatch{{0x22, 4, 0xa00000}, {0x3b8, 2, 0}, {0xf0c, 2, 8}, {0xf0e, 2, 0}, {0xeb44, 2, 2}, {0x5f44, 2, 20}, {0x5f46, 2, 20}, {0xf32, 4, 0x200000 + uint32(at)}, {0xf36, 4, 0x200000 + uint32(at)}, {0xf30, 2, 3}, {0xeb18, 2, 12}, {at, 1, 4}, {at + 1, 1, 4}, {at + 12, 1, 1}, {at + 22, 1, 6}, {at + 26, 4, 1000}} {
		renderFramePatch(memory, patch)
	}
	background := make([]byte, 32000)
	callbacks := NativeFrameSessionCallbacks{Bitmap: func(address uint32) ([]byte, error) {
		if address != 0xa00000 {
			return nil, fmt.Errorf("unknown bitmap%x", address)
		}
		return background, nil
	}}
	calls := 0
	var saved [8]uint32
	// This controlled child verifies session ownership and continuation.
	// The actual protection body has separate complete CPU/pixel references.
	bindings.Children.TownInfoAdvance = func(gotAt int, frame *NativeFrameRegisterContext) (bool, error) {
		if gotAt != at {
			t.Fatal("selected actor changed across the source wait")
		}
		calls++
		if calls == 1 {
			saved = frame.D
			frame.D[5] = 0x12345678
			return false, nil
		}
		if calls != 2 || frame.D[5] != 0x12345678 {
			t.Fatal("selected child registers or invocation count were lost")
		}
		frame.D = saved
		_ = memory.Write16(0x3b8, 1)
		return true, nil
	}
	bindings.SelectedOwnership = func(bool, *NativeFrameRegisterContext) error { return nil }
	callbacks.Render = session.RenderMain(bindings, &state)
	session.Presentation.Input.setWord(0xa, 1)
	complete, err := session.Advance(callbacks)
	if err != nil || complete || calls != 1 || state.Step != 4 || !state.selecting || w.nativeCallDepth != 1 {
		t.Fatal("selected actor wait was not retained", complete, err, calls, state.Step)
	}
	if timer, _ := memory.Read16(0xf30); timer != 2 {
		t.Fatal("selected timer did not advance exactly once", timer)
	}
	if command, _ := memory.Read16(0xeb18); command != 0xffff {
		t.Fatal("selected command word was restored before the child returned")
	}
	_ = memory.Write32(0xf36, 0)
	complete, err = session.Advance(callbacks)
	if err != nil || !complete || calls != 2 || state.Step != 11 || session.RenderPhase != 2 || w.nativeCallDepth != 0 {
		t.Fatal("selected parent did not resume to frame completion", complete, err, calls, state.Step)
	}
	if timer, _ := memory.Read16(0xf30); timer != 2 {
		t.Fatal("selected prefix was repeated after the wait", timer)
	}
	if command, _ := memory.Read16(0xeb18); command != 12 {
		t.Fatal("original selected command word was not restored", command)
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
