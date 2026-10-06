package populous2

import "testing"

func TestNativeRuntimeCommandRestartRetainsRawBorrowAndDefersCaches(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	if done, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); err != nil || !done {
		t.Fatal(done, err)
	}
	device, _, err := h.InitializeAudio(&frame, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []nativeHeroPatch{{0xeb44, 2, 4}, {0xeb42, 2, 1}, {0xeb46, 2, 0}, {0xeb22, 2, 0}, {0x3ac, 4, 0xffffffff}} {
		renderFramePatch(h.Memory.BSS, p)
	}
	background, err := h.Memory.BSS.Read32(0xdbe)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write32(0x22, background); err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeStartupCampaignHostRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
	state := NativeRuntimeCommandChildren{Host: h, Rules: rules, Audio: audio, Supplied: NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio}, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}}}
	h.World.nativeCallDepth++
	context := NativeCommandRegisterContext{D: frame.D}
	phase := uint32(0)
	step, err := state.Call(NativeCommandFrameCall{NativeCommandCall: NativeCommandCall{Routine: 0x10ad8, Caller: 0xeb56, Context: &context}}, &phase)
	if err != nil || !step.Complete || !state.RefreshPending || h.World.nativeCallDepth != 1 {
		t.Fatal("source restart lost raw continuation or refreshed inside borrowed frame", step, err)
	}
	h.World.nativeCallDepth--
	if err := h.RefreshWorldCaches(); err != nil {
		t.Fatal(err)
	}
}
