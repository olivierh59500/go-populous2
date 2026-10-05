package populous2

import "testing"

func TestNativeRuntimeCampaignCallbacksSharePhysicalOwners(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	rules, err := DecodeNativeCampaignSelectionChildrenRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	children := NativeCampaignSelectionChildren{}
	cb, err := h.CampaignCallbacks(&frame, &rules, &children, NativeCampaignSelectionChildrenCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	if cb.Frame != &frame || cb.Presentation != h.Session.Presentation || cb.Child == nil {
		t.Fatal("campaign callback owner missing")
	}
	if err := cb.Memory.Write16(0xeb46, 42); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.RAM.Read16(0x20eb46); err != nil || got != 42 {
		t.Fatal("campaign BSS detached", got, err)
	}
	if err := cb.Code.Write16(0x4468, 0x1234); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.RAM.Read16(0x104468); err != nil || got != 0x1234 {
		t.Fatal("campaign CODE detached", got, err)
	}
	if _, err := h.CampaignCallbacks(&NativeFrameRegisterContext{}, &rules, &children, NativeCampaignSelectionChildrenCallbacks{}); err == nil {
		t.Fatal("campaign accepted mismatched frame base")
	}
	a := [7]NativeRequesterAddress{}
	phase := uint32(0)
	if _, err := cb.Child(NativeStartupResetFrameCall{Routine: 0x517a, Frame: &frame, A: &a}, &phase); err == nil {
		t.Fatal("campaign help acknowledged missing real audio/blitter operations")
	}
}

func TestNativeRuntimeCampaignChooserStartsAfterActualInitialPrelude(t *testing.T) {
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
	startupRules, err := DecodeNativeStartupHostFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	startup := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10a10}}
	cb, err := h.StartupCallbacks(&frame, NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }, Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x3b64 {
			t.Fatalf("initial prelude reached unexpected operation%x", call.Routine)
		}
		return NativeCommandFrameResult{}, nil
	}}}, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	step, err := startup.Advance(&startupRules, cb)
	if err != nil || step.Complete || !step.Waiting {
		t.Fatal("initial source prelude failed", step, err)
	}
	// Enter the chooser as an explicit integration operation after the real
	// destructive prelude. The still-pending initial menu is not completed.
	childRules, err := DecodeNativeCampaignSelectionChildrenRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	children := NativeCampaignSelectionChildren{}
	var blitter NativeCampaignBlitterState
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
	campaign, err := h.CampaignCallbacks(&frame, &childRules, &children, NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{Blitter: &blitter, AudioCommand: device.Command, AudioControl: audio}})
	if err != nil {
		t.Fatal(err)
	}
	selectionRules, err := DecodeNativeCampaignSelectionFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	chooser := NativeCampaignSelectionFrameState{}
	selection, err := chooser.Advance(&selectionRules, campaign)
	if err != nil || selection.Complete || !selection.Waiting {
		t.Fatal("actual chooser did not retain its first input/video gate", selection, err)
	}
	if len(h.Files.handles) != 0 {
		t.Fatal("chooser prefix leaked source resource handles")
	}
}
