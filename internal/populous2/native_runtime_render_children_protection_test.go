package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func nativeRuntimeRenderFacesReady(t *testing.T, h *NativeRuntimeHost) {
	t.Helper()
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: [8]uint32{8}}
	cb, err := h.ResourceCallbacks(&frame, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	state := NativeResourceHostFrameState{}
	step, err := state.Advance(&h.ResourceRules, cb)
	if err != nil || !step.Complete {
		t.Fatal("actual encoded FACES preload failed", step, err)
	}
	// CPU reference starts after resource preparation, with explicit zero BSS.
	for i := 0; i < 0x11280; i++ {
		if err := h.Memory.BSS.Write8(i, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Memory.Code.Write32(0x19e46, 0); err != nil {
		t.Fatal(err)
	}
}
func TestNativeRuntimeProtectionChildAgainstOriginalCPU(t *testing.T) {
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

	for _, f := range catalog.Cases {
		for _, suspended := range []bool{false} {
			t.Run(fmt.Sprintf("%s-resource-pending%v", f.Input.Name, suspended), func(t *testing.T) {
				h := nativeRuntimeHostTest(t)
				nativeRuntimeRenderFacesReady(t, h)
				p := h.Session.Presentation
				var e error
				for i := 0x408; i < len(p.Chip); i++ {
					p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
				}
				p.InterruptChain = false
				if _, e = p.Initialize(exe, NativeMouseSample{}); e != nil {
					t.Fatal(e)
				}
				m := h.Memory.BSS
				code := h.Code.RawData()[:len(exe.Hunks[0].Data)]
				cm := h.Memory.Code
				_ = cm.Write16(0x3ea, 0)
				_ = cm.Write16(0xa2a, f.Input.Cursor)
				p.Input.Mouse.Image = f.Input.Cursor
				_ = cm.Write32(0x77a, p.CopperSelector)
				_ = cm.Write32(0x77e, p.SpritePatchPointer)
				_ = m.Write16(0x3b0, f.Input.Gate)
				_ = m.Write16(0xe, f.Input.Counter)
				_ = m.Write32(0x3ac, 0x3ffffff)
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				var callerA [7]NativeRequesterAddress
				for i, a := range f.CallerA {
					callerA[i] = NativeRequesterAddress{Address: a, Absolute: true}
				}
				calls, sounds, expected := 0, 0, 0

				callCheck := func(routine int, c *NativeFrameRegisterContext) error {
					want := f.Frames[expected]
					if calls >= len(want.Calls) {
						return fmt.Errorf("unexpected native child%x", routine)
					}
					for calls < len(want.Calls) && (want.Calls[calls].Routine == 0x19cd0 || want.Calls[calls].Routine == 0x1a32a) {
						calls++
					}
					native := want.Calls[calls]
					if routine != native.Routine || c.D != native.D {
						return fmt.Errorf("child%x D differs:got%08x want%08x", routine, c.D, native.D)
					}
					calls++
					return nil
				}
				soundBody := nativeRuntimeDisabledRenderSound(t, h)
				children, e := h.NewRenderChildren(NativeRuntimeRenderChildrenCallbacks{CallerA: &callerA,
					Beam: func() (uint16, error) { return f.Input.Beam, nil },
					Ownership: func(owned bool, c *NativeFrameRegisterContext) error {
						// Original OS child is an explicit external source boundary.
						// The synchronous software host owns the bitmap via Execute.
						routine := 0xe28
						if owned {
							routine = 0xe4c
						}
						return callCheck(routine, c)
					},
					Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
						want := f.Frames[expected]
						if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
							return fmt.Errorf("original protection cue differs")
						}
						sounds++
						return soundBody(cue, c)
					}})
				if e != nil {
					t.Fatal(e)
				}

				syncCode := func() {
					for i := range code {
						v, e := cm.Read8(i)
						if e != nil {
							t.Fatal(e)
						}
						code[i] = v
					}
				}

				check := func() {
					_, e := children.AdvanceProtection(0x76f4, &frame)
					step := children.ProtectionStep
					state := children.Protection

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
					syncCode()
					if fileFrameHash(code) != want.CodeHash {
						t.Fatalf("frame%d complete retained CODE differs", expected)
					}
					if !bytes.Equal(code[0xab4e:0xab4e+1100], want.Scratch) || !bytes.Equal(code[0x326c:0x326f], want.Faces) {
						t.Fatal("native requester workspace or face selection differs")
					}
					if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper {
						t.Fatal("native screen/Copper continuation differs")
					}
					if sounds != len(want.Sounds) {
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
