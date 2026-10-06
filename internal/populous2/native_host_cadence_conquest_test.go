package populous2

import "testing"

func TestNativeHostCadenceStartsActualConquestWithoutStatueDialog(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
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
	control := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &c)
	operations := NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}
	ownership := func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }
	startup := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{
		NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &control, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}},
		Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: control,
			NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}},
		Ownership: ownership,
	}}
	blank := func(frame *NativeFrameRegisterContext) {
		t.Helper()
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, frame); err != nil {
			t.Fatal(err)
		}
	}
	director := NativeRuntimeDirector{}
	step, err := director.Advance(h, &rules, &c, startup)
	if err != nil || step.Complete || director.Menu == nil {
		t.Fatal("original initial menu did not start", step, err)
	}
	advance := func() {
		t.Helper()
		step, err = director.Advance(h, &rules, &c, startup)
		if err != nil {
			t.Fatal(err)
		}
	}
	for range 18 {
		blank(&c)
		advance()
	}
	// Select conquest using the original menu, rather than writing EB44 or
	// supplying a custom-game initializer with campaign-shaped parameters.
	nativeRuntimeClickAction(t, h, &c, 4)
	for i := 0; i < 40; i++ {
		advance()
		blank(&c)
	}
	if step.Complete || director.Selection == nil {
		t.Fatal("conquest chooser was not retained after action 4")
	}
	nativeRuntimeClickAction(t, h, &c, 6)
	for i := 0; i < 40 && !step.Complete; i++ {
		advance()
		if !step.Complete {
			blank(&c)
		}
	}
	if !step.Complete || director.Selection != nil {
		t.Fatal("original conquest action 6 did not start its world", step)
	}
	if mode, err := h.Memory.BSS.Read16(0xeb44); err != nil || mode != 2 {
		t.Fatal("original menu failed to produce campaign mode", mode, err)
	}
	frame, err := h.NewFrame(NativeRuntimeFrameBindings{Audio: operations,
		RenderChildren: NativeRuntimeRenderChildrenCallbacks{SkipCopyProtection: true, Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: device.DirectCue},
		InputChildren:  NativeGameplayHUDHostCallbacks{Campaign: startup.Campaign, Ownership: ownership, Audio: operations},
		Menu:           NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if h.Session.Phase != NativeFrameSessionIdle {
			h.Session.finish(nil)
		}
	}()
	initialInterrupt := h.Session.Presentation.Input.long(0x16)
	initialClock, err := h.Memory.BSS.Read32(0xf40)
	if err != nil {
		t.Fatal(err)
	}
	var cadence NativeHostCadence
	completed := 0
	for tick := uint64(0); tick < 200; tick++ {
		registers := &h.Session.Frame
		if h.Session.Phase == NativeFrameSessionIdle {
			registers = &c
		}
		blank(registers)
		if h.Session.Phase == NativeFrameSessionIdle {
			if !cadence.Ready(tick, NativeHostGameplayPeriod) {
				continue
			}
			if err := h.Session.BeginRaw(h.World, c); err != nil {
				t.Fatal(err)
			}
		}
		done, err := frame.Advance()
		if err != nil {
			t.Fatal("actual paced conquest failed", tick, err)
		}
		if done {
			completed++
			c = h.Session.Frame
		}
		if frame.RenderChildren.Protection.Started {
			t.Fatal("actual conquest entered the manual statue challenge")
		}
	}
	if completed < 40 || h.Session.Phase != NativeFrameSessionIdle {
		t.Fatal("conquest did not continue completing paced passes", completed, h.Session.Phase)
	}
	if interrupts := h.Session.Presentation.Input.long(0x16) - initialInterrupt; interrupts != 200 {
		t.Fatal("conquest pacing lost source PAL interrupts", interrupts)
	}
	if clock, err := h.Memory.BSS.Read32(0xf40); err != nil || clock-initialClock < uint32(completed) {
		t.Fatal("original conquest simulation clock did not advance", clock, completed, err)
	}
	if flag, err := h.Memory.BSS.Read16(0x3b8); err != nil || flag != 1 {
		t.Fatal("conquest first town did not validate the recreation session", flag, err)
	}
}
