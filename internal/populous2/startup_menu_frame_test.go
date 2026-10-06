package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type startupMenuFixture struct {
	Input struct {
		Name             string
		D                [8]uint32
		A                [7]uint32
		DialogFlag, Mode uint16
		Events           []struct {
			Action int
			VBlank bool
		}
		Absolute []byte
	}
	Frames []struct {
		PC                                       int
		D                                        [8]uint32
		A                                        [7]uint32
		CCR                                      uint16
		BSSHash, CodeHash, ChipHash, PointerHash string
		Selector, Patch, Copper                  uint32
		Sounds                                   []uint16
	}
}

func TestNativeStartupMenuAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/startup_menu_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []startupMenuFixture }
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 60 {
		t.Fatalf("native startup menu corpus changed:%d", len(catalog.Cases))
	}
	bundle := testBundle(t)
	base := resourceFrameInitialRAM(t)
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
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
			copy(ram, f.Input.Absolute)
			copy(ram[0x500000:], p.Chip)
			copy(ram[0x400000:], p.PointerData)
			code := ram[0x100000:0x13fa2c]
			raw := make([]byte, 0x11280)
			m, cm := p.Memory(commandFrameBacking(raw)), commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			_ = cm.Write32(0x77a, p.CopperSelector)
			_ = cm.Write32(0x77e, p.SpritePatchPointer)
			_ = m.Write16(0x3b0, f.Input.DialogFlag)
			_ = m.Write16(0xeb44, f.Input.Mode)
			_ = m.Write32(0xf40, 10)
			for i, v := range []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY} {
				_ = cm.Write16(0xa2a+i*2, v)
			}
			physical := nativeByteAddressMemory(func(at int) (byte, error) {
				if at >= 0x200000 && at < 0x211280 {
					return m.Read8(at - 0x200000)
				}
				if at >= 0x500000 && at < 0x500000+len(p.Chip) {
					return p.Chip[at-0x500000], nil
				}
				return commandFrameBacking(ram).Read8(at)
			}, func(at int, v byte) error {
				if at >= 0x200000 && at < 0x211280 {
					return m.Write8(at-0x200000, v)
				}
				if at >= 0x500000 && at < 0x500000+len(p.Chip) {
					p.Chip[at-0x500000] = v
					return nil
				}
				return commandFrameBacking(ram).Write8(at, v)
			})
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			host := NativeStartupMenuHostFrameState{}
			s := &host.Menu
			for i, v := range f.Input.A {
				s.A[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			expected, sounds := 0, 0
			cb := NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: func(address uint32) ([]byte, error) {
				off := int(int64(address) - 0x500000)
				if off < 0 || off > len(p.Chip)-32000 {
					return nil, fmt.Errorf("actual menu bitmap%x unavailable", address)
				}
				return p.Chip[off : off+32000], nil
			}, ReadAbsolute: func(address uint32) (byte, error) { return physical.Read8(int(address)) }, Sound: func(cue uint16, _ *NativeFrameRegisterContext) error {
				want := f.Frames[expected].Sounds
				if sounds >= len(want) || cue != want[sounds] {
					return fmt.Errorf("actual menu cue differs")
				}
				sounds++
				return nil
			}}, RAM: physical}
			cb.Child = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if call.Routine != f.Frames[expected].PC {
					return NativeCommandFrameResult{}, fmt.Errorf("unexpected genuine menu child%x", call.Routine)
				}
				*phase++
				return NativeCommandFrameResult{}, nil // Actual unresolved child entry, never a fabricated success.
			}
			check := func() {
				want := f.Frames[expected]
				step, e := host.Advance(cb)
				if e != nil {
					t.Fatal(e)
				}
				if want.PC == 0 {
					if !step.Complete || !step.FlagsKnown || step.Zero != (want.CCR&4 != 0) || step.Negative != (want.CCR&8 != 0) {
						t.Fatal("native menu terminal condition differs")
					}
				} else if step.Complete || !step.Waiting {
					t.Fatal("native menu pending boundary completed")
				}
				if frame.D != want.D {
					t.Errorf("native menu D differs:%08x/%08x", frame.D, want.D)
				}
				for i, a := range s.A {
					if a.Address != want.A[i] {
						t.Errorf("native menu A%d differs:%x/%x", i, a.Address, want.A[i])
					}
				}
				if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(p.Chip) != want.ChipHash || fileFrameHash(p.PointerData[:15260]) != want.PointerHash {
					t.Fatal("native menu full BSS/CODE/screens/pointer differs")
				}
				if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper || sounds != len(want.Sounds) {
					t.Fatal("native menu Copper/cue state differs")
				}
				sounds = 0
			}
			irq := func(x, y byte, left bool) {
				if _, e := p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame); e != nil {
					t.Fatal(e)
				}
				for i, v := range []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY} {
					_ = cm.Write16(0xa2a+i*2, v)
				}
			}
			check()
			for i, event := range f.Input.Events {
				expected = i + 1
				if event.Action > 0 {
					start := 0xab4e + int(int16(binary.BigEndian.Uint16(code[0xab4e:])))
					width := int(binary.BigEndian.Uint16(code[0xab54:]))
					count, found, x, y := 0, false, 0, 0
					for j := 0; code[start+j] != 0; j++ {
						v := code[start+j]
						if int8(v) > 0x5a && int8(code[0x4e92+int(v)-0x5b]) > 0 {
							count += 2
							if count == event.Action {
								x = (int(binary.BigEndian.Uint16(code[0xab50:])) + j%(width+1)) * 8
								y = int(binary.BigEndian.Uint16(code[0xab52:])) + j/(width+1)*8
								found = true
								break
							}
						}
					}
					if !found {
						t.Fatal("native menu button absent")
					}
					for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
						dx := max(-100, min(100, x*2-int(p.Input.Mouse.PositionX)))
						dy := max(-100, min(100, y*2-int(p.Input.Mouse.PositionY)))
						irq(byte(int(p.Input.Mouse.CounterX)+dx), byte(int(p.Input.Mouse.CounterY)+dy), false)
					}
					irq(byte(p.Input.Mouse.CounterX), byte(p.Input.Mouse.CounterY), true)
				} else if event.VBlank {
					irq(byte(p.Input.Mouse.CounterX), byte(p.Input.Mouse.CounterY), false)
				}
				check()
			}
		})
	}
}
