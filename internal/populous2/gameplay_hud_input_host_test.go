package populous2

import (
	"encoding/json"
	"fmt"
	embedded "go-populous2/assets"
	"os"
	"strings"
	"testing"
)

type gameplayHUDHostInput struct {
	startupCampaignInput
	Power   int
	HUDMode string
}
type gameplayHUDHostFixture struct {
	Input       gameplayHUDHostInput
	Prelude     startupCampaignSnapshot
	Frames, HUD []startupCampaignSnapshot
}

func TestNativeGameplayHUDHostAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/gameplay_hud_input_host_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []gameplayHUDHostFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 18 {
		t.Fatal("whole campaign startup corpus changed")
	}
	files, e := embedded.DataFS()
	if e != nil {
		t.Fatal(e)
	}
	bundle := testBundle(t)
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			low := make([]byte, 256)
			low[4] = 0
			low[5] = 0xd0
			tracked := &startupCampaignTrackedFS{FS: files}
			h, e := NewNativeRuntimeHost(bundle, tracked, NativeRuntimeHostConfig{HunkBases: []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000}, Regions: []NativeHostRegion{{Name: "actual configured Exec low vector", Base: 0, Bytes: low}}, AllocationStart: 0x700000, AllocationLimit: 0x800000})
			if e != nil {
				t.Fatal(e)
			}
			defer h.Close()
			_ = h.Memory.Code.Write16(0x3ea, 0)
			h.Session.Presentation.InterruptChain = false
			_ = h.Memory.BSS.Write32(0x14c, 0xc00000)
			if _, e = h.InitializePresentation(NativeMouseSample{}); e != nil {
				t.Fatal(e)
			}
			frame := NativeFrameRegisterContext{AddressBase: 0x200000}
			if done, e := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); e != nil || !done {
				t.Fatal(done, e)
			}
			device, _, e := h.InitializeAudio(&frame, 0)
			if e != nil {
				t.Fatal(e)
			}
			rules, e := DecodeNativeStartupCampaignHostRules(bundle.Executable)
			if e != nil {
				t.Fatal(e)
			}
			prelude := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10a10}}
			preCB, e := h.StartupCallbacks(&frame, NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }, Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
				if call.Routine != 0x3b64 {
					return NativeCommandFrameResult{}, fmt.Errorf("initial sourceprelude reached unexpectedchild%x", call.Routine)
				}
				return NativeCommandFrameResult{}, nil
			}}}, NativeErrorFrameCallbacks{})
			if e != nil {
				t.Fatal(e)
			}
			step, e := prelude.Advance(&rules.Startup, preCB)
			if e != nil || step.Complete || !step.Waiting {
				t.Fatal("realprelude didnotretainactualmenu", step, e)
			}
			checkBacking := func(want startupCampaignSnapshot) {
				t.Helper()
				raw, e := h.Memory.SnapshotBSS()
				if e != nil {
					t.Fatal(e)
				}
				code := make([]byte, 0x3fa2c)
				for i := range code {
					v, err := h.Memory.Code.Read8(i)
					if err != nil {
						t.Fatal(err)
					}
					code[i] = v
				}

				if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(h.Session.Presentation.Chip) != want.ChipHash {
					t.Fatalf("complete backing BSS%v CODE%v chip%v", fileFrameHash(raw) == want.BSSHash, fileFrameHash(code) == want.CodeHash, fileFrameHash(h.Session.Presentation.Chip) == want.ChipHash)
				}
			}
			if frame.D != f.Prelude.D {
				t.Fatalf("preludeD got%08x want%08x", frame.D, f.Prelude.D)
			}
			for i, a := range prelude.Startup.A {
				if a.Address != f.Prelude.A[i] {
					t.Fatalf("preludeA%d got%x want%x", i, a.Address, f.Prelude.A[i])
				}
			}
			checkBacking(f.Prelude)
			for _, v := range [][2]uint16{{0xeb44, 2}, {0xeb42, f.Input.Profile}, {0xeb46, f.Input.World}} {
				_ = h.Memory.BSS.Write16(int(v[0]), v[1])
			}
			frame.D = f.Input.D
			state := NativeStartupCampaignHostState{}
			for i := range state.Startup.Startup.A {
				state.Startup.Startup.A[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
			}
			audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
			supplied := NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
				if call.Routine != 0x3b64 {
					return NativeCommandFrameResult{}, fmt.Errorf("unexpectedcampaignconstructorchild%x", call.Routine)
				}
				return NativeCommandFrameResult{}, nil
			}}}, Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: func(offset uint16, c *NativeFrameRegisterContext) error { return device.DirectCue(offset, c) }}}}}, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}
			expected := 0
			ioIndex := 0
			check := func() {
				step, e := state.Advance(h, &rules, &frame, supplied)
				if e != nil {
					t.Fatal(e)
				}
				want := f.Frames[expected]
				for i, operation := range want.IO {
					if operation.Kind == "open" {
						if ioIndex >= len(tracked.Names) || tracked.Names[ioIndex] != strings.ToUpper(operation.Name) {
							t.Fatal("actualencodedresourcefilename differs")
						}
						if i+1 >= len(want.IO) || want.IO[i+1].Kind != "read" || tracked.Counts[ioIndex] != int(want.IO[i+1].Value) {
							t.Fatal("actualencodedresourcetransfercount differs")
						}
						ioIndex++
					}
				}

				pc := uint32(step.PC)
				if state.Startup.Startup.ChildActive && state.Startup.Startup.ChildRoutine == 0x3b64 {
					pc = 0x3b64
				}
				addresses := state.Startup.Startup.A
				if state.Selection != nil {
					addresses = state.Selection.A
					pc = uint32(state.Selection.PC)
					if state.Selection.ChildActive && state.Selection.ChildRoutine == 0x102e4 {
						pc = 0x786
					}
				}
				if step.Complete {
					pc = 0
				}
				if int(pc) != want.PC && !(want.PC == 0x786 && pc == 0x3d9a) {
					t.Fatalf("sourcePC%x want%x", pc, want.PC)
				}
				if frame.D != want.D {
					t.Fatalf("frame%d allDgot%08x want%08x", expected, frame.D, want.D)
				}
				for i, a := range addresses {
					if a.Address != want.A[i] {
						t.Fatalf("frame%d A%d got%x want%x", expected, i, a.Address, want.A[i])
					}
				}
				if step.Complete && (!step.FlagsKnown || step.Zero != (want.CCR&4 != 0) || step.Negative != (want.CCR&8 != 0)) {
					t.Fatal("actualterminalCCR differs")
				}
				checkBacking(want)
				if fileFrameHash(h.Session.Presentation.PointerData[:15260]) != want.PointerHash {
					t.Fatal("actualpointerRAM differs")
				}
			}
			check()
			irq := func(x, y uint8, left bool) {
				if _, e := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, h.Memory.BSS, &frame); e != nil {
					t.Fatal(e)
				}
			}
			for i, ev := range f.Input.Events {
				expected = i + 1
				if ev.Action != 0 {
					start, _ := h.Memory.Code.Read16(0xab4e)
					width, _ := h.Memory.Code.Read16(0xab54)
					col, _ := h.Memory.Code.Read16(0xab50)
					row, _ := h.Memory.Code.Read16(0xab52)
					count, x, y := 0, 0, 0
					found := false
					for j := 0; ; j++ {
						v, e := h.Memory.Code.Read8(0xab4e + int(int16(start)) + j)
						if e != nil || v == 0 {
							break
						}
						if int8(v) > 0x5a {
							m, _ := h.Memory.Code.Read8(0x4e92 + int(v-0x5b))
							if int8(m) > 0 {
								count += 2
								if count == ev.Action {
									x = (int(col) + j%(int(width)+1)) * 8
									y = int(row) + j/(int(width)+1)*8
									found = true
									break
								}
							}
						}
					}
					if !found {
						t.Fatal("actualmenuaction missing")
					}
					p := &h.Session.Presentation.Input
					for int(p.Mouse.PositionX) != x*2 || int(p.Mouse.PositionY) != y*2 {
						dx := max(-100, min(100, x*2-int(p.Mouse.PositionX)))
						dy := max(-100, min(100, y*2-int(p.Mouse.PositionY)))
						irq(uint8(int(p.Mouse.CounterX)+dx), uint8(int(p.Mouse.CounterY)+dy), false)
					}
					irq(uint8(p.Mouse.CounterX), uint8(p.Mouse.CounterY), true)
				} else if ev.VBlank {
					p := &h.Session.Presentation.Input
					irq(uint8(p.Mouse.CounterX), uint8(p.Mouse.CounterY), false)
				}
				check()
			}
			wantComplete := f.Frames[len(f.Frames)-1].PC == 0
			if state.Startup.Startup.Finished != wantComplete || len(h.Files.handles) != 0 {
				t.Fatal("campaignstartupdidnotcompleteorretainedresourcehandles")
			}
			hudRules, _ := DecodeNativeGameplayHUDInputRules(bundle.Executable)
			childRules, _ := DecodeNativeCampaignSelectionChildrenRules(bundle.Executable)
			panel, _ := DecodeNativeStartupPanelFrameRules(bundle.Executable)
			frame.D = f.Input.D
			frame.Word(2, uint16(f.Input.Power%6))
			a := [7]NativeRequesterAddress{}
			for i := range a {
				a[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
			}
			for _, p := range []nativeHeroPatch{{0xf3a, 2, uint32(f.Input.Power / 6 * 2)}, {0x140, 2, 1}, {0x142, 2, 0}, {0x146, 2, 0}, {0xad, 1, 0}, {0x73, 1, 0}, {0xe8a4, 4, 0xffffffff}, {0xe914 + f.Input.Power, 1, 1}} {
				renderFramePatch(h.Memory.BSS, p)
			}
			if f.Input.HUDMode == "help" {
				_ = h.Memory.BSS.Write8(0x73, 1)
			}
			if f.Input.HUDMode == "toggle" {
				_ = h.Memory.BSS.Write8(0xad, 1)
			}
			hudHost := NativeGameplayHUDHost{}
			phase := uint32(0)
			hudSupplied := NativeGameplayHUDHostCallbacks{Panel: panel, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }, Audio: NativeRuntimeAudioOperations{Command: device.Command, DirectCue: device.DirectCue}, Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: func(offset uint16, c *NativeFrameRegisterContext) error { return device.DirectCue(offset, c) }}}}}}
			checkHUD := func(want startupCampaignSnapshot) {
				result, e := hudHost.Call(h, &hudRules, &childRules, hudSupplied, NativeStartupResetFrameCall{Routine: 0x2472, Frame: &frame, A: &a}, &phase)
				if e != nil {
					t.Fatal(e)
				}
				pc := 0
				if hudHost.Input != nil {
					pc = hudHost.Input.PC
					if hudHost.Input.ChildRoutine == 0x517a {
						pc = hudHost.Children.Help.PC
					}
				}
				if result.Complete {
					pc = 0
				}
				if pc != want.PC && !(want.PC == 0x786 && pc == 0x51dc) {
					t.Fatalf("HUDPC%x want%x", pc, want.PC)
				}
				if frame.D != want.D {
					t.Fatalf("HUDallDgot%08x want%08x", frame.D, want.D)
				}
				for i, v := range a {
					if v.Address != want.A[i] {
						t.Fatalf("HUDA%d got%x want%x", i, v.Address, want.A[i])
					}
				}
				checkBacking(want)
				if fileFrameHash(h.Session.Presentation.PointerData[:15260]) != want.PointerHash {
					t.Fatal("HUDactualpointer differs")
				}
			}
			checkHUD(f.HUD[0])
			for i := 1; i < len(f.HUD); i++ {
				if i == 4 {
					start, _ := h.Memory.Code.Read16(0xab4e)
					width, _ := h.Memory.Code.Read16(0xab54)
					col, _ := h.Memory.Code.Read16(0xab50)
					row, _ := h.Memory.Code.Read16(0xab52)
					x, y, found := 0, 0, false
					for j := 0; ; j++ {
						v, e := h.Memory.Code.Read8(0xab4e + int(int16(start)) + j)
						if e != nil || v == 0 {
							break
						}
						if int8(v) > 0x5a {
							mark, _ := h.Memory.Code.Read8(0x4e92 + int(v-0x5b))
							if int8(mark) > 0 {
								x = (int(col) + j%(int(width)+1)) * 8
								y = int(row) + j/(int(width)+1)*8
								found = true
								break
							}
						}
					}
					if !found {
						t.Fatal("realhelpcloseaction missing")
					}
					p := &h.Session.Presentation.Input
					for int(p.Mouse.PositionX) != x*2 || int(p.Mouse.PositionY) != y*2 {
						dx := max(-100, min(100, x*2-int(p.Mouse.PositionX)))
						dy := max(-100, min(100, y*2-int(p.Mouse.PositionY)))
						irq(uint8(int(p.Mouse.CounterX)+dx), uint8(int(p.Mouse.CounterY)+dy), false)
					}
					irq(uint8(p.Mouse.CounterX), uint8(p.Mouse.CounterY), true)
				} else {
					p := &h.Session.Presentation.Input
					irq(uint8(p.Mouse.CounterX), uint8(p.Mouse.CounterY), false)
				}
				checkHUD(f.HUD[i])
			}

		})
	}
}
