package populous2

import "testing"

type nativeRuntimeGameplayTestSetup struct {
	Host    *NativeRuntimeHost
	Frame   NativeFrameRegisterContext
	Rules   NativeStartupCampaignHostRules
	Audio   NativeRuntimeAudioOperations
	Startup NativeRuntimeDirectorCallbacks
	Apply   func(nativeStockSample, *NativeFrameRegisterContext)
}

func nativeRuntimeGameplayStart(t *testing.T, snapshot nativeStockSnapshot, polls []int) nativeRuntimeGameplayTestSetup {
	t.Helper()
	h := nativeRuntimeHostTest(t)
	if err := h.Memory.BSS.Write32(0x14c, 0xc00000); err != nil {
		t.Fatal(err)
	}
	c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if _, err := h.InterruptVectors(0x39e, &c, func(uint32) (uint16, error) { return 0, nil }, nil); err != nil {
		t.Fatal(err)
	}
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
	audio := NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}
	control := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &c)
	ownership := func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }
	startup := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &control, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}}, Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: control, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}}, Ownership: ownership}}
	apply := func(s nativeStockSample, frame *NativeFrameRegisterContext) {
		if s.Key != 0 {
			if err := h.Session.Presentation.Input.KeyboardInterrupt(s.Key); err != nil {
				t.Fatal(err)
			}
			return
		}
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: s.X, CounterY: s.Y, Left: s.Left, Right: s.Right}, h.Memory.BSS, frame); err != nil {
			t.Fatal(err)
		}
	}
	director := NativeRuntimeDirector{}
	pos := 0
	for _, end := range polls {
		if end < pos || end > len(snapshot.Samples) {
			t.Fatal("invalid original startup poll")
		}
		for ; pos < end; pos++ {
			apply(snapshot.Samples[pos], &c)
		}
		step, err := director.Advance(h, &rules, &c, startup)
		if err != nil {
			t.Fatal(err)
		}
		if step.Complete && end != len(snapshot.Samples) {
			t.Fatal("native startup returned before original source")
		}
	}
	if pos != len(snapshot.Samples) || !director.ready {
		t.Fatal("native stock startup did not complete")
	}
	return nativeRuntimeGameplayTestSetup{Host: h, Frame: c, Rules: rules, Audio: audio, Startup: startup, Apply: apply}
}
func nativeRuntimeGameplayCheck(t *testing.T, h *NativeRuntimeHost, want nativeStockSnapshot, d [8]uint32) {
	t.Helper()
	if len(want.Heights) > 0 {
		if len(want.Heights) != 9 {
			t.Fatal("native corner neighborhood truncated")
		}
		rules, err := DecodeNativeRenderFrameRules(h.Bundle.Executable)
		if err != nil {
			t.Fatal(err)
		}
		if err := rules.BindCode(h.Memory.Code, h.Code.Logical().Read32); err != nil {
			t.Fatal(err)
		}
		for i, expected := range want.Heights {
			c := NativeFrameRegisterContext{D: [8]uint32{uint32(want.HeightOrigin[0] + i%3), uint32(want.HeightOrigin[1] + i/3)}, AddressBase: h.Memory.BSSBase}
			if err := rules.TerrainHeight(h.Memory.BSS, &c); err != nil || int32(c.D[2]) != expected {
				t.Fatalf("native tick%d corner%d height differs:got%d want%d err%v", want.Tick, i, int32(c.D[2]), expected, err)
			}
		}
		for i, at := range []int{0x5f4c, 0x5f4e} {
			got, err := h.Memory.BSS.Read16(at)
			if err != nil || got != want.Picked[i] {
				t.Fatal("native mouse surface selection differs", got, want.Picked, err)
			}
		}
	}
	check := func(want nativeStockSnapshot, d [8]uint32) {
		if d != want.D {
			t.Fatalf("stock tick%d D differs: got%x want%x", want.Tick, d, want.D)
		}
		bss, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		low, err := h.Host.Span(0, 256)
		if err != nil {
			t.Fatal(err)
		}
		fx, err := h.Host.Span(0x700000, NativeStartupAudioBytes)
		if err != nil {
			t.Fatal(err)
		}
		background, err := h.Bitmap(0x700000 + NativeStartupAudioBytes)
		if err != nil {
			t.Fatal(err)
		}
		heap := append(append([]byte(nil), fx...), background...)
		p := h.Session.Presentation
		for _, pair := range []struct{ name, got, want string }{{"BSS", fileFrameHash(bss), want.BSSHash}, {"CODE", fileFrameHash(nativeRuntimeResultCodeBytes(t, h)), want.CodeHash}, {"screens/Copper", fileFrameHash(p.Chip), want.ChipHash}, {"pointer", fileFrameHash(p.PointerData[:15260]), want.PointerHash}, {"heap", fileFrameHash(heap), want.HeapHash}, {"low RAM", fileFrameHash(low), want.LowHash}} {
			if pair.got != pair.want {
				t.Fatalf("stock tick%d %s differs: got%s want%s", want.Tick, pair.name, pair.got, pair.want)
			}
		}
	}
	check(want, d)
}
func (g nativeRuntimeGameplayTestSetup) newFrame(t *testing.T, result func(uint16, *NativeFrameRegisterContext) (bool, error)) *NativeRuntimeFrame {
	t.Helper()
	ownership := func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }
	frame, err := g.Host.NewFrame(NativeRuntimeFrameBindings{Audio: g.Audio, Session: NativeFrameSessionCallbacks{ResultAdvance: result}, RenderChildren: NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: g.Audio.DirectCue}, InputChildren: NativeGameplayHUDHostCallbacks{Campaign: g.Startup.Campaign, Ownership: ownership, Audio: g.Audio}, Menu: NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}
