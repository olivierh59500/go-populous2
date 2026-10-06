package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type nativeStockSample struct {
	X, Y uint8
	Key  uint8
	Left bool
}
type nativeStockSnapshot struct {
	Tick                                                        int
	Label                                                       string
	Routine                                                     uint32
	D                                                           [8]uint32
	BSSHash, CodeHash, ChipHash, PointerHash, HeapHash, LowHash string
	Samples                                                     []nativeStockSample
	Polls                                                       []int
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
			Startup                 nativeStockSnapshot
			ResultEntry, ResultWait nativeStockSnapshot
			ResultFlow              []nativeStockSnapshot
			StartupPolls            []int
			Frames                  []nativeStockSnapshot
			FrameInputs             []struct {
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
	if len(f.Frames) != 15 || len(f.FrameInputs) != 2499 || len(f.StartupPolls) == 0 {
		t.Fatal("stock main checkpoints or inputs truncated")
	}
	g := nativeRuntimeGameplayStart(t, f.Startup, f.StartupPolls)
	h, c, rules, audio, startup, apply := g.Host, g.Frame, g.Rules, g.Audio, g.Startup, g.Apply
	check := func(want nativeStockSnapshot, d [8]uint32) { nativeRuntimeGameplayCheck(t, h, want, d) }
	check(f.Startup, c.D)
	result := NativeRuntimeResultHost{Rules: rules, Startup: startup, Callbacks: NativeRuntimeResultCallbacks{Audio: audio, Sound: audio.DirectCue, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}}
	entered := false
	resultAdvance := func(identity uint16, ctx *NativeFrameRegisterContext) (bool, error) {
		if !entered {
			check(f.ResultEntry, ctx.D)
			entered = true
		}
		done, err := result.Advance(h, identity, ctx)
		if err == nil && done && result.RefreshPending {
			if err = h.RefreshResultWorldCaches(); err != nil {
				return false, err
			}
			result.RefreshPending = false
		}
		return done, err
	}
	frame := g.newFrame(t, resultAdvance)
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
		if tick == 2498 {
			if complete || !entered || result.Result == nil || result.Result.PC != 0x3868 {
				t.Fatal("natural result did not retain its original wait")
			}
			check(f.ResultWait, h.Session.Frame.D)
			break
		}
		if !complete || pos != len(events.Samples) {
			t.Fatalf("stock tick%d source frame remained pending: phase%d render%d protection%x", tick, h.Session.Phase, frame.RenderState.Step, frame.RenderChildren.ProtectionStep.PC)
		}
		c = h.Session.Frame
		if checkpoint < len(f.Frames) && f.Frames[checkpoint].Tick == tick {
			check(f.Frames[checkpoint], c.D)
			checkpoint++
		}
	}
	if checkpoint != 14 || !entered || len(f.ResultFlow) != 163 {
		t.Fatal("stock natural result checkpoints incomplete", checkpoint, len(f.ResultFlow))
	}
	newFrames, uiWaits := 0, 0
	for index, event := range f.ResultFlow {
		if event.Label == "new-world" {
			if h.Session.Phase != NativeFrameSessionIdle {
				t.Fatal("previous source result frame remained pending")
			}
			c = h.Session.Frame
			if err := h.Session.BeginRaw(h.World, c); err != nil {
				t.Fatal(err)
			}
		}
		pos, complete := 0, false
		for _, end := range event.Polls {
			if end < pos || end > len(event.Samples) {
				t.Fatal("invalid result/reset input poll")
			}
			for ; pos < end; pos++ {
				apply(event.Samples[pos], &h.Session.Frame)
			}
			complete, err = frame.Advance()
			if err != nil {
				t.Fatalf("result event%d %s: %v", index, event.Label, err)
			}
		}
		if pos != len(event.Samples) {
			t.Fatal("result event inputs truncated")
		}
		if event.Label == "new-world" {
			newFrames++
			if !complete {
				t.Fatal("new stock world frame did not return", index)
			}
		} else if event.Routine == 0xe94 {
			if !complete || result.Result != nil || result.Reset != nil {
				t.Fatal("actual reset did not return to main")
			}
		} else if complete {
			t.Fatal("result/reset source wait completed prematurely", index, event.Label)
		}
		if event.Label == "result-blank" {
			uiWaits++
		}
		check(event, h.Session.Frame.D)
	}
	if newFrames != 10 || uiWaits != 100 || h.World.nativeCallDepth != 0 || h.Session.Phase != NativeFrameSessionIdle {
		t.Fatal("stock result/reset/new-world coverage incomplete", newFrames, uiWaits)
	}
}
