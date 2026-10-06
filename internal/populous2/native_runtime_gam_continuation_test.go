package populous2

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeRuntimeGAMBrowserRestoresSimulationContinuation(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(fmt.Sprint(async), func(t *testing.T) { testNativeRuntimeGAMBrowserContinuation(t, async) })
	}
}

func testNativeRuntimeGAMBrowserContinuation(t *testing.T, async bool) {
	h := nativeRuntimeHostTest(t)
	device := runtimeFilesPrelude(t, h)
	c := runtimeFilesCampaign(t, h, device)
	// Real campaign startup supplies the saved LAND and linked actors. This
	// configuration avoids normal input/result UI while comparing resumed
	// simulation; browser/resource/palette/DOS operations run their real bodies.
	if err := h.Memory.BSS.Write16(0xeb44, 8); err != nil {
		t.Fatal(err)
	}
	if err := h.RefreshWorldCaches(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	store, err := NewNativeRuntimeFileStore(root, []string{"SAVES"}, async)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rules, err := DecodeNativeRuntimeFileBrowserRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	browser := func(save bool) {
		t.Helper()
		value := uint16(0)
		if save {
			value = 1
		}
		if err := h.Memory.Code.Write16(0x4468, value); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.Code.Write16(0x3f90, 0); err != nil {
			t.Fatal(err)
		}
		for _, v := range []struct {
			at   int
			text string
		}{{0x4440, "SAVES:"}, {0x4416, "CONTINUE"}} {
			for i, b := range append([]byte(v.text), 0) {
				if err := h.Memory.Code.Write8(v.at+i, b); err != nil {
					t.Fatal(err)
				}
			}
		}
		state := NativeRuntimeFileBrowserState{}
		phase := uint32(0)
		var a [7]NativeRequesterAddress
		cb := NativeRuntimeFileBrowserCallbacks{Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }, NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}
		step, err := state.AdvanceChild(h, store, &rules, NativeStartupResetFrameCall{Routine: 0x3f92, Frame: &c, A: &a}, &phase, cb)
		if err != nil || step.Complete {
			t.Fatal("browser entry failed", step, err)
		}
		runtimeFilesClick(t, h, &c, 34)
		for tick := 0; tick < 80 && !step.Complete; tick++ {
			deadline := time.Now().Add(5 * time.Second)
			for {
				step, err = state.AdvanceChild(h, store, &rules, NativeStartupResetFrameCall{Routine: 0x3f92, Frame: &c, A: &a}, &phase, cb)
				if err != nil {
					t.Fatal(err)
				}
				if step.Complete || !state.Browser.ChildActive || state.Browser.ChildRoutine == 0x102e4 || state.Files.Overwrite != nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("actual asynchronous file operation did not return")
				}
				time.Sleep(time.Millisecond)
			}
			if !step.Complete {
				p := &h.Session.Presentation.Input
				if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &c); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !step.Complete {
			t.Fatalf("browser did not finish source save/load save%v PC%x", save, state.Browser.PC)
		}
		if state.Files.RefreshPending {
			if err := state.Files.RefreshLoadedViews(h); err != nil {
				t.Fatal(err)
			}
		}
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &c); err != nil {
			t.Fatal(err)
		}
	}
	browser(true)
	saved, err := os.ReadFile(filepath.Join(root, "CONTINUE.GAM"))
	if err != nil || len(saved) != NativeGAMSize {
		t.Fatal("browser did not save native GAM", len(saved), err)
	}
	var expected [][]byte
	simulate := func() {
		t.Helper()
		if err := h.Session.BeginRaw(h.World, c); err != nil {
			t.Fatal(err)
		}
		defer h.Session.finish(nil)
		h.Session.bitmapResolver = h.Bitmap
		h.Session.directSound = func(cue uint16) error { return device.DirectCue(cue, &h.Session.Frame) }
		cb := NativeFrameSessionCallbacks{Audio: NativeFrameAudioCallbacks{Command: device.Command}, DirectSound: h.Session.directSound, Bitmap: h.Bitmap}
		for tick := 0; tick < 20; tick++ {
			clock, err := h.Memory.BSS.Read32(0xf40)
			if err != nil {
				t.Fatal(err)
			}
			if err = h.Memory.BSS.Write32(0xf40, clock+1); err != nil {
				t.Fatal(err)
			}
			h.Session.Audio.Entries = h.Session.Image.AudioBank
			if err := h.ImageAudioCode.SetOwner(NativeAudioCodeOwner); err != nil {
				t.Fatal(err)
			}
			bitmap, err := h.Session.Presentation.BackBuffer()
			if err != nil {
				t.Fatal(err)
			}
			physics := h.Session.physicsCallbacks(cb, bitmap)
			physics.Swap = func(*NativeFrameRegisterContext) (bool, error) { return false, nil }
			done, err := h.Session.Pass.TickFramePass(&h.Session.Frame, physics)
			if err != nil {
				t.Fatal(tick, err)
			}
			if done || h.Session.Pass.Stage != NativeFrameSwap {
				t.Fatal("simulation did not stop before the real swap", done, h.Session.Pass.Stage)
			}
			raw, err := h.Memory.SnapshotBSS()
			if err != nil {
				t.Fatal(err)
			}
			if len(expected) <= tick {
				expected = append(expected, append([]byte(nil), raw[NativeGAMStart:NativeGAMEnd]...))
			} else if !bytes.Equal(expected[tick], raw[NativeGAMStart:NativeGAMEnd]) {
				t.Fatalf("native GAM resumed simulation diverged at pass%d", tick)
			}
			h.Session.Image.AudioBank = h.Session.Audio.Entries
			if err := h.ImageAudioCode.SetOwner(NativeImageCodeOwner); err != nil {
				t.Fatal(err)
			}
			if _, err := h.Session.Presentation.Swap(&h.Session.Frame); err != nil {
				t.Fatal(err)
			}
			h.Session.Pass = NativeFramePassState{}
		}
		c = h.Session.Frame
	}
	expected = make([][]byte, 0, 20)
	simulate()
	browser(false)
	simulate()
}
