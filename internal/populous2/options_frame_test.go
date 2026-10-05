package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type optionsFrameFixture struct {
	Input struct {
		Name                                                     string
		Owner, Mode, Flags1, Flags2, Reaction1, Reaction2, Music uint16
		D                                                        [8]uint32
		Events                                                   []struct {
			Action int
			Keys   []byte
			VBlank bool
		}
	}
	Frames []struct {
		PC                                       int
		Waiting, Complete                        bool
		D                                        [8]uint32
		BSSHash, CodeHash, ChipHash, PointerHash string
		Selector, Patch, Copper                  uint32
		Sounds                                   []uint16
	}
}

func TestNativeOptionsRequesterAgainstOriginalCPUAndIRQs(t *testing.T) {
	data, err := os.ReadFile("testdata/options_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []optionsFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 72 {
		t.Fatalf("native options corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeOptionsFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	frames, waiting, finished := 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) == 0 {
				t.Fatal("empty original options trace")
			}
			p, err := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			p.InterruptChain = false
			if _, err = p.Initialize(bundle.Executable, NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			m := p.Memory(commandFrameBacking(make([]byte, 0x11280)))
			code := fileFrameRelocatedCode(t)
			cm := commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			_ = cm.Write32(0x77a, p.CopperSelector)
			_ = cm.Write32(0x77e, p.SpritePatchPointer)
			for _, patch := range []nativeHeroPatch{{0xeb42, 2, uint32(f.Input.Owner)}, {0xeb44, 2, uint32(f.Input.Mode)}, {0xeb2c, 2, uint32(f.Input.Flags1)}, {0xeb2e, 2, uint32(f.Input.Flags2)}, {0xe8a4 + 0x68, 2, uint32(f.Input.Reaction1)}, {0xe9de + 0x68, 2, uint32(f.Input.Reaction2)}, {0x3bc, 2, uint32(f.Input.Music)}} {
				renderFramePatch(m, patch)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := NativeOptionsFrameState{}
			expected, sounds := 0, 0
			resolver := func(address uint32) ([]byte, error) {
				at := int(int64(address) - int64(p.ChipBase))
				if at < 0 || at > len(p.Chip)-32000 {
					return nil, fmt.Errorf("native options target %#x unavailable", address)
				}
				return p.Chip[at : at+32000], nil
			}
			cb := NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
				want := f.Frames[expected]
				if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
					return fmt.Errorf("frame%d source sound differs", expected)
				}
				sounds++
				return nil
			}}
			check := func() {
				step, err := state.Advance(&rules, cb)
				if err != nil {
					t.Fatalf("frame%d: %v", expected, err)
				}
				want := f.Frames[expected]
				if step.PC != want.PC || step.Waiting != want.Waiting || step.Complete != want.Complete || step.Idle != (!want.Waiting && !want.Complete) || frame.D != want.D {
					t.Fatalf("frame%d native poll/modal context differs: got%+v D%x wantPC%x wait%v done%v D%x", expected, step, frame.D, want.PC, want.Waiting, want.Complete, want.D)
				}
				values := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
				for i, v := range values {
					binary.BigEndian.PutUint16(code[0xa2a+i*2:], v)
				}
				for _, pair := range []struct{ name, got, want string }{{"BSS", fileFrameHash(fileFrameMemoryBytes(t, m)), want.BSSHash}, {"CODE", fileFrameHash(code), want.CodeHash}, {"chip pixels/Copper", fileFrameHash(p.Chip), want.ChipHash}, {"pointer RAM", fileFrameHash(p.PointerData[:15260]), want.PointerHash}} {
					if pair.got != pair.want {
						t.Fatalf("frame%d native options %s differs: got%s want%s", expected, pair.name, pair.got, pair.want)
					}
				}
				if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper || sounds != len(want.Sounds) {
					t.Fatal("native options retained video/sound metadata differs")
				}
				frames++
				if step.Waiting {
					waiting++
				}
				if step.Complete {
					finished++
				}
				sounds = 0
			}
			check()
			irq := func(x, y uint8, left bool) {
				if _, err = p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame); err != nil {
					t.Fatal(err)
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
				t.Fatalf("source action%d unavailable", action)
				return 0, 0
			}
			move := func(x, y int) {
				for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
					dx, dy := max(-100, min(100, x*2-int(p.Input.Mouse.PositionX))), max(-100, min(100, y*2-int(p.Input.Mouse.PositionY)))
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
					// The original IRQ chains to the genuine interrupted $786
					// instruction. Retain that mutable CODE pointer explicitly.
					if state.Modal.Waiting {
						if err := cm.Write32(0x68a, 0x100786); err != nil {
							t.Fatal(err)
						}
					}
					if err := p.Input.KeyboardInterrupt(wire); err != nil {
						t.Fatal(err)
					}
				}
				check()
			}
			if state.Finished {
				before, registers := fileFrameHash(p.Chip), frame.D
				step, err := state.Advance(&rules, cb)
				if err != nil || !step.Complete || before != fileFrameHash(p.Chip) || frame.D != registers {
					t.Fatal("completed options requester replayed its prefix")
				}
			}
		})
	}
	if frames != 864 || waiting != 132 || finished != 60 {
		t.Fatalf("native options coverage changed: frames%d waits%d exits%d", frames, waiting, finished)
	}
}
