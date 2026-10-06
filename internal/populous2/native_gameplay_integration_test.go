package populous2

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// This integration starts with the real physical loader, audio initializer,
// one-time 10A10 prelude and stock 10AD8 custom constructor. The main-menu
// child is a genuine pending boundary; its prefix is not replaced by NewWorld.
func nativeGameplayIntegrationStartup(t *testing.T, land int, seed uint32) (*NativeRuntimeHost, *NativeAudioDevice, NativeFrameRegisterContext) {
	t.Helper()
	h := nativeRuntimeHostTest(t)
	if h.FollowerCode == nil {
		t.Fatal("native host follower CODE owner missing")
	}
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if err := h.Memory.BSS.Write32(0x14c, 0xc00000); err != nil {
		t.Fatal(err)
	}
	if done, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); err != nil || !done {
		t.Fatal(done, err)
	}
	device, _, err := h.InitializeAudio(&frame, 0)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeStartupHostFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	prelude := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10a10}}
	cb, err := h.StartupCallbacks(&frame, NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }, Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x3b64 {
			return NativeCommandFrameResult{}, fmt.Errorf("unexpected initial source child%x", call.Routine)
		}
		return NativeCommandFrameResult{}, nil
	}}}, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	if step, err := prelude.Advance(&rules, cb); err != nil || step.Complete {
		t.Fatal(step, err)
	}
	for _, p := range []struct {
		at    int
		value uint16
	}{{0xeb44, 4}, {0xeb42, 1}, {0xeb46, 0}, {0xeb22, uint16(land)}} {
		if err := h.Memory.BSS.Write16(p.at, p.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Memory.BSS.Write32(0xeb28, seed); err != nil {
		t.Fatal(err)
	}
	panel, err := DecodeNativeStartupPanelFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
	cb, err = h.StartupCallbacks(&frame, NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x1da0 {
			return NativeCommandFrameResult{}, fmt.Errorf("unexpected constructor child%x", call.Routine)
		}
		err := h.RestoreStartupPanel(&panel, call.Frame, call.A, func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil })
		return NativeCommandFrameResult{Complete: err == nil}, err
	}}}, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	startup := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10ad8, A: prelude.Startup.A}}
	if step, err := startup.Advance(&rules, cb); err != nil || !step.Complete {
		t.Fatal(step, err)
	}
	if err := h.RefreshWorldCaches(); err != nil {
		t.Fatal(err)
	}
	return h, device, frame
}

type nativeGameplayIntegrationFixture struct {
	Input struct {
		Name     string
		Land     int
		Seed     uint32
		Frames   int
		Commands []byte
		Interval int
		View     uint16
	}
	Startup        nativeGameplayIntegrationPoint
	Frames         []nativeGameplayIntegrationPoint
	ResultBoundary bool
	ErrorPC        uint32
}
type nativeGameplayIntegrationPoint struct {
	Tick                                   int
	Routine                                uint32
	D                                      [8]uint32
	A                                      [7]uint32
	BSSHash, CodeHash, ChipHash, HeapHash  string
	RNG                                    uint32
	TownFlag, TownProperty, MinimapVariant uint16
}

func TestNativeGameplayIntegrationAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_gameplay_integration_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []nativeGameplayIntegrationFixture
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			h, device, frame := nativeGameplayIntegrationStartup(t, f.Input.Land, f.Input.Seed)
			check := func(want nativeGameplayIntegrationPoint) {
				t.Helper()
				bss, err := h.Memory.SnapshotBSS()
				if err != nil {
					t.Fatal(err)
				}
				// Snapshot the physical allocation and each explicit callback-owned
				// span. This is a diagnostic copy; gameplay reads remain live.
				code := append([]byte(nil), h.Code.RawData()[:0x3fa2c]...)
				for _, span := range [][2]int{{0x3ea, 2}, {0x77a, 8}, {0xa2a, 12}, {0xeee0, 2}, {0x1117c, 4}, {0x124a0, 2}, {0x13350, 2}, {0x13550, 2}, {0x136e8, 100}, {0x18426, 8}, {0x185a8, 1330}} {
					for i := span[0]; i < span[0]+span[1]; i++ {
						code[i], err = h.Memory.Code.Read8(i)
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				if want.Tick == -1 {
					for i, value := range code {
						observed, e := h.Memory.Code.Read8(i)
						if e != nil || value != observed {
							t.Fatalf("diagnostic CODE snapshot missed live byte%x", i)
						}
					}
				}
				audioAllocation, err := h.Host.Span(0x700000, 0x1e0dc)
				if err != nil {
					t.Fatal(err)
				}
				backgroundAllocation, err := h.Host.Span(0x71e0dc, 0x7d00)
				if err != nil {
					t.Fatal(err)
				}
				heap := append(append([]byte(nil), audioAllocation...), backgroundAllocation...)
				d := frame.D
				if h.Session.world != nil {
					d = h.Session.Frame.D
				}
				if d != want.D {
					t.Fatalf("tick%d routine%x D differs: %08x/%08x", want.Tick, want.Routine, d, want.D)
				}
				if fileFrameHash(bss) != want.BSSHash {
					t.Fatalf("tick%d routine%x BSS differs", want.Tick, want.Routine)
				}
				if fileFrameHash(code) != want.CodeHash {
					if dir := os.Getenv("NATIVE_GAMEPLAY_DEBUG"); dir != "" && (want.Tick == -1 || want.Tick == 0 || want.Tick == 6 || want.Tick == 15 || want.Tick == 88) {
						_ = os.MkdirAll(dir, 0755)
						_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("go-%s-%d-%x-code.bin", f.Input.Name, want.Tick, want.Routine)), code, 0600)
					}
					t.Fatalf("tick%d routine%x CODE differs; caches13350=%d/%d 13550=%d/%d 124a0=%d/%d", want.Tick, want.Routine, h.Session.Followers.Pass.TownCacheFlag, want.TownFlag, h.Session.Followers.Town.Property13550, want.TownProperty, h.Session.Followers.Pass.MinimapVariant, want.MinimapVariant)
				}
				if fileFrameHash(h.Session.Presentation.Chip) != want.ChipHash {
					t.Errorf("tick%d routine%x chip differs", want.Tick, want.Routine)
				}
				if fileFrameHash(heap) != want.HeapHash {
					t.Errorf("tick%d routine%x heap differs", want.Tick, want.Routine)
				}
			}
			check(f.Startup)
			_ = h.Memory.BSS.Write16(0xf0c, f.Input.View)
			for side := 1; side <= 2; side++ {
				god := 0xe76a + side*314
				_ = h.Memory.BSS.Write32(god, 1000000)
				for i := 0; i < 6; i++ {
					_ = h.Memory.BSS.Write8(god+0x52+i, 255)
				}
			}
			if err := h.Session.BeginRaw(h.World, frame); err != nil {
				t.Fatal(err)
			}
			defer h.Session.finish(nil)
			resultBoundary := errors.New("actual381E result entry")
			callbacks := NativeFrameSessionCallbacks{Result: func(uint16, *NativeFrameRegisterContext) error { return resultBoundary }, Audio: NativeFrameAudioCallbacks{Command: device.Command}, DirectSound: func(cue uint16) error { return device.DirectCue(cue, &h.Session.Frame) }, Bitmap: h.Bitmap}
			h.Session.directSound = callbacks.DirectSound
			h.Session.bitmapResolver = h.Bitmap
			bindings := NativeCommandWorldBindings{WallRules: &h.Session.wallRules, WallPlacement: &h.Session.wallPlacement}
			index := 0
			for tick := 0; tick < f.Input.Frames; tick++ {
				interval := f.Input.Interval
				if interval == 0 {
					interval = 8
				}
				if tick%interval == 0 && tick/interval < len(f.Input.Commands) {
					packet := []byte{1, f.Input.Commands[tick/interval], 32, 32, 0, 0, 0, 0, 2, 0}
					for i, v := range packet {
						_ = h.Memory.BSS.Write8(0xeb56+i, v)
					}
					context := h.Session.Frame.CommandContext()
					if _, err := h.Session.commandRules.Execute(0xeb56, &context, h.World.nativeNormalCommandCallbacks(bindings)); err != nil {
						t.Fatal(err)
					}
					h.Session.Frame.SetCommandContext(context)
					check(f.Frames[index])
					index++
					_ = h.Memory.BSS.Write8(0xeb57, 0)
				}
				h.Session.Audio.Entries = h.Session.Image.AudioBank
				if err := h.ImageAudioCode.SetOwner(NativeAudioCodeOwner); err != nil {
					t.Fatal(err)
				}
				bitmap, err := h.Session.Presentation.BackBuffer()
				if err != nil {
					t.Fatal(err)
				}
				cb := h.Session.physicsCallbacks(callbacks, bitmap)
				cb.Swap = func(*NativeFrameRegisterContext) (bool, error) { return false, nil }
				h.Session.Pass = NativeFramePassState{}
				done, err := h.Session.Pass.TickFramePass(&h.Session.Frame, cb)
				atResult := errors.Is(err, resultBoundary)
				if err != nil && !atResult {
					t.Fatalf("tick%d physics: %v", tick, err)
				}
				if !atResult && (done || h.Session.Pass.Stage != NativeFrameSwap) {
					t.Fatal("source before-swap boundary lost")
				}
				check(f.Frames[index])
				index++
				if atResult {
					if !f.ResultBoundary {
						t.Fatal("unexpected actual result entry")
					}
					break
				}
				h.Session.Image.AudioBank = h.Session.Audio.Entries
				if err := h.ImageAudioCode.SetOwner(NativeImageCodeOwner); err != nil {
					t.Fatal(err)
				}
				if _, err := h.Session.Presentation.Swap(&h.Session.Frame); err != nil {
					t.Fatal(err)
				}
				clock, err := h.Memory.BSS.Read32(0xf40)
				if err != nil {
					t.Fatal(err)
				}
				_ = h.Memory.BSS.Write32(0xf40, clock+1)
			}
			if index != len(f.Frames) || f.ErrorPC != 0 {
				t.Fatalf("source frame count/fault: %d/%d pc%x", index, len(f.Frames), f.ErrorPC)
			}
		})
	}
}
