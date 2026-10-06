package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type runtimeFileBrowserInput struct {
	LoadLand                     uint16
	Name, Kind, Drawer, Filename string
	Save                         bool
	Length                       int
	Alternate, DialogFlag        uint16
	Bootstrap                    startupCampaignInput
	D                            [8]uint32
	Events                       []fileFrameEvent
}
type runtimeFileBrowserFixture struct {
	Input  runtimeFileBrowserInput
	Frames []struct {
		runtimeFilesSnapshot
		PC  int
		CCR uint16
	}
	FinalFiles []struct {
		Name, Hash string
		Length     int
	}
}

func TestNativeRuntimeFileBrowserWholeAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/native_runtime_file_browser_whole_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []runtimeFileBrowserFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 28 {
		t.Fatalf("whole browser corpus changed%d", len(corpus.Cases))
	}
	for _, f := range corpus.Cases {
		for _, async := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, async), func(t *testing.T) {
				h := nativeRuntimeHostTest(t)
				device := runtimeFilesPrelude(t, h)
				runtimeFilesCampaign(t, h, device)
				payload, e := h.Memory.SnapshotBSS()
				if e != nil {
					t.Fatal(e)
				}
				payload = append([]byte(nil), payload[NativeGAMStart:NativeGAMEnd]...)
				for _, at := range []int{0xf32, 0xf36} {
					v, e := h.Memory.BSS.Read32(at)
					if e != nil {
						t.Fatal(e)
					}
					if v != 0 {
						v -= h.Memory.BSSBase + 0x76c0
						offset := at - NativeGAMStart
						payload[offset], payload[offset+1], payload[offset+2], payload[offset+3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
					}
				}
				root := filepath.Join(t.TempDir(), f.Input.Name)
				if e = os.Mkdir(root, 0700); e != nil {
					t.Fatal(e)
				}
				in := f.Input
				if in.Kind == "load" || in.Kind == "load-short" {
					b := append([]byte(nil), payload...)
					at := 0xeb22 - NativeGAMStart
					b[at], b[at+1] = byte(in.LoadLand>>8), byte(in.LoadLand)
					if in.Kind == "load-short" {
						b = b[:in.Length]
					}
					if e = os.WriteFile(filepath.Join(root, "RAW.GAM"), b, 0600); e != nil {
						t.Fatal(e)
					}
				}
				if in.Kind == "overwrite" || in.Kind == "overwrite-cancel" {
					if e = os.WriteFile(filepath.Join(root, "RAW.GAM"), []byte{1, 2, 3, 4}, 0600); e != nil {
						t.Fatal(e)
					}
				}
				if in.Kind == "list" {
					for i := 0; i < 20; i++ {
						if e = os.WriteFile(filepath.Join(root, fmt.Sprintf("FILE%02d.GAM", i)), payload, 0600); e != nil {
							t.Fatal(e)
						}
					}
					if e = os.WriteFile(filepath.Join(root, "lower.gam"), []byte{1, 2, 3}, 0600); e != nil {
						t.Fatal(e)
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
				_ = h.Memory.Code.Write16(0x4468, 0)
				if in.Save {
					_ = h.Memory.Code.Write16(0x4468, 1)
				}
				_ = h.Memory.Code.Write16(0x3f90, in.Alternate)
				_ = h.Memory.BSS.Write16(0x3b0, in.DialogFlag)
				for _, text := range []struct {
					At   int
					Text string
				}{{0x4440, in.Drawer}, {0x4416, in.Filename}} {
					for i, v := range append([]byte(text.Text), 0) {
						_ = h.Memory.Code.Write8(text.At+i, v)
					}
				}
				frame := NativeFrameRegisterContext{D: in.D, AddressBase: h.Memory.BSSBase}
				a := [7]NativeRequesterAddress{}
				for i := range a {
					a[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
				}
				state := NativeRuntimeFileBrowserState{}
				phase := uint32(0)
				rules, e := DecodeNativeRuntimeFileBrowserRules(h.Bundle.Executable)
				if e != nil {
					t.Fatal(e)
				}
				cb := NativeRuntimeFileBrowserCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}
				complete := false
				for index, want := range f.Frames {
					deadline := time.Now().Add(10 * time.Second)
					var result NativeCommandFrameResult
					for {
						result, e = state.AdvanceChild(h, store, &rules, NativeStartupResetFrameCall{Routine: 0x3f92, Frame: &frame, A: &a}, &phase, cb)
						if e != nil {
							t.Fatal(e)
						}
						if result.Complete {
							break
						}
						if state.Browser != nil && ((!state.Browser.ChildActive) || (state.Browser.ChildRoutine == 0x102e4) || (state.Files.Overwrite != nil)) {
							break
						}
						if time.Now().After(deadline) {
							t.Fatal("actual browser filesystem continuation stalled")
						}
						time.Sleep(time.Millisecond)
					}
					complete = result.Complete
					if frame.D != want.D {
						t.Fatalf("stage%d D got%08x want%08x", index, frame.D, want.D)
					}
					for i := range a {
						if a[i].Address != want.A[i] {
							t.Fatalf("stage%d A%d got%x want%x", index, i, a[i].Address, want.A[i])
						}
					}
					if result.Complete && (result.Zero != (want.CCR&4 != 0)) {
						t.Fatal("actual browser terminalCCR differs")
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
					if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(h.Session.Presentation.Chip) != want.ChipHash || fileFrameHash(h.Session.Presentation.PointerData[:15260]) != want.PointerHash {
						t.Fatalf("stage%d backing BSS%v CODE%v chip%v pointer%v", index, fileFrameHash(raw) == want.BSSHash, fileFrameHash(code) == want.CodeHash, fileFrameHash(h.Session.Presentation.Chip) == want.ChipHash, fileFrameHash(h.Session.Presentation.PointerData[:15260]) == want.PointerHash)
					}
					if index < len(in.Events) {
						event := in.Events[index]
						if event.Action != 0 {
							runtimeFilesClick(t, h, &frame, event.Action)
						} else if event.VBlank {
							p := h.Session.Presentation
							if _, e = p.VBlank(NativeMouseSample{CounterX: uint8(p.Input.Mouse.CounterX), CounterY: uint8(p.Input.Mouse.CounterY)}, h.Memory.BSS, &frame); e != nil {
								t.Fatal(e)
							}
						}
						for _, wire := range event.Keys {
							if e = h.Session.Presentation.Input.KeyboardInterrupt(wire); e != nil {
								t.Fatal(e)
							}
						}
					}
				}
				wantComplete := f.Frames[len(f.Frames)-1].PC == 0
				if complete != wantComplete {
					t.Fatal("source browser completion boundary differs")
				}
				for _, file := range f.FinalFiles {
					b, e := os.ReadFile(filepath.Join(root, file.Name))
					if e != nil {
						t.Fatal(e)
					}
					if len(b) != file.Length || fileFrameHash(b) != file.Hash {
						t.Fatalf("actual filesystem%s differs", file.Name)
					}
				}
				if complete && state.Files.RefreshPending {
					before, e := h.Memory.SnapshotBSS()
					if e != nil {
						t.Fatal(e)
					}
					if e = state.Files.RefreshLoadedViews(h); e != nil {
						t.Fatal(e)
					}
					after, e := h.Memory.SnapshotBSS()
					if e != nil {
						t.Fatal(e)
					}
					if !bytes.Equal(before, after) {
						t.Fatal("post-browser hydration rewrote source bytes")
					}
				}
			})
		}
	}
}
