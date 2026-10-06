package populous2

import "testing"

func TestNativeRuntimeResultHostRetainsRealInitialVBlankWait(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if complete, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); err != nil || !complete {
		t.Fatal(complete, err)
	}
	device, _, err := h.InitializeAudio(&frame, 0)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeStartupCampaignHostRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	state := NativeRuntimeResultHost{Rules: rules, Callbacks: NativeRuntimeResultCallbacks{Audio: NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}, Sound: device.DirectCue, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}}
	for i := 0; i < 100; i++ {
		if i > 0 {
			p := &h.Session.Presentation.Input
			if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frame); err != nil {
				t.Fatal(err)
			}
		}
		complete, err := state.Advance(h, 1, &frame)
		if err != nil || complete {
			t.Fatal("native result skipped its101sourcewaits", i, complete, err)
		}
		if state.Result == nil || state.Result.PC != 0x3868 {
			t.Fatal("native result recreated or escaped wait prematurely", i)
		}
	}
	if value, err := h.Memory.Code.Read16(0x3b62); err != nil || value != 1 {
		t.Fatal("elimination identity lost", value, err)
	}
}

// This starts at the independently proven 381E caller boundary. Startup,
// result, reset, menu, chooser, resource, palette and audio children all run
// their actual bodies; no callback acknowledges an unimplemented operation.
func TestNativeRuntimeResultHostRunsLossAndCustomReset(t *testing.T) {
	for _, mode := range []uint16{2, 4} {
		t.Run(map[uint16]string{2: "campaign-loss-to-conquest", 4: "campaign-loss-to-custom"}[mode], func(t *testing.T) {
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
			audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &c)
			startup := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{
				NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}},
				Campaign:                        NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}},
				Ownership:                       func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil },
			}}
			blank := func() {
				p := &h.Session.Presentation.Input
				if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &c); err != nil {
					t.Fatal(err)
				}
			}
			initial := NativeRuntimeDirector{}
			step, err := initial.Advance(h, &rules, &c, startup)
			if err != nil || step.Complete || initial.Menu == nil {
				t.Fatal("real initial menu missing", step, err)
			}
			for range 18 {
				blank()
				step, err = initial.Advance(h, &rules, &c, startup)
				if err != nil {
					t.Fatal(err)
				}
			}
			action := 6
			if mode == 2 {
				action = 4
			}
			nativeRuntimeClickAction(t, h, &c, 4)
			for i := 0; i < 40 && !step.Complete; i++ {
				step, err = initial.Advance(h, &rules, &c, startup)
				if err != nil {
					t.Fatal(err)
				}
				if !step.Complete {
					blank()
				}
			}
			if !step.Complete {
				if initial.Selection == nil {
					t.Fatal("real initial campaign chooser missing")
				}
				for range 18 {
					blank()
					step, err = initial.Advance(h, &rules, &c, startup)
					if err != nil {
						t.Fatal(err)
					}
				}
				nativeRuntimeClickAction(t, h, &c, 6)
				for i := 0; i < 40 && !step.Complete; i++ {
					step, err = initial.Advance(h, &rules, &c, startup)
					if err != nil {
						t.Fatal(err)
					}
					if !step.Complete {
						blank()
					}
				}
			}
			if !step.Complete {
				t.Fatal("true startup did not finish")
			}
			// The real normal-frame clock producer copies EB2C/EB2E to
			// God+4A before any follower result; startup alone does not.
			if done, err := h.Session.Presentation.AdvanceClock(&c, NativeFrameClockCallbacks{Memory: h.Memory.BSS}); err != nil || !done {
				t.Fatal("actual pre-result clock producer failed", done, err)
			}
			worldBefore, err := h.Memory.BSS.Read16(0xeb46)
			if err != nil {
				t.Fatal(err)
			}
			profile, err := h.Memory.BSS.Read16(0xeb42)
			if err != nil {
				t.Fatal(err)
			}
			// The result caller has already completed/recounted its follower
			// pool. Preserve that exact raw ownership through every UI wait.
			if err := h.Session.BeginRaw(h.World, c); err != nil {
				t.Fatal(err)
			}
			h.Session.Phase = NativeFrameSessionPhysics
			h.Session.Pass.Stage = NativeFrameFollowers
			h.Session.followersCompleted, h.Session.resultPending = true, true
			h.Session.resultIdentity = profile
			defer h.Session.finish(nil)
			if err := h.RefreshWorldCaches(); err == nil {
				t.Fatal("idle cache refresh accepted a retained result")
			}
			state := NativeRuntimeResultHost{Rules: rules, Startup: startup, Callbacks: NativeRuntimeResultCallbacks{Audio: NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}, Sound: device.DirectCue, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}}
			complete, err := state.Advance(h, profile, &c)
			if err != nil || complete || state.Result == nil {
				t.Fatal("result source entry failed", complete, err)
			}
			for i := 0; i < 101; i++ {
				blank()
				complete, err = state.Advance(h, profile, &c)
				if err != nil || complete {
					t.Fatal("real result wait failed", i, complete, err)
				}
			}
			if state.Result.PC != 0x39fa || state.Reset != nil {
				t.Fatal("result skipped actual continue click")
			}
			nativeRuntimeClickAction(t, h, &c, 2)
			for i := 0; i < 32 && state.Reset == nil; i++ {
				complete, err = state.Advance(h, profile, &c)
				if err != nil || complete {
					t.Fatal("result fade/reset entry failed", i, complete, err)
				}
				if state.Reset == nil {
					blank()
				}
			}
			if state.Reset == nil || state.Reset.Menu == nil || state.RefreshPending {
				t.Fatal("real reset menu was not retained")
			}
			for range 18 {
				blank()
				complete, err = state.Advance(h, profile, &c)
				if err != nil || complete {
					t.Fatal("reset menu fade failed", complete, err)
				}
			}
			nativeRuntimeClickAction(t, h, &c, action)
			for i := 0; i < 40 && !complete; i++ {
				complete, err = state.Advance(h, profile, &c)
				if err != nil {
					t.Fatal(err)
				}
				if !complete {
					blank()
				}
			}
			if mode == 2 && !complete {
				if state.Reset == nil || state.Reset.Selection == nil {
					t.Fatal("reset campaign chooser missing")
				}
				for range 18 {
					blank()
					complete, err = state.Advance(h, profile, &c)
					if err != nil || complete {
						t.Fatal("reset chooser wait failed", complete, err)
					}
				}
				nativeRuntimeClickAction(t, h, &c, 6)
				for i := 0; i < 40 && !complete; i++ {
					complete, err = state.Advance(h, profile, &c)
					if err != nil {
						t.Fatal(err)
					}
					if !complete {
						blank()
					}
				}
			}
			if !complete || state.Result != nil || state.Reset != nil || !state.RefreshPending || h.World.nativeCallDepth != 1 {
				t.Fatal("true result/reset did not return inside retained frame", complete, state.RefreshPending)
			}
			worldAfter, err := h.Memory.BSS.Read16(0xeb46)
			if err != nil {
				t.Fatal(err)
			}
			if worldAfter != worldBefore+1 {
				t.Fatal("native loss progression changed", worldBefore, worldAfter)
			}
			bssBefore, err := h.Memory.SnapshotBSS()
			if err != nil {
				t.Fatal(err)
			}
			codeBefore := nativeRuntimeResultCodeBytes(t, h)
			registers := c.D
			if err := h.RefreshResultWorldCaches(); err != nil {
				t.Fatal(err)
			}
			bssAfter, err := h.Memory.SnapshotBSS()
			if err != nil {
				t.Fatal(err)
			}
			if fileFrameHash(bssBefore) != fileFrameHash(bssAfter) || fileFrameHash(codeBefore) != fileFrameHash(nativeRuntimeResultCodeBytes(t, h)) || registers != c.D || h.World.nativeCallDepth != 1 || !h.Session.resultPending {
				t.Fatal("result cache refresh changed raw state or continuation")
			}
			if h.World.Level.Number != int(worldAfter) || h.World.NativeGameMode != mode || h.World.Custom != (mode != 2) {
				t.Fatal("post-reset decoded world remains stale")
			}
			if len(h.Files.handles) != 0 {
				t.Fatal("real reset leaked resource file handles")
			}
		})
	}
}
