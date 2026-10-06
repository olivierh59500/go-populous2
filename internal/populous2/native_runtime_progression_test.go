package populous2

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

type nativeProgressionControllerFixture struct {
	Input struct {
		Name         string
		World, Score uint16
		Steps        int
		ClickAt      int
		EmptyQueue   bool
		D            [8]uint32
	}
	Frames []struct {
		PC                                       int
		D                                        [8]uint32
		A                                        [7]uint32
		BSSHash, CodeHash, ChipHash, PointerHash string
	}
}

func TestNativeRuntimeProgressionAgainstOriginalParentCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_runtime_progression_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []nativeProgressionControllerFixture
	}
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 16 {
		t.Fatal("native progression corpus missing", err)
	}
	snapshots := 0
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) != f.Input.Steps+1 || f.Frames[len(f.Frames)-1].PC != 0xb740 {
				t.Fatal("native award trace did not reach its actual deity child")
			}
			h := nativeRuntimeHostTest(t)
			device := runtimeFilesPrelude(t, h)
			for _, patch := range []nativeHeroPatch{{0xeb46, 2, uint32(f.Input.World)}, {0xdd0, 2, uint32(f.Input.Score)}, {0xeb42, 2, 1}, {0xeb24, 4, 0x12345678}} {
				renderFramePatch(h.Memory.BSS, patch)
			}
			if !f.Input.EmptyQueue {
				for _, patch := range []nativeHeroPatch{{0xeb6e, 2, 4}, {0xeb70, 2, 0x0820}, {0xeb72, 2, 0x1821}} {
					renderFramePatch(h.Memory.BSS, patch)
				}
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: h.Memory.BSSBase}
			state := NativeRuntimeProgressionState{}
			for i := range state.A {
				state.A[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
			}
			callbacks := NativeRuntimeProgressionCallbacks{
				Audio:     NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue},
				Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil },
				Child: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
					if call.Routine == 0xb740 {
						return NativeCommandFrameResult{}, nil
					}
					err := RunNativeProgressionChild(call.Routine, h, call.Frame, call.A)
					return NativeCommandFrameResult{Complete: err == nil}, err
				},
			}
			for index, want := range f.Frames {
				snapshots++
				step, err := state.Advance(h, &frame, callbacks)
				if err != nil {
					t.Fatal(index, err)
				}
				if frame.D != want.D {
					t.Fatalf("stage%d D got%08x want%08x", index, frame.D, want.D)
				}
				for i := range state.A {
					if state.A[i].Address != want.A[i] {
						t.Fatalf("stage%d A%d got%x want%x", index, i, state.A[i].Address, want.A[i])
					}
				}
				if step.Complete != (want.PC == 0) {
					t.Fatalf("stage%d completion differs sourcePC%x", index, want.PC)
				}
				raw, err := h.Memory.SnapshotBSS()
				if err != nil {
					t.Fatal(err)
				}
				code := make([]byte, 0x3fa2c)
				for i := range code {
					code[i], err = h.Memory.Code.Read8(i)
					if err != nil {
						t.Fatal(err)
					}
				}
				if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(h.Session.Presentation.Chip) != want.ChipHash || fileFrameHash(h.Session.Presentation.PointerData[:15260]) != want.PointerHash {
					t.Fatalf("stage%d backing differs BSS%v CODE%v chip%v pointer%v", index, fileFrameHash(raw) == want.BSSHash, fileFrameHash(code) == want.CodeHash, fileFrameHash(h.Session.Presentation.Chip) == want.ChipHash, fileFrameHash(h.Session.Presentation.PointerData[:15260]) == want.PointerHash)
				}
				if index < f.Input.Steps {
					if _, err = h.Session.Presentation.VBlank(NativeMouseSample{Left: index == f.Input.ClickAt}, h.Memory.BSS, &frame); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
	if snapshots != 1220 {
		t.Fatal("native progression snapshot coverage changed", snapshots)
	}
}

func TestNativeRuntimeProgressionRetainsActualEnding(t *testing.T) {
	var hosts [2]*NativeRuntimeHost
	var frames [2]NativeFrameRegisterContext
	for i := range hosts {
		hosts[i] = nativeRuntimeHostTest(t)
		runtimeFilesPrelude(t, hosts[i])
		if err := hosts[i].Memory.BSS.Write16(0xeb46, 1000); err != nil {
			t.Fatal(err)
		}
		frames[i] = NativeFrameRegisterContext{D: [8]uint32{0x12345678, 0x23456789, 0x3456789a, 0x456789ab, 0x56789abc, 0x6789abcd, 0x789abcde, 0x89abcdef}, AddressBase: hosts[i].Memory.BSSBase}
	}
	var a [7]NativeRequesterAddress
	for i := range a {
		a[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
	}
	parent := NativeRuntimeProgressionState{A: a}
	ending := NativeRuntimeEndingState{A: a}
	for tick := 0; tick < 24; tick++ {
		step, err := parent.Advance(hosts[0], &frames[0], NativeRuntimeProgressionCallbacks{})
		if err != nil || step.Complete || parent.PC != 0xb24e || parent.Ending == nil {
			t.Fatal("progression did not retain the real ending child", step, err)
		}
		ref, err := ending.Advance(hosts[1], &frames[1], NativeRuntimeProgressionCallbacks{})
		if err != nil || ref.Complete {
			t.Fatal(ref, err)
		}
		if frames[0].D != frames[1].D || parent.Ending.A != ending.A {
			t.Fatal("parent altered the source ending registers", tick)
		}
		raw0, err := hosts[0].Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		raw1, err := hosts[1].Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw0, raw1) || !bytes.Equal(hosts[0].Code.RawData(), hosts[1].Code.RawData()) || !bytes.Equal(hosts[0].Session.Presentation.Chip, hosts[1].Session.Presentation.Chip) {
			t.Fatal("progression detached the CPU-verified ending state", tick)
		}
		for i, h := range hosts {
			if _, err := h.Session.Presentation.VBlank(NativeMouseSample{}, h.Memory.BSS, &frames[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestNativeRuntimeProgressionReturnsThroughActualDeityEditor(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	device := runtimeFilesPrelude(t, h)
	for _, patch := range []nativeHeroPatch{{0xeb46, 2, 32}, {0xdd0, 2, 13007}, {0xeb42, 2, 1}, {0xeb24, 4, 0x12345678}, {0xeb6e, 2, 4}, {0xeb70, 2, 0x0820}, {0xeb72, 2, 0x1821}} {
		renderFramePatch(h.Memory.BSS, patch)
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	var deity NativeRuntimeDeity
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
	campaign := NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}}
	callbacks := NativeRuntimeProgressionCallbacks{Audio: NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}
	callbacks.Child = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		if call.Routine == 0xb740 {
			return deity.AdvanceChild(h, call, phase, campaign)
		}
		err := RunNativeProgressionChild(call.Routine, h, call.Frame, call.A)
		return NativeCommandFrameResult{Complete: err == nil}, err
	}
	state := NativeRuntimeProgressionState{}
	blank := func(left bool) {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY), Left: left}, h.Memory.BSS, &frame); err != nil {
			t.Fatal(err)
		}
	}
	var complete bool
	for tick := 0; tick < 100; tick++ {
		step, err := state.Advance(h, &frame, callbacks)
		if err != nil {
			t.Fatal(tick, err)
		}
		if step.Complete {
			t.Fatal("award skipped the actual deity child")
		}
		blank(tick == 51)
	}
	if deity.State == nil || state.PC != 0xb6ae {
		t.Fatal("award did not retain the actual deity editor", state.PC)
	}
	nativeRuntimeClickAction(t, h, &frame, 66)
	for tick := 0; tick < 40 && !complete; tick++ {
		step, err := state.Advance(h, &frame, callbacks)
		if err != nil {
			t.Fatal(err)
		}
		complete = step.Complete
		if !complete {
			blank(false)
		}
	}
	if !complete || deity.State != nil || !state.Finished {
		t.Fatal("original deity proceed did not return from B244")
	}
	if len(h.Files.handles) != 0 {
		t.Fatal("award/deity resource handles leaked")
	}
}
