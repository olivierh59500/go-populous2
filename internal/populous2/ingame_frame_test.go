package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type ingameFrameInput struct {
	fileFrameInput
	Profile, GameMode, ControlMode, PaintFlag uint16
	ResumeZero                                bool
	ResumeFailures                            int
}
type ingameFrameFixture struct {
	Input  ingameFrameInput
	Frames []struct {
		PC                             int
		D                              [8]uint32
		BSSHash, ChipHash, PointerHash string
		ScratchHash                    string
		Window, Mouse                  []byte
		Selector, Patch, Copper        uint32
		Calls                          []struct {
			Routine    int
			D          [8]uint32
			A          [7]uint32
			Hash, Path string
		}
		Sounds []uint16
	}
}

func TestNativeInGameFrameAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/ingame_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []ingameFrameFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 784 {
		t.Fatalf("source file frame corpus coverage changed: %d", len(catalog.Cases))
	}
	exe := testBundle(t).Executable
	_, e = DecodeNativeFileFrameRules(exe)
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
				_ = m.Write16(0xeb42, f.Input.Profile)
				_ = m.Write16(0xeb44, f.Input.GameMode)
				_ = m.Write16(0xf0e, f.Input.PaintFlag)
				_ = m.Write16(0xe76a+int(int16(f.Input.Profile*314))+0x1a, f.Input.ControlMode)
				_ = m.Write32(0xeb6a, 0x200000+0xeb56)
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
				state := NativeInGameFrameState{}
				resolver := func(address uint32) ([]byte, error) {
					at := int(int64(address) - int64(p.ChipBase))
					if at < 0 || at > len(p.Chip)-32000 {
						return nil, fmt.Errorf("source screen outside chip RAM")
					}
					return p.Chip[at : at+32000], nil
				}
				calls, sounds := 0, 0
				var expected int
				resumeCalls := 0
				pendingZero := false
				cb := NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, ReadAbsolute: func(at uint32) (uint8, error) {
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
				}, Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if *phase != 0 {
						*phase = 0
						return NativeCommandFrameResult{Complete: true, Zero: pendingZero}, nil
					}
					want := f.Frames[expected]
					if calls >= len(want.Calls) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected source child%x", call.Routine)
					}
					native := want.Calls[calls]
					if call.Routine != native.Routine || call.Frame.D != native.D {
						return NativeCommandFrameResult{}, fmt.Errorf("source child%x D differs: got%08x expected%08x", call.Routine, call.Frame.D, native.D)
					}
					for i, a := range call.A {
						if call.Arguments&(1<<i) != 0 && a.Address != native.A[i] {
							return NativeCommandFrameResult{}, fmt.Errorf("source child%x A%d differs%x/%x", call.Routine, i, a.Address, native.A[i])
						}
					}
					if fileFrameHash(fileFrameMemoryBytes(t, m)) != native.Hash {
						return NativeCommandFrameResult{}, fmt.Errorf("source child%x BSS prefix differs", call.Routine)
					}
					if call.Routine == 0x111ae {
						_ = m.Write16(0xeb42, uint16(frame.D[0]))
					}
					zero := false
					if call.Routine == 0x181c0 {
						zero = f.Input.ResumeZero && resumeCalls >= f.Input.ResumeFailures
						resumeCalls++
					}
					for i := range frame.D {
						frame.D[i] ^= 0x567800 + uint32(calls+1)*0x10101 + uint32(i)*0x11111
					}

					calls++
					if suspended {
						pendingZero = zero
						*phase = 1
						return NativeCommandFrameResult{Complete: false, Zero: true}, nil
					}
					return NativeCommandFrameResult{Complete: true, Zero: zero}, nil
				}}
				check := func() {
					t.Helper()
					out, e := state.Advance(cb)
					for resumes := 0; state.ChildActive && e == nil && resumes < 16; resumes++ {
						out, e = state.Advance(cb)
					}
					if e != nil {
						t.Fatal(e)
					}
					if state.ChildActive {
						t.Fatal("source child continuation did not complete")
					}
					want := f.Frames[expected]
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
					if !bytes.Equal(code[0x4710:0x471c], want.Window) {
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

func TestNativeInGameFrameRequiresActualChildrenAndRetainsReturn(t *testing.T) {
	exe := testBundle(t).Executable
	p, e := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
	if e != nil {
		t.Fatal(e)
	}
	code := fileFrameRelocatedCode(t)
	memory := p.Memory(commandFrameBacking(make([]byte, 0x11280)))
	frame := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	cb := NativeFileFrameCallbacks{Code: commandFrameBacking(code), Memory: memory, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: func(uint32) ([]byte, error) { return nil, fmt.Errorf("not reached") }}
	state := NativeInGameFrameState{}
	out, e := state.Advance(cb)
	if e == nil || out.Complete || state.Finished {
		t.Fatal("missing actual audio-stop child completed menu")
	}
	hash, registers := fileFrameHash(code), frame.D
	out, again := state.Advance(cb)
	if again != e || out.Complete || fileFrameHash(code) != hash || frame.D != registers {
		t.Fatal("failed menu prefix replayed")
	}
	state = NativeInGameFrameState{Started: true, Finished: true, Registers: registers, Saved: registers}
	frame.D = [8]uint32{}
	out, e = state.Advance(cb)
	if e != nil || !out.Complete || frame.D != registers || fileFrameHash(code) != hash {
		t.Fatal("finished menu reran its children")
	}
}
