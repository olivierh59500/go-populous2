package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type nativeInputFixture struct {
	Input struct {
		Name          string
		Initial, Code []nativeHeroPatch
		Events        []struct {
			Mode                     string
			Wire, CounterX, CounterY uint8
			Left, Right              bool
		}
		Registers [8]uint32
		Clock     uint32
	}
	Snapshots []struct {
		Low       []byte
		Mouse     [6]uint16
		Registers [8]uint32
		Image     uint32
		Pointers  [4]uint16
		Control   [4]byte
	}
}

func TestNativeInputProductionAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/input_state_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeInputFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 690 {
		t.Fatal("original input corpus incomplete")
	}
	exe := testBundle(t).Executable
	r, err := DecodeNativeInputRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			s, err := NewNativeInputState(exe)
			if err != nil {
				t.Fatal(err)
			}
			memory := s.Memory(FollowerCleanupMemory{})
			for _, p := range f.Input.Initial {
				switch p.Width {
				case 1:
					err = memory.Write8(p.Address, uint8(p.Value))
				case 2:
					err = memory.Write16(p.Address, uint16(p.Value))
				case 4:
					err = memory.Write32(p.Address, p.Value)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			mouseWords := []*uint16{&s.Mouse.Image, &s.Mouse.CounterX, &s.Mouse.CounterY, &s.Mouse.PositionX, &s.Mouse.PositionY, &s.Mouse.MaximumY}
			for _, p := range f.Input.Code {
				if p.Address < 0xa2a || p.Address >= 0xa36 || p.Width != 2 || p.Address&1 != 0 {
					t.Fatal("original input CODE initialization outside mouse state")
				}
				*mouseWords[(p.Address-0xa2a)/2] = uint16(p.Value)
			}
			registers := f.Input.Registers
			if len(f.Input.Events) == 0 || len(f.Input.Events) != len(f.Snapshots) {
				t.Fatal("original input event snapshots incomplete")
			}
			for i, e := range f.Input.Events {
				count++
				sample := NativeMouseSample{e.CounterX, e.CounterY, e.Left, e.Right}
				var plan NativePointerPlan
				switch e.Mode {
				case "key":
					err = s.KeyboardInterrupt(e.Wire)
				case "character":
					_, err = r.Character(&s, &registers)
				case "reset":
					err = s.ResetMouseCounters(sample)
				case "mouse":
					plan, err = s.PollMouse(sample, f.Input.Clock, 0x400000, &registers)
				case "vblank":
					plan, err = s.VBlank(sample, f.Input.Clock, 0x400000)
				default:
					t.Fatal("unknown original input source")
				}
				if err != nil {
					t.Fatal(err)
				}
				want := f.Snapshots[i]
				if registers != want.Registers {
					t.Fatalf("original event%d full register continuation differs: got%x want%x", i, registers, want.Registers)
				}
				if len(want.Low) != len(s.Low) {
					t.Fatal("original low BSS snapshot incomplete")
				}
				for a, v := range s.Low {
					if v != want.Low[a] {
						t.Fatalf("original event%d low byte%x got%x want%x", i, a, v, want.Low[a])
					}
				}
				for j, v := range mouseWords {
					if *v != want.Mouse[j] {
						t.Fatalf("original event%d mouse word%d got%x want%x", i, j, *v, want.Mouse[j])
					}
				}
				if e.Mode == "mouse" || e.Mode == "vblank" {
					if plan.ImageAddress != want.Image || plan.Pointers != want.Pointers || plan.Control != want.Control {
						t.Fatalf("original event%d pointer/copper plan differs: got%+v want%x/%x/%x", i, plan, want.Image, want.Pointers, want.Control)
					}
				}
			}
		})
	}
	if count != 3819 {
		t.Fatalf("original input event coverage%d want3819", count)
	}
}

func TestNativeInputRawKeyEncodingAndSharedBacking(t *testing.T) {
	for raw := uint16(0); raw < 128; raw++ {
		press, err := NativeKeyWire(uint8(raw), true)
		if err != nil {
			t.Fatal(err)
		}
		release, err := NativeKeyWire(uint8(raw), false)
		if err != nil {
			t.Fatal(err)
		}
		if press&1 != 1 || release != press-1 || uint8(-press)>>1 != uint8(raw) {
			t.Fatal("native press/release encoding differs")
		}
	}
	if _, err := NativeKeyWire(128, true); err == nil {
		t.Fatal("out-of-bank raw key accepted")
	}
	s, err := NewNativeInputState(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	base := &scenarioScriptMemory{}
	m := s.Memory(base.callbacks())
	if err := m.Write16(0xe, 0xffff); err != nil {
		t.Fatal(err)
	}
	if err := m.Write8(0xf, 0x12); err != nil {
		t.Fatal(err)
	}
	if v, _ := m.Read16(0xe); v != 0xff12 {
		t.Fatal("overlapping VBlank word/phase byte split")
	}
	if err := m.Write32(0x138, 0x01230145); err != nil {
		t.Fatal(err)
	}
	if x, _ := m.Read16(0x138); x != 0x123 {
		t.Fatal("shared cursor word split")
	}
	if y, _ := m.Read16(0x13a); y != 0x145 {
		t.Fatal("shared cursor word split")
	}
	if err := m.Write32(0x14a, 0x12345678); err != nil {
		t.Fatal(err)
	}
	if v, _ := m.Read32(0x14a); v != 0x12345678 || base[0x14c] != 0x56 || s.Low[0x14a] != 0x12 {
		t.Fatal("low/runtime seam did not retain both backings")
	}
	if _, err := m.Read16(0x139); err == nil {
		t.Fatal("unaligned input word accepted")
	}
	before := s
	if err := s.ResetMouseCounters(NativeMouseSample{CounterX: 250, CounterY: 251}); err != nil {
		t.Fatal(err)
	}
	before.Mouse.CounterX, before.Mouse.CounterY = 250, 251
	if !reflect.DeepEqual(s, before) {
		t.Fatal("mouse counter reset changed cursor/input bytes")
	}
}
