package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type errorFrameFixture struct {
	Input struct {
		Name             string
		Routine          uint32
		Gate             uint16
		Physical         bool
		PhysicalTemplate bool
		Message          string
		D                [8]uint32
		Events           []struct {
			Action int
			Keys   []byte
			VBlank bool
		}
	}
	Frames []struct {
		PC                                       int
		Waiting, Complete                        bool
		D                                        [8]uint32
		A                                        [7]uint32
		BSSHash, CodeHash, ChipHash, PointerHash string
		Selector, Patch, Copper                  uint32
		Sounds                                   []uint16
	}
}

func TestNativeErrorFrameAgainstOriginalCPUAndVBlank(t *testing.T) {
	data, err := os.ReadFile("testdata/error_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []errorFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 54 {
		t.Fatalf("native error corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	base := resourceFrameInitialRAM(t)
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
			ram := append([]byte(nil), base...)
			m := p.Memory(commandFrameBacking(ram[0x200000:0x211280]))
			code := ram[0x100000 : 0x100000+0x3fa2c]
			cm := commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			_ = cm.Write32(0x77a, p.CopperSelector)
			_ = cm.Write32(0x77e, p.SpritePatchPointer)
			_ = m.Write16(0x3b0, f.Input.Gate)
			if f.Input.Physical {
				binary.BigEndian.PutUint32(ram[0x900000:], 0x900010)
				copy(ram[0x900010:], append([]byte(f.Input.Message), 0))
			}

			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := NativeErrorFrameState{Routine: int(f.Input.Routine)}
			for i := range state.A {
				state.A[i] = NativeRequesterAddress{Address: 0x800000 + uint32(i)*0x1000, Absolute: true}
			}
			state.A[1] = NativeRequesterAddress{Address: 0x100000 + 0x91d0, Code: true}
			if f.Input.PhysicalTemplate {
				copy(ram[0x900100:], code[0x91d0:0x930c])
				state.A[1] = NativeRequesterAddress{Address: 0x900100, Absolute: true}
			}
			state.A[2] = NativeRequesterAddress{Address: 0x100000 + 0xa93c, Code: true}
			if f.Input.Physical {
				state.A[2] = NativeRequesterAddress{Address: 0x900000, Absolute: true}
			}
			expected, sounds := 0, 0
			resolver := func(address uint32) ([]byte, error) {
				at := int(int64(address) - int64(p.ChipBase))
				if at < 0 || at > len(p.Chip)-32000 {
					return nil, fmt.Errorf("native error target %#x unavailable", address)
				}
				return p.Chip[at : at+32000], nil
			}
			cb := NativeErrorFrameCallbacks{RAM: commandFrameBacking(ram), NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
				want := f.Frames[expected]
				if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
					return fmt.Errorf("frame%d source sound differs", expected)
				}
				sounds++
				return nil
			}},
			}
			check := func() {
				step, err := state.Advance(cb)
				if err != nil {
					t.Fatalf("frame%d: %v", expected, err)
				}
				want := f.Frames[expected]
				if (step.Complete && want.PC != 0) || (!step.Complete && step.PC != want.PC) || step.Waiting != want.Waiting || step.Complete != want.Complete || step.Idle != (!want.Waiting && !want.Complete) || frame.D != want.D {
					t.Fatalf("frame%d native poll/modal context differs: got%+v D%x wantPC%x wait%v done%v D%x", expected, step, frame.D, want.PC, want.Waiting, want.Complete, want.D)
				}
				values := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
				for i, v := range values {
					binary.BigEndian.PutUint16(code[0xa2a+i*2:], v)
				}
				for _, pair := range []struct{ name, got, want string }{{"BSS", fileFrameHash(fileFrameMemoryBytes(t, m)), want.BSSHash}, {"CODE", fileFrameHash(code), want.CodeHash}, {"chip pixels/Copper", fileFrameHash(p.Chip), want.ChipHash}, {"pointer RAM", fileFrameHash(p.PointerData[:15260]), want.PointerHash}} {
					if pair.got != pair.want {
						t.Fatalf("frame%d native error %s differs: got%s want%s", expected, pair.name, pair.got, pair.want)
					}
				}
				if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper || sounds != len(want.Sounds) {
					t.Fatal("native error retained video/sound metadata differs")
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

				check()
			}
			if state.Finished {
				for i, a := range state.A {
					if a.Address != f.Frames[len(f.Frames)-1].A[i] {
						t.Fatal("native fullA restoration differs")
					}
				}
				before, registers := fileFrameHash(p.Chip), frame.D
				step, err := state.Advance(cb)
				if err != nil || !step.Complete || before != fileFrameHash(p.Chip) || frame.D != registers {
					t.Fatal("completed options requester replayed its prefix")
				}
			}
		})
	}
	if frames != 774 || waiting != 612 || finished != 54 {
		t.Fatalf("native error coverage changed: frames%d waits%d exits%d", frames, waiting, finished)
	}
}
