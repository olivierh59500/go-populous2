package populous2

import "testing"

func TestNativeRuntimeDeityUsesActualStartupAndEncodedFaces(t *testing.T) {
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
	rules, err := DecodeNativeStartupCampaignHostRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
	cb := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}}, Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}}, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}}
	deity := NativeRuntimeDeity{}
	cb.MenuChild = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		return deity.AdvanceChild(h, call, phase, cb.Campaign)
	}
	director := NativeRuntimeDirector{}
	step, err := director.Advance(h, &rules, &frame, cb)
	if err != nil || step.Complete {
		t.Fatal(step, err)
	}
	blank := func() {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frame); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 18; i++ {
		blank()
		step, err = director.Advance(h, &rules, &frame, cb)
		if err != nil {
			t.Fatal(err)
		}
	}
	nativeRuntimeClickAction(t, h, &frame, 2)
	step, err = director.Advance(h, &rules, &frame, cb)
	if err != nil || step.Complete {
		t.Fatal("real deity child not retained", step, err)
	}
	for i := 0; i < 40; i++ {
		blank()
		step, err = director.Advance(h, &rules, &frame, cb)
		if err != nil {
			t.Fatal(err)
		}
	}
	if deity.State == nil || deity.State.Finished || len(h.Files.handles) != 0 {
		t.Fatal("deity editor falsely completed or leaked resource handles")
	}
	if profile, err := h.Memory.BSS.Read16(0xeb42); err != nil || profile != 1 {
		t.Fatal("deity source profile child not executed", profile, err)
	}
	bitmap, err := h.Session.Presentation.Image()
	if err != nil || len(bitmap.Pix) != 64000 {
		t.Fatal("deity source display unavailable", err)
	}
	nativeRuntimeClickAction(t, h, &frame, 66)
	for i := 0; i < 40 && deity.State != nil; i++ {
		step, err = director.Advance(h, &rules, &frame, cb)
		if err != nil {
			t.Fatal(err)
		}
		if deity.State != nil {
			blank()
		}
	}
	if deity.State != nil || director.Menu == nil {
		t.Fatal("deity proceed did not return to the retained source menu")
	}
}
