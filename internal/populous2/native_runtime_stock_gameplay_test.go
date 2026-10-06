package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type nativeStockSample struct {
	X, Y uint8
	Left bool
}
type nativeStockSnapshot struct {
	Tick                                                        int
	D                                                           [8]uint32
	BSSHash, CodeHash, ChipHash, PointerHash, HeapHash, LowHash string
	Samples                                                     []nativeStockSample
}

// The fixture runs the original stock custom menu and complete E94 main,
// including the genuine protection requester. IRQ sample/poll boundaries
// come from the independent CPU, not a Go-derived gameplay schedule.
func TestNativeRuntimeStockGameplayAgainstOriginalMain(t *testing.T) {
	data, err := os.ReadFile("testdata/native_runtime_stock_gameplay_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Startup      nativeStockSnapshot
			StartupPolls []int
			Frames       []nativeStockSnapshot
			FrameInputs  []struct {
				Tick    int
				Samples []nativeStockSample
				Polls   []int
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 1 {
		t.Fatal("stock main corpus incomplete", err)
	}
	f := catalog.Cases[0]
	if len(f.Frames) != 13 || len(f.FrameInputs) != 1000 || len(f.StartupPolls) == 0 {
		t.Fatal("stock main checkpoints or inputs truncated")
	}
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
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: s.X, CounterY: s.Y, Left: s.Left}, h.Memory.BSS, frame); err != nil {
			t.Fatal(err)
		}
	}
	director := NativeRuntimeDirector{}
	pos := 0
	for _, end := range f.StartupPolls {
		if end < pos || end > len(f.Startup.Samples) {
			t.Fatal("invalid original startup poll")
		}
		for ; pos < end; pos++ {
			apply(f.Startup.Samples[pos], &c)
		}
		step, err := director.Advance(h, &rules, &c, startup)
		if err != nil {
			t.Fatal(err)
		}
		if step.Complete && end != len(f.Startup.Samples) {
			t.Fatal("native startup returned before original source")
		}
	}
	if pos != len(f.Startup.Samples) || !director.ready {
		t.Fatal("native stock startup did not complete")
	}
	check := func(want nativeStockSnapshot) {
		if c.D != want.D {
			t.Fatalf("stock tick%d D differs: got%x want%x", want.Tick, c.D, want.D)
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
	check(f.Startup)
	frame, err := h.NewFrame(NativeRuntimeFrameBindings{Audio: audio, RenderChildren: NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: device.DirectCue}, InputChildren: NativeGameplayHUDHostCallbacks{Campaign: startup.Campaign, Ownership: ownership, Audio: audio}, Menu: NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := 0
	for tick, events := range f.FrameInputs {
		if events.Tick != tick || len(events.Polls) == 0 {
			t.Fatal("original frame input sequence incomplete")
		}
		if err := h.Session.BeginRaw(h.World, c); err != nil {
			t.Fatal(err)
		}
		pos, complete := 0, false
		for j, end := range events.Polls {
			if end < pos || end > len(events.Samples) {
				t.Fatal("invalid original main poll")
			}
			for ; pos < end; pos++ {
				apply(events.Samples[pos], &h.Session.Frame)
			}
			complete, err = frame.Advance()
			if err != nil {
				t.Fatalf("stock tick%d poll%d: %v", tick, j, err)
			}
			if complete && j != len(events.Polls)-1 {
				t.Fatalf("stock tick%d returned before original wait boundary", tick)
			}
		}
		if !complete || pos != len(events.Samples) {
			t.Fatalf("stock tick%d source frame remained pending: phase%d render%d protection%x", tick, h.Session.Phase, frame.RenderState.Step, frame.RenderChildren.ProtectionStep.PC)
		}
		c = h.Session.Frame
		if checkpoint < len(f.Frames) && f.Frames[checkpoint].Tick == tick {
			check(f.Frames[checkpoint])
			checkpoint++
		}
	}
	if checkpoint != 13 || h.World.nativeCallDepth != 0 || h.Session.Phase != NativeFrameSessionIdle {
		t.Fatal("stock main checkpoints/borrow incomplete")
	}
}
