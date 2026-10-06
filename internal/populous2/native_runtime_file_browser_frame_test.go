package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeRuntimeFileBrowserFrameAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/file_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []fileFrameFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 340 {
		t.Fatalf("source file frame corpus coverage changed: %d", len(catalog.Cases))
	}
	aData, e := os.ReadFile("testdata/native_runtime_file_browser_a_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var aCorpus struct {
		Cases []struct {
			Name   string
			Frames [][7]uint32
		}
	}
	if e = json.Unmarshal(aData, &aCorpus); e != nil {
		t.Fatal(e)
	}
	fullA := map[string][][7]uint32{}
	for _, row := range aCorpus.Cases {
		fullA[row.Name] = row.Frames
	}
	if len(fullA) != 340 {
		t.Fatal("full A corpus changed")
	}
	exe := testBundle(t).Executable
	rules, e := DecodeNativeFileFrameRules(exe)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		for _, suspended := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, suspended), func(t *testing.T) {
				p, e := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0x408; i < len(p.Chip); i++ {
					p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
				}
				p.InterruptChain = false
				if _, e := p.Initialize(exe, NativeMouseSample{}); e != nil {
					t.Fatal(e)
				}
				raw := make([]byte, 0x11280)
				m := p.Memory(commandFrameBacking(raw))
				_ = m.Write16(0x3b0, f.Input.DialogFlag)
				_ = m.Write32(0xf40, 10)
				_ = m.Write32(0xf32, 0xc930)
				code := fileFrameRelocatedCode(t)
				cm := commandFrameBacking(code)
				_ = cm.Write16(0x3ea, 0)
				_ = cm.Write16(0x4468, 0)
				if f.Input.Save {
					_ = cm.Write16(0x4468, 1)
				}
				_ = cm.Write16(0x3f90, f.Input.Alternate)
				copy(code[0x4440:], f.Input.Drawer)
				code[0x4440+len(f.Input.Drawer)] = 0
				copy(code[0x4416:], f.Input.Filename)
				code[0x4416+len(f.Input.Filename)] = 0
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeRuntimeFileBrowserFrameState{}
				resolver := func(address uint32) ([]byte, error) {
					at := int(int64(address) - int64(p.ChipBase))
					if at < 0 || at > len(p.Chip)-32000 {
						return nil, fmt.Errorf("source screen outside chip RAM")
					}
					return p.Chip[at : at+32000], nil
				}
				calls, sounds := 0, 0
				var expected int
				cb := NativeRuntimeFileBrowserFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, ReadAbsolute: func(at uint32) (uint8, error) {
					if at >= 0x100000 && at < 0x100000+uint32(len(code)) {
						return cm.Read8(int(at - 0x100000))
					}
					if at >= 0x200000 && at < 0x211280 {
						return m.Read8(int(at - 0x200000))
					}
					if int(at) >= len(f.Input.Absolute) {
						return 0, fmt.Errorf("physical RAM%x unavailable", at)
					}
					return f.Input.Absolute[at], nil
				}, Sound: func(argument uint16, c *NativeFrameRegisterContext) error {
					want := f.Frames[expected]
					if sounds >= len(want.Sounds) || want.Sounds[sounds] != argument {
						return fmt.Errorf("unexpected source sound%x", argument)
					}
					sounds++
					return nil
				}}, Child: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if *phase != 0 {
						*phase = 0
						return NativeCommandFrameResult{Complete: true}, nil
					}
					want := f.Frames[expected]
					if calls >= len(want.Calls) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected source child%x", call.Routine)
					}
					native := want.Calls[calls]
					if call.Routine != native.Routine || call.Frame.D != native.D {
						return NativeCommandFrameResult{}, fmt.Errorf("source child%x D differs: got%08x expected%08x", call.Routine, call.Frame.D, native.D)
					}
					for i, a := range *call.A {
						if a.Address != native.A[i] {
							return NativeCommandFrameResult{}, fmt.Errorf("source child%x A%d differs%x/%x", call.Routine, i, a.Address, native.A[i])
						}
					}
					if fileFrameHash(fileFrameMemoryBytes(t, m)) != native.Hash {
						return NativeCommandFrameResult{}, fmt.Errorf("source child%x BSS prefix differs", call.Routine)
					}
					if call.Routine == 0x19936 {
						at := 0x3be
						for i := 0; i < f.Input.Count; i++ {
							name := fmt.Sprintf("FILE%02d.GAM", i)
							copy(raw[at:], name)
							for j, v := range []byte(name) {
								if e := m.Write8(at+j, v); e != nil {
									return NativeCommandFrameResult{}, e
								}
							}
							at += len(name)
							_ = m.Write8(at, 0)
							at++
						}
						_ = cm.Write16(0x443e, uint16(f.Input.Count))
						frame.D[0] = uint32(f.Input.Count)
						frame.D[1] = 0x9abc3210
						frame.D[2] = 0x100000 + 0x199f8
						frame.Word(7, uint16(f.Input.Count))
					} else if call.Routine == 0x19c1c || call.Routine == 0x19afc {
						if fileFrameCString(t, cm, int(call.A[0].Address-0x100000)) != native.Path {
							return NativeCommandFrameResult{}, fmt.Errorf("actual DOS path differs")
						}
						frame.D[0] = uint32(f.Input.Return)
					}
					calls++
					if suspended {
						*phase = 1
						return NativeCommandFrameResult{Complete: false, Zero: true}, nil
					}
					return NativeCommandFrameResult{Complete: true, Zero: false}, nil
				}}
				check := func() {
					t.Helper()
					out, e := state.Advance(&rules, cb)
					for resumes := 0; state.ChildActive && e == nil && resumes < 16; resumes++ {
						out, e = state.Advance(&rules, cb)
					}
					if e != nil {
						t.Fatal(e)
					}
					if state.ChildActive {
						t.Fatal("source child continuation did not complete")
					}
					want := f.Frames[expected]
					addresses := fullA[f.Input.Name][expected]
					for i, a := range state.A {
						if a.Address != addresses[i] {
							t.Fatalf("stage%d A%d got%x want%x", expected, i, a.Address, addresses[i])
						}
					}
					if frame.D != want.D {
						t.Fatalf("source PC%x D differs\ngot %08x\nwant%08x", out.PC, frame.D, want.D)
					}
					if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash {
						t.Fatal("complete source BSS differs")
					}
					if fileFrameHash(code[0xab4e:0xab4e+2048]) != want.ScratchHash {
						t.Fatal("requester workspace digest differs")
					}
					if fileFrameHash(p.Chip) != want.ChipHash {
						t.Fatal("complete source chip/pixel RAM differs")
					}
					if fileFrameHash(p.PointerData[:15260]) != want.PointerHash {
						t.Fatal("actual pointer sprite RAM differs")
					}
					if fileFrameHash(code[0x3f90:0x446a]) != want.WindowHash {
						t.Fatal("raw file CODE window differs")
					}
					if fileFrameHash(code[0xab4e:0xab4e+2048]) != want.ScratchHash {
						t.Fatal("raw requester CODE workspace differs")
					}
					mouse := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
					for i, v := range mouse {
						if v != binary.BigEndian.Uint16(want.Mouse[i*2:]) {
							t.Fatal("actual mouse CODE state differs")
						}
					}
					if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper {
						t.Fatal("actual Copper continuation differs")
					}
					if calls != len(want.Calls) || sounds != len(want.Sounds) {
						t.Fatalf("source child/audio calls missing frame%d calls%d/%d sounds%d/%d", expected, calls, len(want.Calls), sounds, len(want.Sounds))
					}
					calls, sounds = 0, 0
				}
				check()
				irq := func(x, y uint8, left bool) {
					if _, e := p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame); e != nil {
						t.Fatal(e)
					}
				}
				find := func(action int) (int, int) {
					start := 0xab4e + int(int16(binary.BigEndian.Uint16(code[0xab4e:])))
					width := int(binary.BigEndian.Uint16(code[0xab54:]))
					count := 0
					for i := 0; code[start+i] != 0; i++ {
						v := code[start+i]
						if int8(v) > 0x5a && int8(code[0x4e92+int(v)-0x5b]) > 0 {
							count += 2
							if count == action {
								return (int(binary.BigEndian.Uint16(code[0xab50:])) + i%(width+1)) * 8, int(binary.BigEndian.Uint16(code[0xab52:])) + i/(width+1)*8
							}
						}
					}
					t.Fatalf("action%d missing", action)
					return 0, 0
				}
				move := func(x, y int) {
					for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
						dx, dy := x*2-int(p.Input.Mouse.PositionX), y*2-int(p.Input.Mouse.PositionY)
						dx = max(-100, min(100, dx))
						dy = max(-100, min(100, dy))
						irq(uint8(int(p.Input.Mouse.CounterX)+dx), uint8(int(p.Input.Mouse.CounterY)+dy), false)
					}
					irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), true)
				}
				for i, event := range f.Input.Events {
					expected = i + 1
					if event.Action > 0 {
						x, y := find(event.Action)
						move(x, y)
					} else if event.VBlank {
						irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), false)
					}
					for _, wire := range event.Keys {
						if e := p.Input.KeyboardInterrupt(wire); e != nil {
							t.Fatal(e)
						}
					}
					check()
				}
			})
		}
	}
}
