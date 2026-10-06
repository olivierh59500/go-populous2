package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type runtimeFilesInput struct {
	Name, Kind string
	Length     int
	Land       uint16
	D          [8]uint32
}
type runtimeFilesSnapshot struct {
	D                                        [8]uint32
	A                                        [7]uint32
	BSSHash, CodeHash, ChipHash, PointerHash string
	Library                                  []struct {
		Vector int
		D      [8]uint32
		A      [7]uint32
		Value  uint32
	}
	IO []struct {
		Vector int
		Name   string
		D      [8]uint32
		Value  uint32
	}
}
type runtimeFilesFixture struct {
	Input      runtimeFilesInput
	Frames     []runtimeFilesSnapshot
	FinalFiles []struct {
		Name, Hash string
		Length     int
	}
}

// This prelude runs the actual resource allocation, initialized audio and
// ICON preparation. Its still-pending $3b64 is a separate source operation.
func runtimeFilesPrelude(t *testing.T, h *NativeRuntimeHost) *NativeAudioDevice {
	t.Helper()
	_ = h.Memory.Code.Write16(0x3ea, 0)
	h.Session.Presentation.InterruptChain = false
	_ = h.Memory.BSS.Write32(0x14c, 0xc00000)
	if _, e := h.InitializePresentation(NativeMouseSample{}); e != nil {
		t.Fatal(e)
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if done, e := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); e != nil || !done {
		t.Fatal(done, e)
	}
	device, _, e := h.InitializeAudio(&frame, 0)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := DecodeNativeStartupHostFrameRules(h.Bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	state := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10a10}}
	cb, e := h.StartupCallbacks(&frame, NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }, Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x3b64 {
			return NativeCommandFrameResult{}, fmt.Errorf("unexpected prelude child%x", call.Routine)
		}
		return NativeCommandFrameResult{}, nil
	}}}, NativeErrorFrameCallbacks{})
	if e != nil {
		t.Fatal(e)
	}
	step, e := state.Advance(&rules, cb)
	if e != nil || step.Complete || !step.Waiting {
		t.Fatal(step, e)
	}
	return device
}

