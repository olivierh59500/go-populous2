package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type protectionFrameInput struct {
	Name                        string
	Beam, Counter, Gate, Cursor uint16
	D                           [8]uint32
	Events                      []struct {
		Action int
		VBlank bool
	}
}
type protectionFrameFixture struct {
	Input   protectionFrameInput
	CallerA [7]uint32
	Frames  []struct {
		PC                                       int
		D                                        [8]uint32
		A                                        [7]uint32
		BSSHash, ChipHash, PointerHash, CodeHash string
		ScratchHash                              string
		Faces, Mouse                             []byte
		Selector, Patch, Copper                  uint32
		Calls                                    []struct {
			Routine   int
			D, AfterD [8]uint32
			A         [7]uint32
		}
		Sounds []uint16
	}
}

func TestNativeProtectionFrameAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/protection_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []protectionFrameFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 68 {
		t.Fatalf("native protection corpus changed:%d", len(catalog.Cases))
	}
	bundle := testBundle(t)
	exe := bundle.Executable
	rules, e := DecodeNativeProtectionFrameRules(exe, bundle.Raw["faces.pak"])
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		for _, suspended := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-resource-pending%v", f.Input.Name, suspended), func(t *testing.T) {
				p, e := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0x408; i < len(p.Chip); i++ {
					p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
				}
				p.InterruptChain = false
				if _, e = p.Initialize(exe, NativeMouseSample{}); e != nil {
					t.Fatal(e)
				}
				raw := make([]byte, 0x11280)
				m := p.Memory(commandFrameBacking(raw))
				code := fileFrameRelocatedCode(t)
				cm := commandFrameBacking(code)
				_ = cm.Write16(0x3ea, 0)
				_ = cm.Write16(0xa2a, f.Input.Cursor)
				p.Input.Mouse.Image = f.Input.Cursor
				_ = cm.Write32(0x77a, p.CopperSelector)
				_ = cm.Write32(0x77e, p.SpritePatchPointer)
				_ = m.Write16(0x3b0, f.Input.Gate)
				_ = m.Write16(0xe, f.Input.Counter)
				_ = m.Write32(0x3ac, 0x3ffffff)
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeProtectionFrameState{}
				for i, a := range f.CallerA {
					state.A[i] = NativeRequesterAddress{Address: a, Absolute: true}
				}
				calls, sounds, expected := 0, 0, 0
				resolver := func(address uint32) ([]byte, error) {
					at := int(int64(address) - int64(p.ChipBase))
					if at < 0 || at > len(p.Chip)-32000 {
						return nil, fmt.Errorf("native screen address%x unavailable", address)
					}
					return p.Chip[at : at+32000], nil
				}
				callCheck := func(routine int, c *NativeFrameRegisterContext) error {
					want := f.Frames[expected]
					if calls >= len(want.Calls) {
						return fmt.Errorf("unexpected native child%x", routine)
					}
					native := want.Calls[calls]
					if routine != native.Routine || c.D != native.D {
						return fmt.Errorf("child%x D differs:got%08x want%08x", routine, c.D, native.D)
					}
					calls++
					return nil
				}
				cb := NativeProtectionFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Beam: func() (uint16, error) { return f.Input.Beam, nil },
					Ownership: func(owned bool, c *NativeFrameRegisterContext) error {
						routine := 0xe28
						if owned {
							routine = 0xe4c
						}
						return callCheck(routine, c)
					},
					Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
						want := f.Frames[expected]
						if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
							return fmt.Errorf("native protection cue differs")
						}
						sounds++
						return nil
					},
					Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
						if *phase == 0 {
							if e := callCheck(call.Routine, call.Frame); e != nil {
								return NativeCommandFrameResult{}, e
							}
							if suspended {
								*phase = 1
								return NativeCommandFrameResult{}, nil
							}
						}
						*phase = 0
						if call.Routine != 0x19cd0 && call.Routine != 0x1a32a {
							return NativeCommandFrameResult{}, fmt.Errorf("unexpected protection resource child")
						}
						// This fixture configures the genuine cached-resource path. Both
						// original children preserve D1-D7, retain cursor+3ba and return2.
						cursor, e := cm.Read16(0xa2a)
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						if e = m.Write16(0x3ba, cursor); e != nil {
							return NativeCommandFrameResult{}, e
						}
						call.Frame.D[0] = 2
						if call.Frame.D != f.Frames[expected].Calls[calls-1].AfterD {
							return NativeCommandFrameResult{}, fmt.Errorf("cached native resource output differs")
						}
						return NativeCommandFrameResult{Complete: true}, nil
					},
				}
				syncMouse := func() {
					values := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
					for i, v := range values {
						binary.BigEndian.PutUint16(code[0xa2a+i*2:], v)
					}
				}
				check := func() {
					step, e := state.Advance(&rules, cb)
					if state.ChildActive && e == nil {
						if !step.Waiting || step.Complete {
							t.Fatal("pending resource child incorrectly completed")
						}
						step, e = state.Advance(&rules, cb)
					}
					if e != nil {
						t.Fatalf("frame%d:%v", expected, e)
					}
					want := f.Frames[expected]
					if step.PC != want.PC || frame.D != want.D {
						t.Fatalf("native continuation frame%d PC got%x want%x Dgot%08x want%08x", expected, step.PC, want.PC, frame.D, want.D)
					}
					if step.Complete != (want.PC == 0) || step.Waiting == (want.PC == 0) {
						t.Fatal("native wait/completion differs")
					}
					if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash {
						t.Fatalf("frame%d complete BSS differs", expected)
					}
					if fileFrameHash(p.Chip) != want.ChipHash {
						t.Fatalf("frame%d full pixel/Copper memory differs", expected)
					}
					if fileFrameHash(p.PointerData[:15260]) != want.PointerHash {
						t.Fatalf("frame%d real pointer bitmap differs", expected)
					}
					syncMouse()
					if fileFrameHash(code) != want.CodeHash {
						t.Fatalf("frame%d complete retained CODE differs", expected)
					}
					if fileFrameHash(code[0xab4e:0xab4e+1100]) != want.ScratchHash || !bytes.Equal(code[0x326c:0x326f], want.Faces) {
						t.Fatal("native requester workspace or face selection differs")
					}
					if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper {
						t.Fatal("native screen/Copper continuation differs")
					}
					if calls != len(want.Calls) || sounds != len(want.Sounds) {
						t.Fatalf("frame%d source child count differs", expected)
					}
					if step.Complete {
						if frame.D != f.Input.D {
							t.Fatal("caller registers not restored")
						}
						for i, a := range state.A {
							if a.Address != f.CallerA[i] || want.A[i] != f.CallerA[i] {
								t.Fatal("source saved address registers not restored")
							}
						}
					}
					calls, sounds = 0, 0
				}
				check()
				irq := func(x, y uint8, left bool) {
					if _, e = p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame); e != nil {
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
					t.Fatalf("source action%d not found", action)
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
					check()
				}
			})
		}
	}
}

