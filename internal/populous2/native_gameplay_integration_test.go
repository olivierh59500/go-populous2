package populous2

import (
	"fmt"
	"testing"
)

// This integration starts with the real physical loader, audio initializer,
// one-time 10A10 prelude and stock 10AD8 custom constructor. The main-menu
// child is a genuine pending boundary; its prefix is not replaced by NewWorld.
func nativeGameplayIntegrationStartup(t *testing.T, land int, seed uint32) (*NativeRuntimeHost, *NativeAudioDevice, NativeFrameRegisterContext) {
	t.Helper()
	h := nativeRuntimeHostTest(t)
	if h.FollowerCode == nil {
		t.Fatal("native host follower CODE owner missing")
	}
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if err := h.Memory.BSS.Write32(0x14c, 0xc00000); err != nil {
		t.Fatal(err)
	}
	if done, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); err != nil || !done {
		t.Fatal(done, err)
	}
	device, _, err := h.InitializeAudio(&frame, 0)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeStartupHostFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	prelude := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10a10}}
	cb, err := h.StartupCallbacks(&frame, NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }, Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x3b64 {
			return NativeCommandFrameResult{}, fmt.Errorf("unexpected initial source child%x", call.Routine)
		}
		return NativeCommandFrameResult{}, nil
	}}}, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	if step, err := prelude.Advance(&rules, cb); err != nil || step.Complete {
		t.Fatal(step, err)
	}
	for _, p := range []struct {
		at    int
		value uint16
	}{{0xeb44, 4}, {0xeb42, 1}, {0xeb46, 0}, {0xeb22, uint16(land)}} {
		if err := h.Memory.BSS.Write16(p.at, p.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Memory.BSS.Write32(0xeb28, seed); err != nil {
		t.Fatal(err)
	}
	panel, err := DecodeNativeStartupPanelFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
	cb, err = h.StartupCallbacks(&frame, NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x1da0 {
			return NativeCommandFrameResult{}, fmt.Errorf("unexpected constructor child%x", call.Routine)
		}
		err := h.RestoreStartupPanel(&panel, call.Frame, call.A, func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil })
		return NativeCommandFrameResult{Complete: err == nil}, err
	}}}, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	startup := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10ad8, A: prelude.Startup.A}}
	if step, err := startup.Advance(&rules, cb); err != nil || !step.Complete {
		t.Fatal(step, err)
	}
	if err := h.RefreshWorldCaches(); err != nil {
		t.Fatal(err)
	}
	return h, device, frame
}

func TestNativeGameplayIntegrationAgainstOriginalCPU(t *testing.T) {
	runNativeGameplayCPUCorpus(t, "testdata/native_gameplay_integration_native.json", 124)
}
