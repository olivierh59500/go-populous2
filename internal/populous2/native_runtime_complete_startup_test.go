package populous2

import "testing"

func TestNativeRuntimeCustomStartupCompletesWithActualPanelAndAudio(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	c := NativeFrameRegisterContext{AddressBase: 0x200000}
	if done, err := h.AdvanceAllocations(0x1a43e, &c, NativeErrorFrameCallbacks{}); err != nil || !done {
		t.Fatal(done, err)
	}
	device, _, err := h.InitializeAudio(&c, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []struct {
		at    int
		value uint16
	}{{0xeb44, 4}, {0xeb42, 1}, {0xeb46, 0}, {0xeb22, 0}} {
		if err := h.Memory.BSS.Write16(patch.at, patch.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Memory.BSS.Write32(0x3ac, 0xffffffff); err != nil {
		t.Fatal(err)
	}
	background, err := h.Memory.BSS.Read32(0xdbe)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write32(0x22, background); err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeStartupHostFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	panel, err := DecodeNativeStartupPanelFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	state := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10ad8}}
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &c)
	panelCalls := 0
	cb, err := h.StartupCallbacks(&c, NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x1da0 {
			t.Fatalf("complete custom startup still required body%x", call.Routine)
		}
		panelCalls++
		err := h.RestoreStartupPanel(&panel, call.Frame, call.A, func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil })
		return NativeCommandFrameResult{Complete: err == nil}, err
	}}}, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	step, err := state.Advance(&rules, cb)
	if err != nil || !step.Complete || !step.FlagsKnown || !step.Zero || step.Negative || panelCalls != 1 {
		t.Fatal("complete custom source startup failed", step, panelCalls, err)
	}
	if h.ImageAudioCode.Owner() != NativeImageCodeOwner {
		t.Fatal("startup returned with wrong image/audio owner")
	}
	if len(h.Files.handles) != 0 {
		t.Fatal("complete startup retained resource handles")
	}
	if err := h.Session.BeginRaw(h.World, c); err != nil {
		t.Fatal("completed source startup cannot enter native frame", err)
	}
	h.Session.finish(nil)
}
