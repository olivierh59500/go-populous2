package populous2

import "testing"

func TestNativeFrameSessionRetainsInputAfterCommandsWithoutReplayingSwap(t *testing.T) {
	w, s := nativeSessionTestSetup(t)
	if err := s.Begin(w, NativeFrameRegisterContext{}); err != nil {
		t.Fatal(err)
	}
	s.Presentation.Input.setWord(0xa, 1)
	renders, inputs := 0, 0
	cb := NativeFrameSessionCallbacks{Render: func(FollowerCleanupMemory, *NativeFrameRegisterContext, *NativeImageRenderState, []byte, *uint32) (bool, error) {
		renders++
		return true, nil
	}, Input: func(m FollowerCleanupMemory, c *NativeFrameRegisterContext, phase *uint32) (bool, error) {
		inputs++
		if w.nativeCallDepth != 1 || s.Pass.Stage != NativeFrameFinished || s.imageAudioCode != nil {
			t.Fatal("input entered before completed borrowed frame")
		}
		if *phase == 0 {
			*phase = 1
			c.D[3] = 0x12345678
			return false, nil
		}
		if c.D[3] != 0x12345678 {
			t.Fatal("input lost retained register")
		}
		return true, m.Write8(0xeb57, 14)
	}}
	if done, err := s.Advance(cb); err != nil || done || s.Phase != NativeFrameSessionInput {
		t.Fatal("input wait not retained", done, err)
	}
	selector := s.Presentation.CopperSelector
	if done, err := s.Advance(cb); err != nil || !done || renders != 1 || inputs != 2 || s.Presentation.CopperSelector != selector || w.nativeCallDepth != 0 {
		t.Fatal("input resumption replayed frame work", done, err, renders, inputs)
	}
	if command, err := w.nativeCleanupMemory().Read8(0xeb57); err != nil || command != 14 {
		t.Fatal("input command was executed or cleared in the same frame", command, err)
	}
}

func TestNativeRuntimeSessionRejectsMissingGameplaySuffix(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []nativeHeroPatch{{0x138, 2, 20}, {0x3b0, 2, 1}, {0xf3c, 2, 1}} {
		renderFramePatch(h.Memory.BSS, p)
	}
	if err := h.Session.BeginRaw(h.World, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	h.Session.Presentation.Input.setWord(0xa, 1)
	done, err := h.Session.Advance(NativeFrameSessionCallbacks{Render: func(FollowerCleanupMemory, *NativeFrameRegisterContext, *NativeImageRenderState, []byte, *uint32) (bool, error) {
		return true, nil
	}})
	if err == nil || done || h.World.nativeCallDepth != 0 {
		t.Fatal("runtime completed without the native input suffix", done, err)
	}
}
