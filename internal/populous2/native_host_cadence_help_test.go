package populous2

import "testing"

func TestNativeHostCadenceRetainsRealHelpAnimationAndClick(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
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
	supplied := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{
		NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &control, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}},
		Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: control,
			NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}},
		Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil },
	}}
	director := NativeRuntimeDirector{}
	advance := func() {
		t.Helper()
		step, err := director.Advance(h, &rules, &c, supplied)
		if err != nil || step.Complete {
			t.Fatal("conquest chooser must remain an actual pending source operation", step, err)
		}
	}
	irq := func(left bool) {
		t.Helper()
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY), Left: left}, h.Memory.BSS, &c); err != nil {
			t.Fatal(err)
		}
	}
	advance()
	for blank := 0; blank < 18; blank++ {
		irq(false)
		advance()
	}
	nativeRuntimeClickAction(t, h, &c, 4)
	for blank := 0; blank < 40; blank++ {
		advance()
		irq(false)
	}
	if director.Selection == nil {
		t.Fatal("actual conquest menu did not retain its chooser")
	}
	// Invoke the chooser's genuine help child with its original register
	// argument. LAND loading, descriptor preparation, text and preview drawing
	// remain the real source operations; the chooser itself is not completed.
	profile, err := h.Memory.BSS.Read16(0xeb42)
	if err != nil {
		t.Fatal(err)
	}
	slot := -1
	for i := 0; i < 36; i++ {
		descriptor, e := h.Memory.Code.Read16(0x21102 + i*2)
		if e != nil {
			t.Fatal(e)
		}
		flag, e := h.Memory.BSS.Read8(0xe76a + int(int16(profile*314)) + 0x70 + i)
		if e != nil {
			t.Fatal(e)
		}
		branch, e := h.Memory.Code.Read16(0x52ac + i*2)
		if e != nil {
			t.Fatal(e)
		}
		body := 0x52ac + int(int16(branch))
		if descriptor != 0 && int8(flag) > 0 && body != 0x5526 && body != 0x53cc && body != 0x53e0 {
			slot = i
			break
		}
	}
	if slot < 0 {
		t.Fatal("original world has no admitted help icon")
	}
	c.Word(1, uint16(slot*2))
	var child NativeCampaignSelectionChildren
	var blitter NativeCampaignBlitterState
	supplied.Campaign.Blitter = &blitter
	cb, err := h.CampaignCallbacks(&c, &rules.Children, &child, supplied.Campaign)
	if err != nil {
		t.Fatal(err)
	}
	a, phase := [7]NativeRequesterAddress{}, uint32(0)
	call := NativeStartupResetFrameCall{Routine: 0x517a, Frame: &c, A: &a}
	for n := 0; !child.Help.Started || child.Help.PC != 0x51dc; n++ {
		if n >= 40 {
			t.Fatal("actual help did not reach its source redraw wait")
		}
		result, err := cb.Child(call, &phase)
		if err != nil || result.Complete {
			t.Fatal("original help returned before a close click", result, err)
		}
		irq(false)
	}
	word := func(at int) uint16 {
		t.Helper()
		v, err := h.Memory.Code.Read16(at)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	var cadence NativeHostCadence
	updates, previous := 0, word(0x552a)
	initialBlank, err := h.Memory.BSS.Read32(0x16)
	if err != nil {
		t.Fatal(err)
	}
	for blank := uint64(0); blank < 50; blank++ {
		irq(false)
		if cadence.Ready(blank, NativeHostHelpPeriod) {
			result, err := cb.Child(call, &phase)
			if err != nil || result.Complete || child.Help.PC != 0x51dc {
				t.Fatal("paced help lost its genuine redraw continuation", result, err)
			}
			updates++
			current := word(0x552a)
			if current == previous {
				t.Fatal("help preview did not advance its original CODE animation", slot, current)
			}
			previous = current
		} else if word(0x552a) != previous {
			t.Fatal("help preview advanced during an unadmitted PAL interrupt")
		}
	}
	finalBlank, err := h.Memory.BSS.Read32(0x16)
	if err != nil || finalBlank-initialBlank != 50 || updates != 10 {
		t.Fatal("help pacing reduced source interrupts", finalBlank-initialBlank, updates, err)
	}
	// A close click arrives between scheduled previews. The source IRQ keeps
	// it pending after button release, so the host can immediately run the
	// genuine click/exit body without waiting for the next preview deadline.
	nativeRuntimeClickAction(t, h, &c, 2)
	irq(false)
	pending, err := h.Memory.BSS.Read16(0x140)
	if err != nil || pending == 0 {
		t.Fatal("close click was lost while the help preview was paced", pending, err)
	}
	result, err := cb.Child(call, &phase)
	if err != nil || !result.Complete || !child.Help.Finished {
		t.Fatal("pending close click did not complete the actual help body", result, err)
	}
}