func runtimeFilesClick(t *testing.T, h *NativeRuntimeHost, frame *NativeFrameRegisterContext, action int) {
	t.Helper()
	code := h.Memory.Code
	start, e := code.Read16(0xab4e)
	if e != nil {
		t.Fatal(e)
	}
	width, e := code.Read16(0xab54)
	if e != nil {
		t.Fatal(e)
	}
	column, _ := code.Read16(0xab50)
	row, _ := code.Read16(0xab52)
	count, x, y := 0, 0, 0
	found := false
	for i := 0; i < 2048; i++ {
		v, e := code.Read8(0xab4e + int(int16(start)) + i)
		if e != nil {
			t.Fatal(e)
		}
		if v == 0 {
			break
		}
		if int8(v) > 0x5a {
			enabled, e := code.Read8(0x4e92 + int(v) - 0x5b)
			if e != nil {
				t.Fatal(e)
			}
			if int8(enabled) > 0 {
				count += 2
				if count == action {
					x = (int(column) + i%(int(width)+1)) * 8
					y = int(row) + i/(int(width)+1)*8
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Fatalf("native overwrite button%d missing", action)
	}
	p := h.Session.Presentation
	for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
		dx, dy := x*2-int(p.Input.Mouse.PositionX), y*2-int(p.Input.Mouse.PositionY)
		dx, dy = max(-100, min(100, dx)), max(-100, min(100, dy))
		if _, e := p.VBlank(NativeMouseSample{CounterX: uint8(int(p.Input.Mouse.CounterX) + dx), CounterY: uint8(int(p.Input.Mouse.CounterY) + dy)}, h.Memory.BSS, frame); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := p.VBlank(NativeMouseSample{CounterX: uint8(p.Input.Mouse.CounterX), CounterY: uint8(p.Input.Mouse.CounterY), Left: true}, h.Memory.BSS, frame); e != nil {
		t.Fatal(e)
	}
}

func TestNativeRuntimeFilesAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/native_runtime_files_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []runtimeFilesFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 30 {
		t.Fatalf("runtime DOS corpus changed %d", len(corpus.Cases))
	}
	for _, want := range corpus.Cases {
		for _, async := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", want.Input.Name, async), func(t *testing.T) {
				h := nativeRuntimeHostTest(t)
				device := runtimeFilesPrelude(t, h)
				root := filepath.Join(t.TempDir(), want.Input.Name)
				if e = os.Mkdir(root, 0700); e != nil {
					t.Fatal(e)
				}
				in := want.Input
				if in.Kind == "load" {
					b := make([]byte, in.Length)
					for i := range b {
						b[i] = byte(i*17 + 93)
					}
					if len(b) > 0xeb23-NativeGAMStart {
						b[0xeb22-NativeGAMStart], b[0xeb23-NativeGAMStart] = byte(in.Land>>8), byte(in.Land)
					}
					if e = os.WriteFile(filepath.Join(root, "RAW.GAM"), b, 0600); e != nil {
						t.Fatal(e)
					}
				}
				if in.Kind == "overwrite" || in.Kind == "cancel" {
					if e = os.WriteFile(filepath.Join(root, "RAW.GAM"), []byte{1, 2, 3, 4}, 0600); e != nil {
						t.Fatal(e)
					}
				}
				if in.Kind == "list" {
					for _, name := range []string{"zeta.GAM", "lower.gam", "Mixed.GAM", "NOT.GAM.old", "abc", "é.GAM"} {
						if e = os.WriteFile(filepath.Join(root, name), []byte{1, 2, 3}, 0600); e != nil {
							t.Fatal(e)
						}
					}
					if e = os.Mkdir(filepath.Join(root, "DIR.GAM"), 0700); e != nil {
						t.Fatal(e)
					}
				}
				store, e := NewNativeRuntimeFileStore(root, []string{"SAVES"}, async)
				if e != nil {
					t.Fatal(e)
				}
				defer store.Close()
				for at := NativeGAMStart; at < NativeGAMEnd; at++ {
					if e = h.Memory.BSS.Write8(at, byte(at*37+11)); e != nil {
						t.Fatal(e)
					}
				}
				_ = h.Memory.BSS.Write16(0xeb22, 0)
				_ = h.Memory.BSS.Write32(0xf32, 0x1240)
				_ = h.Memory.BSS.Write32(0xf36, 0x82)
				_ = h.Memory.Code.Write16(0xa2a, 0x120)
				routine, path, pathAt := 0x19afc, "SAVES:RAW.GAM", 0x43c0
				if in.Kind == "save-denied" {
					path = "OTHER:RAW.GAM"
				}
				if in.Kind == "load" || in.Kind == "load-missing" {
					routine = 0x19c1c
				}
				if strings.HasPrefix(in.Kind, "list") {
					routine, path, pathAt = 0x19936, "SAVES:", 0x4440
				}
				for i, v := range append([]byte(path), 0) {
					_ = h.Memory.Code.Write8(pathAt+i, v)
				}
				frame := NativeFrameRegisterContext{D: in.D, AddressBase: h.Memory.BSSBase}
				a := [7]NativeRequesterAddress{}
				for i := range a {
					a[i] = NativeRequesterAddress{Address: h.Memory.CodeBase + uint32(0x200+i*4), Code: true}
				}
				a[0] = NativeRequesterAddress{Address: h.Memory.CodeBase + uint32(pathAt), Code: true}
				a[1] = NativeRequesterAddress{Address: h.Memory.BSSBase + 0x3be}
				a[4] = NativeRequesterAddress{Address: h.Memory.BSSBase + 0x5f4}
				state := NativeRuntimeFilesState{}
				phase := uint32(0)
				cb := NativeRuntimeFilesCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}
				for index, snapshot := range want.Frames {
					deadline := time.Now().Add(10 * time.Second)
					var result NativeCommandFrameResult
					for {
						result, e = state.AdvanceChild(h, store, NativeStartupResetFrameCall{Routine: routine, Frame: &frame, A: &a}, &phase, cb)
						if e != nil {
							t.Fatal(e)
						}
						if result.Complete || state.Overwrite != nil {
							break
						}
						if time.Now().After(deadline) {
							t.Fatal("retained filesystem operation stalled")
						}
						time.Sleep(time.Millisecond)
					}
					if index == len(want.Frames)-1 && !result.Complete {
						t.Fatal("actual native DOS did not return")
					}
					if frame.D != snapshot.D {
						t.Fatalf("snapshot%d D got%08x want%08x", index, frame.D, snapshot.D)
					}
					for i := range a {
						if a[i].Address != snapshot.A[i] {
							t.Fatalf("snapshot%d A%d got%x want%x", index, i, a[i].Address, snapshot.A[i])
						}
					}
					raw, e := h.Memory.SnapshotBSS()
					if e != nil {
						t.Fatal(e)
					}
					code := make([]byte, 0x3fa2c)
					for i := range code {
						code[i], e = h.Memory.Code.Read8(i)
						if e != nil {
							t.Fatal(e)
						}
					}
					if fileFrameHash(raw) != snapshot.BSSHash || fileFrameHash(code) != snapshot.CodeHash || fileFrameHash(h.Session.Presentation.Chip) != snapshot.ChipHash || fileFrameHash(h.Session.Presentation.PointerData[:15260]) != snapshot.PointerHash {
						t.Fatalf("snapshot%d backing BSS%v CODE%v chip%v pointer%v", index, fileFrameHash(raw) == snapshot.BSSHash, fileFrameHash(code) == snapshot.CodeHash, fileFrameHash(h.Session.Presentation.Chip) == snapshot.ChipHash, fileFrameHash(h.Session.Presentation.PointerData[:15260]) == snapshot.PointerHash)
					}
					if index < len(want.Frames)-1 {
						action := 2
						if in.Kind == "cancel" {
							action = 4
						}
						runtimeFilesClick(t, h, &frame, action)
					}
				}
				for _, f := range want.FinalFiles {
					b, e := os.ReadFile(filepath.Join(root, f.Name))
					if e != nil {
						t.Fatal(e)
					}
					if len(b) != f.Length || fileFrameHash(b) != f.Hash {
						t.Fatalf("actual native file%s differs", f.Name)
					}
				}
				if len(store.DOS.handles) != 0 || len(store.DOS.pending) != 0 {
					t.Fatal("actual DOS handle/future retained after return")
				}
				if state.RefreshPending != (in.Kind == "load" && in.Length > 0) {
					t.Fatal("load hydration request differs from actual transferred bytes")
				}
				// Raw test patterns are intentionally not typed actors; no hydration runs.
			})
		}
	}
}
