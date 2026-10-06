package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type startupDeityEditorFixture struct {
	Input struct {
		Name                             string
		D                                [8]uint32
		A                                [7]uint32
		DialogFlag, Mode, Profile, Bolts uint16
		Parts                            [3]uint8
		XP                               [6]uint8
		Events                           []struct {
			Action int
			VBlank bool
			Keys   []byte
		}
		Absolute     []byte
		FieldName    []byte
		NameSentinel bool
	}
	Frames []struct {
		PC                                       int
		D                                        [8]uint32
		A                                        [7]uint32
		CCR                                      uint16
		BSSHash, CodeHash, ChipHash, PointerHash string
		Selector, Patch, Copper                  uint32
		Sounds                                   []uint16
		GodBytes, NameBytes, PasswordBytes       []byte
	}
}

func TestNativeStartupDeityEditorAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/startup_menu_children_deity_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []startupDeityEditorFixture }
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 284 {
		t.Fatalf("native startup menu corpus changed:%d", len(catalog.Cases))
	}
	bundle := testBundle(t)
	base := resourceFrameInitialRAM(t)
	faceAddress := binary.BigEndian.Uint32(base[0x1212ba:])
	copy(base[faceAddress:], bundle.Raw["faces.pak"])
	if err = PrepareNativeResourceFramePlanes(commandFrameBacking(base), 0x1212ba, 0x2003be); err != nil {
		t.Fatal(err)
	}
	render, err := DecodeNativeRenderFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	childrenRules, err := DecodeNativeCampaignSelectionChildrenRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	resourceRules, err := DecodeNativeResourceFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
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
			_ = m.Write16(0xeb42, f.Input.Profile)
			_ = m.Write16(0xe8fc, f.Input.Bolts)
			for i, v := range f.Input.Parts {
				_ = m.Write8(0xe8f2+i, v)
			}
			for i, v := range f.Input.XP {
				_ = m.Write8(0xe8f6+i, v)
			}
			_ = m.Write32(0x3ac, 1<<8)
			name := f.Input.FieldName
			if name == nil {
				name = []byte("NATIVE")
			}
			for i, v := range name {
				_ = m.Write8(0xeb30+i, v)
			}
			if f.Input.NameSentinel {
				_ = m.Write8(0xeb41, 0xff)
			}
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
			state := NativeStartupDeityFrameState{}
			s := &state
			var palette *NativeFramePaletteState
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
				if call.Routine == 0x111ae {
					plan, e := childrenRules.Panel.SwitchProfile(m, call.Frame)
					if e != nil {
						return NativeCommandFrameResult{}, e
					}
					if plan.A0 != 0 {
						call.A[0] = NativeRequesterAddress{Address: plan.A0}
						call.A[1] = NativeRequesterAddress{Address: plan.A1}
					}
					return NativeCommandFrameResult{Complete: true}, nil
				}
				if call.Routine == 0x19cd0 {
					load := NativeResourceFrameState{}
					step, e := load.Advance(&resourceRules, NativeResourceFrameCallbacks{RAM: physical, CodeBase: 0x100000, Frame: call.Frame, Input: &p.Input})
					return NativeCommandFrameResult{Complete: step.Complete}, e
				}
				if call.Routine == 0x102e4 {
					if palette == nil {
						bank := func(a NativeRequesterAddress) (NativeFramePaletteBank, error) {
							v := NativeFramePaletteBank{Address: a.Address}
							for i := range v.Words {
								x, e := cm.Read16(int(a.Address-0x100000) + i*2)
								if e != nil {
									return v, e
								}
								v.Words[i] = x
							}
							return v, nil
						}
						source, e := bank(call.A[2])
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						target, e := bank(call.A[3])
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						palette = NewNativeFramePaletteState(source, target, 0x100000)
					}
					done, e := palette.Advance(p, call.Frame, m)
					off := uint32(0x34)
					if done {
						off = 0x74
						palette = nil
					}
					call.A[0] = NativeRequesterAddress{Address: p.ChipBase + off, Chip: true}
					call.A[1] = NativeRequesterAddress{Address: p.ChipBase + off + 0x200, Chip: true}
					return NativeCommandFrameResult{Complete: done}, e
				}
				if call.Routine != f.Frames[expected].PC {
					return NativeCommandFrameResult{}, fmt.Errorf("unexpected genuine menu child%x", call.Routine)
				}
				*phase++
				return NativeCommandFrameResult{}, nil // Actual unresolved child entry, never a fabricated success.
			}
			check := func() {
				want := f.Frames[expected]
				step, e := state.Advance(&render, cb)
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
					for j := 0; j < (width+1)*(int(binary.BigEndian.Uint16(code[0xab56:]))/8+1); j++ {
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
				for _, wire := range event.Keys {
					if e := p.Input.KeyboardInterrupt(wire); e != nil {
						t.Fatal(e)
					}
				}
				check()
			}
			last := f.Frames[len(f.Frames)-1]
			if strings.HasPrefix(f.Input.Name, "password-valid-") {
				want := []byte{2, 3, 4, 0, 1, 17, 33, 49, 65, 81, 0, 5}
				if !bytes.Equal(last.GodBytes, want) {
					t.Fatalf("native accepted password fields differ: %v", last.GodBytes)
				}
			}
			if strings.HasPrefix(f.Input.Name, "password-invalid-second-face-") {
				first := f.Frames[0].GodBytes
				if last.GodBytes[0] != 2 || last.GodBytes[1] != 3 || last.GodBytes[2] != first[2] || !bytes.Equal(last.GodBytes[3:], first[3:]) {
					t.Fatalf("native partial face rollback differs: %v/%v", last.GodBytes, first)
				}
			}
			if strings.HasPrefix(f.Input.Name, "password-invalid-letters-") && !bytes.Equal(last.GodBytes, f.Frames[0].GodBytes) {
				t.Fatal("native failed letters changed profile")
			}
			if strings.HasPrefix(f.Input.Name, "name-edit-") && string(last.NameBytes[:8]) != "NATIVEAB" {
				t.Fatalf("native name editing differs: %q", last.NameBytes)
			}
			if strings.HasPrefix(f.Input.Name, "name-full-sentinel-") && !bytes.Equal(last.NameBytes, append([]byte("ABCDEFGHIJKLMNOP"), 0, 255)) {
				t.Fatal("native full name sentinel was not retained")
			}
		})
	}
}
