package populous2

import "testing"

func nativeRuntimeClickAction(t *testing.T, h *NativeRuntimeHost, frame *NativeFrameRegisterContext, action int) {
	t.Helper()
	code := h.Memory.Code
	start, err := code.Read16(0xab4e)
	if err != nil {
		t.Fatal(err)
	}
	width, err := code.Read16(0xab54)
	if err != nil {
		t.Fatal(err)
	}
	col, err := code.Read16(0xab50)
	if err != nil {
		t.Fatal(err)
	}
	row, err := code.Read16(0xab52)
	if err != nil {
		t.Fatal(err)
	}
	count, x, y, found := 0, 0, 0, false
	for j := 0; j < 8192; j++ {
		v, err := code.Read8(0xab4e + int(int16(start)) + j)
		if err != nil {
			t.Fatal(err)
		}
		if v == 0 {
			break
		}
		if int8(v) > 0x5a {
			flag, err := code.Read8(0x4e92 + int(v-0x5b))
			if err != nil {
				t.Fatal(err)
			}
			if int8(flag) > 0 {
				count += 2
				if count == action {
					x = (int(col) + j%(int(width)+1)) * 8
					y = int(row) + j/(int(width)+1)*8
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Fatalf("native menu action%d unavailable", action)
	}
	p := &h.Session.Presentation.Input
	for i := 0; int(p.Mouse.PositionX) != x*2 || int(p.Mouse.PositionY) != y*2; i++ {
		if i >= 16 {
			t.Fatal("native cursor could not reach actual action")
		}
		dx := max(-100, min(100, x*2-int(p.Mouse.PositionX)))
		dy := max(-100, min(100, y*2-int(p.Mouse.PositionY)))
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(int(p.Mouse.CounterX) + dx), CounterY: uint8(int(p.Mouse.CounterY) + dy)}, h.Memory.BSS, frame); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY), Left: true}, h.Memory.BSS, frame); err != nil {
		t.Fatal(err)
	}
}

func TestNativeRuntimeDirectorRunsOriginalInitialMenuAndCustomStartup(t *testing.T) {
	nativeRuntimeDirectorMenuPath(t, 6, 4)
}

func TestNativeRuntimeDirectorRunsOriginalInitialMenuAndCampaignStartup(t *testing.T) {
	nativeRuntimeDirectorMenuPath(t, 4, 2)
}

func nativeRuntimeDirectorMenuPath(t *testing.T, action int, mode uint16) {
	t.Helper()
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
	cb := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{
		NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}},
		Campaign:                        NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio}},
		Ownership:                       func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil },
	}}
	cb.Campaign.Sound = func(cue uint16, c *NativeFrameRegisterContext) error {
		c.Word(0, cue)
		_, err := rules.Startup.Audio.Run(0x184f6, NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, c))
		return err
	}
	state := NativeRuntimeDirector{}
	step, err := state.Advance(h, &rules, &frame, cb)
	if err != nil || step.Complete || state.Menu == nil {
		t.Fatal("initial native menu not retained", step, err)
	}
	for i := 0; i < 18; i++ {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frame); err != nil {
			t.Fatal(err)
		}
		step, err = state.Advance(h, &rules, &frame, cb)
		if err != nil {
			t.Fatal(err)
		}
	}
	nativeRuntimeClickAction(t, h, &frame, action)
	for i := 0; i < 40 && !step.Complete; i++ {
		step, err = state.Advance(h, &rules, &frame, cb)
		if err != nil {
			t.Fatal(err)
		}
		if !step.Complete {
			p := &h.Session.Presentation.Input
			if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frame); err != nil {
				t.Fatal(err)
			}
		}
	}
	if mode == 2 && !step.Complete {
		if state.Selection == nil {
			t.Fatal("conquest did not reach original chooser")
		}
		for i := 0; i < 18; i++ {
			p := &h.Session.Presentation.Input
			if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frame); err != nil {
				t.Fatal(err)
			}
			step, err = state.Advance(h, &rules, &frame, cb)
			if err != nil {
				t.Fatal(err)
			}
		}
		nativeRuntimeClickAction(t, h, &frame, 6)
		for i := 0; i < 40 && !step.Complete; i++ {
			step, err = state.Advance(h, &rules, &frame, cb)
			if err != nil {
				t.Fatal(err)
			}
			if !step.Complete {
				p := &h.Session.Presentation.Input
				if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frame); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if !step.Complete || !step.FlagsKnown || !step.Zero || state.Menu != nil || state.Selection != nil {
		t.Fatal("original initial menu/custom startup did not complete", step)
	}
	if actual, err := h.Memory.BSS.Read16(0xeb44); err != nil || actual != mode {
		t.Fatal("native menu mode selection lost", actual, err)
	}
	if len(h.Files.handles) != 0 {
		t.Fatal("initial startup leaked encoded file handles")
	}
}