func TestNativeProtectionDoesNotCompleteWithoutResource(t *testing.T) {
	bundle := testBundle(t)
	r, e := DecodeNativeProtectionFrameRules(bundle.Executable, bundle.Raw["faces.pak"])
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Initialize(bundle.Executable, NativeMouseSample{}); e != nil {
		t.Fatal(e)
	}
	code := fileFrameRelocatedCode(t)
	m := p.Memory(commandFrameBacking(make([]byte, 0x11280)))
	c := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	s := NativeProtectionFrameState{}
	ownership := 0
	cb := NativeProtectionFrameCallbacks{Code: commandFrameBacking(code), Memory: m, Frame: &c, Presentation: p, CodeBase: 0x100000, Ownership: func(bool, *NativeFrameRegisterContext) error { ownership++; return nil }}
	step, e := s.Advance(&r, cb)
	if e == nil || step.Complete || s.Finished || s.PC != 0x316a {
		t.Fatal("missing real resource operation completed")
	}
	beforeCode, beforeChip, d := fileFrameHash(code), fileFrameHash(p.Chip), c.D
	step, again := s.Advance(&r, cb)
	if again != e || step.Complete || ownership != 1 || beforeCode != fileFrameHash(code) || beforeChip != fileFrameHash(p.Chip) || c.D != d {
		t.Fatal("failed child replayed source prefix")
	}
}
