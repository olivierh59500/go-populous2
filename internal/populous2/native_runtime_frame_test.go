package populous2

import "testing"

func TestNativeRuntimeFrameRunsAfterActualInitialMenuStartup(t *testing.T) {
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
	rules, err := DecodeNativeStartupCampaignHostRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	audio := NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}
	control := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &c)
	ownership := func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }
	sound := func(cue uint16, frame *NativeFrameRegisterContext) error { return device.DirectCue(cue, frame) }
	cb := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}, Audio: &control}, Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: control, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: sound}}}}, Ownership: ownership}}
	director := NativeRuntimeDirector{}
	step, err := director.Advance(h, &rules, &c, cb)
	if err != nil {
		t.Fatal(err)
	}
	blank := func() {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &c); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 18; i++ {
		blank()
		step, err = director.Advance(h, &rules, &c, cb)
		if err != nil {
			t.Fatal(err)
		}
	}
	nativeRuntimeClickAction(t, h, &c, 6)
	for i := 0; i < 40 && !step.Complete; i++ {
		step, err = director.Advance(h, &rules, &c, cb)
		if err != nil {
			t.Fatal(err)
		}
		if !step.Complete {
			blank()
		}
	}
	if !step.Complete {
		t.Fatal("actual native startup did not complete")
	}
	frame, err := h.NewFrame(NativeRuntimeFrameBindings{Audio: audio,
		RenderChildren: NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: sound},
		InputChildren:  NativeGameplayHUDHostCallbacks{Campaign: cb.Campaign, Ownership: ownership, Audio: audio},
		Menu:           NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }},
	})
	if err != nil {
		t.Fatal(err)
	}
	// This test uses a single synchronous owner. Source keyboard/click latches
	// are released with a real IRQ; no synthetic World.Tick replaces physics.
	for iteration := 0; iteration < 120; iteration++ {
		if h.Session.Phase == NativeFrameSessionIdle {
			if err := h.Session.BeginRaw(h.World, c); err != nil {
				t.Fatal(err)
			}
		}
		blank()
		complete, err := frame.Advance()
		if err != nil {
			t.Fatalf("native frame%d failed:%v", iteration, err)
		}
		if complete {
			c = h.Session.Frame
		}
	}
	for wait := 0; h.Session.Phase != NativeFrameSessionIdle && wait < 20; wait++ {
		blank()
		if _, err := frame.Advance(); err != nil {
			t.Fatal(err)
		}
	}
	if h.World.nativeCallDepth != 0 || h.Session.Phase != NativeFrameSessionIdle {
		if h.Session.Phase != NativeFrameSessionRender || frame.RenderChildren.ProtectionStep.PC == 0 || h.World.nativeCallDepth != 1 {
			t.Fatalf("unexpected native frame wait:phase%d render%d editor%x protection%x input%x", h.Session.Phase, frame.RenderState.Step, frame.RenderChildren.EditorStep.PC, frame.RenderChildren.ProtectionStep.PC, frame.InputState.PC)
		}
		// The original town draw entered its real protection requester. It
		// remains borrowed until a user supplies the requested answer.
		defer h.Session.finish(nil)
	}
	output := make([]byte, 320*200*4)
	if err := h.Session.Presentation.WriteRGBA(output, true); err != nil {
		t.Fatal(err)
	}
}
