package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type campaignChildrenComposedSnapshot struct {
	campaignSelectionSnapshot
}
type campaignChildrenComposedFixture struct {
	Input  campaignSelectionInput
	Frames []campaignChildrenComposedSnapshot
}

func TestNativeCampaignSelectionRealChildrenAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/campaign_selection_children_composed_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Cases []campaignChildrenComposedFixture
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 12 {
		t.Fatalf("native selection corpus changed%d", len(corpus.Cases))
	}
	bundle := testBundle(t)
	childRules, _ := DecodeNativeCampaignSelectionChildrenRules(bundle.Executable)
	rules, e := DecodeNativeCampaignSelectionFrameRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range corpus.Cases {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, pending), func(t *testing.T) {
				p, e := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0x408; i < len(p.Chip); i++ {
					p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
				}
				p.InterruptChain = false
				if _, e = p.Initialize(bundle.Executable, NativeMouseSample{}); e != nil {
					t.Fatal(e)
				}
				host, e := NewNativeHunkMemory(bundle.Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
				if e != nil {
					t.Fatal(e)
				}
				ram := host.Memory()
				faceptr, _ := ram.Read32(0x1212ba)
				faces, _ := host.Span(faceptr, len(bundle.Raw["faces.pak"]))
				copy(faces, bundle.Raw["faces.pak"])
				if e = PrepareNativeResourceFramePlanes(ram, 0x1212ba, 0x2003be); e != nil {
					t.Fatal(e)
				}
				if e = PrepareNativeResourceFramePlanes(ram, 0x1214b2, 0x2003be); e != nil {
					t.Fatal(e)
				}
				preparedPointer, _ := host.Span(0x400000, 15260)
				copy(p.PointerData[:15260], preparedPointer)
				code, _ := host.Span(0x100000, 0x3fa2c)
				if e = host.MapRegion(NativeHostRegion{Name: "Campaign", Base: 0x800000, Bytes: append([]byte(nil), bundle.Raw["conquest.pak"]...)}); e != nil {
					t.Fatal(e)
				}
				large, _ := host.Span(0x600000, len(bundle.Raw["s32-0.pak"]))
				copy(large, bundle.Raw["s32-0.pak"])
				small, _ := host.Span(0x6118c8, len(bundle.Raw["s16-0.pak"]))
				copy(small, bundle.Raw["s16-0.pak"])
				blockAllocation := make([]byte, 0x1e0dc)
				copy(blockAllocation, bundle.Raw["block0.pak"])
				if e = host.MapRegion(NativeHostRegion{Name: "BLOCK", Base: 0x900000, Bytes: blockAllocation}); e != nil {
					t.Fatal(e)
				}
				if e = PrepareNativeResourceFramePlanes(ram, 0x121632, 0x2003be); e != nil {
					t.Fatal(e)
				}
				if e = PrepareNativeResourceFrameBlocks(ram, 0x900000, 0x900000+uint32(len(bundle.Raw["block0.pak"])), 0x2003be); e != nil {
					t.Fatal(e)
				}
				_ = ram.Write32(0x11a502, 0x800000)
				_ = ram.Write32(0x1054a4, 0x900000)
				_ = ram.Write32(0x105406, 0x900000)
				raw := make([]byte, 0x11280)
				m := p.Memory(commandFrameBacking(raw))
				cm := commandFrameBacking(code)
				for i, v := range []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY} {
					_ = cm.Write16(0xa2a+i*2, v)
				}
				_ = cm.Write16(0x4468, 0)
				_ = cm.Write16(0x3f90, 0)
				_ = cm.Write8(0x4440, 0)
				_ = cm.Write8(0x4416, 0)
				_ = cm.Write16(0x3ea, 0)
				_ = cm.Write32(0x77a, p.CopperSelector)
				_ = cm.Write32(0x77e, p.SpritePatchPointer)
				_ = m.Write16(0x3b0, f.Input.DialogFlag)
				_ = m.Write32(0x3ac, 1<<12|1<<8|1<<4|1<<0)
				_ = m.Write32(0xf40, 10)
				_ = m.Write16(0xeb46, f.Input.World)
				_ = m.Write16(0xeb22, f.Input.Land)
				_ = m.Write16(0xe90a, 0x155)
				for i := 0; i < 36; i++ {
					v := byte(1)
					if f.Input.Pattern == 1 {
						v = 0
					}
					if f.Input.Pattern == 2 {
						v = byte(i%5 - 2)
					}
					_ = m.Write8(0xe914+i, v)
				}
				_ = m.Write16(0xeb42, f.Input.Profile)
				_ = m.Write16(0xeb44, f.Input.GameMode)
				_ = m.Write16(int(uint32(int64(0xe76a)+int64(int16(uint32(f.Input.Profile)*314))))+0x1a, f.Input.ControlMode)
				_ = m.Write32(0xeb6a, 0x200000+0xeb56)
				_ = m.Write32(0xf32, 0xc930)
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeCampaignSelectionFrameState{}
				var palette *NativeFramePaletteState
				children := NativeCampaignSelectionChildren{}
				blitter := NativeCampaignBlitterState{}
				expected := 0
				calls, sounds := 0, 0
				physical := ram
				physical.Write16 = func(at int, v uint16) error {
					if at >= int(p.ChipBase) && at < int(p.ChipBase)+len(p.Chip) {
						binary.BigEndian.PutUint16(p.Chip[at-int(p.ChipBase):], v)
						return nil
					}
					if at >= 0x200000 && at < 0x211280 {
						return m.Write16(at-0x200000, v)
					}
					return ram.Write16(at, v)
				}
				physical.Write32 = func(at int, v uint32) error {
					if at >= int(p.ChipBase) && at < int(p.ChipBase)+len(p.Chip) {
						binary.BigEndian.PutUint32(p.Chip[at-int(p.ChipBase):], v)
						return nil
					}
					if at >= 0x200000 && at < 0x211280 {
						return m.Write32(at-0x200000, v)
					}
					return ram.Write32(at, v)
				}
				physical.Read8 = func(at int) (byte, error) {
					if at >= 0x200000 && at < 0x211280 {
						return m.Read8(at - 0x200000)
					}
					return ram.Read8(at)
				}
				physical.Read16 = func(at int) (uint16, error) {
					if at >= int(p.ChipBase) && at < int(p.ChipBase)+len(p.Chip) {
						return binary.BigEndian.Uint16(p.Chip[at-int(p.ChipBase):]), nil
					}
					if at >= 0x200000 && at < 0x211280 {
						return m.Read16(at - 0x200000)
					}
					return ram.Read16(at)
				}
				physical.Read32 = func(at int) (uint32, error) {
					if at >= 0x200000 && at < 0x211280 {
						return m.Read32(at - 0x200000)
					}
					return ram.Read32(at)
				}
				cb := NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: func(at uint32) ([]byte, error) {
					off := int(at - p.ChipBase)
					if off < 0 || off > len(p.Chip)-32000 {
						return nil, fmt.Errorf("screen unavailable")
					}
					return p.Chip[off : off+32000], nil
				}, Sound: func(value uint16, _ *NativeFrameRegisterContext) error {
					want := f.Frames[expected].Sounds
					if sounds >= len(want) || value != want[sounds] {
						return fmt.Errorf("actual sound differs")
					}
					sounds++
					return nil
				}}, RAM: physical, Child: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					want := f.Frames[expected].Calls
					if call.Routine == 0x111ae || call.Routine == 0xaf82 || call.Routine == 0x517a {
						childCB := NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: func(at uint32) ([]byte, error) { off := int(at - p.ChipBase); return p.Chip[off : off+32000], nil }, Sound: func(value uint16, _ *NativeFrameRegisterContext) error { sounds++; return nil }}, RAM: physical}, Blitter: &blitter, AudioCommand: func(_, _ uint16, input uint32) (uint32, error) { return input, nil }, AudioControl: NativeAudioControlFrameCallbacks{Command: func(_, _ uint16, input uint32) (uint32, error) { return input, nil }}}, ResourceIO: func(NativeResourceFrameIOCall, *uint32) (NativeResourceFrameIOResult, error) {
							return NativeResourceFrameIOResult{}, fmt.Errorf("unexpected uncached resourceIO")
						}}
						return children.Call(&childRules, childCB, call, phase)
					}

					if call.Routine == 0x102e4 && f.Input.RealPalette {
						if palette == nil {
							bank := func(address uint32) (NativeFramePaletteBank, error) {
								v := NativeFramePaletteBank{Address: address}
								for i := range v.Words {
									x, e := cm.Read16(int(address-0x100000) + i*2)
									if e != nil {
										return v, e
									}
									v.Words[i] = x
								}
								return v, nil
							}
							source, e := bank(call.A[2].Address)
							if e != nil {
								return NativeCommandFrameResult{}, e
							}
							target, e := bank(call.A[3].Address)
							if e != nil {
								return NativeCommandFrameResult{}, e
							}
							palette = NewNativeFramePaletteState(source, target, 0x100000)
						}
						done, e := palette.Advance(p, &frame, m)
						if done {
							call.A[0] = NativeRequesterAddress{Address: p.ChipBase + 0x74, Chip: true}
							call.A[1] = NativeRequesterAddress{Address: p.ChipBase + 0x274, Chip: true}
							palette = nil
						} else {
							call.A[0] = NativeRequesterAddress{Address: p.ChipBase + 0x34, Chip: true}
							call.A[1] = NativeRequesterAddress{Address: p.ChipBase + 0x234, Chip: true}
						}
						return NativeCommandFrameResult{Complete: done}, e
					}
					if call.Routine == 0x19cd0 {
						resource := NativeResourceFrameState{}
						rr, e := DecodeNativeResourceFrameRules(bundle.Executable)
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						wrapped := physical
						wrapped.Write16 = func(at int, v uint16) error {
							if at >= 0x200000 && at < 0x211280 {
								return m.Write16(at-0x200000, v)
							}
							return ram.Write16(at, v)
						}
						wrapped.Write32 = func(at int, v uint32) error {
							if at >= 0x200000 && at < 0x211280 {
								return m.Write32(at-0x200000, v)
							}
							return ram.Write32(at, v)
						}
						step, e := resource.Advance(&rr, NativeResourceFrameCallbacks{RAM: wrapped, CodeBase: 0x100000, Frame: &frame, Input: &p.Input})
						return NativeCommandFrameResult{Complete: step.Complete}, e
					}
					if calls >= len(want) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected child%x", call.Routine)
					}
					w := want[calls]
					if call.Routine != w.Routine || frame.D != w.D {
						return NativeCommandFrameResult{}, fmt.Errorf("child fullD differs")
					}
					for i, v := range *call.A {
						if v.Address != w.A[i] {
							return NativeCommandFrameResult{}, fmt.Errorf("childA%d got%x want%x", i, v.Address, w.A[i])
						}
					}
					if pending && *phase == 0 {
						*phase = 1
						return NativeCommandFrameResult{}, nil
					}
					calls++
					if call.Routine == 0x111ae {
						_ = m.Write16(0xeb42, uint16(frame.D[0]))
					}
					return NativeCommandFrameResult{Complete: true}, nil
				}}
				check := func() {
					step, e := state.Advance(&rules, cb)
					for resumes := 0; state.ChildActive && state.ChildRoutine != 0xaf82 && state.ChildRoutine != 0x517a && !(f.Input.RealPalette && state.ChildRoutine == 0x102e4) && resumes < 32; resumes++ {
						if e != nil {
							break
						}
						step, e = state.Advance(&rules, cb)
					}
					if e != nil {
						t.Fatal(e)
					}
					want := f.Frames[expected]
					pc := step.PC
					if state.ChildActive && state.ChildRoutine == 0xaf82 {
						pc = children.Opponent.PC
					}
					if state.ChildActive && state.ChildRoutine == 0x517a {
						pc = children.Help.PC
					}
					if state.ChildActive && state.ChildRoutine == 0x102e4 && f.Input.RealPalette {
						pc = 0x786
					}
					if step.Complete {
						if !step.FlagsKnown || step.Zero != (want.CCR&4 != 0) || step.Negative != (want.CCR&8 != 0) {
							t.Fatal("native selection terminalCCR changed")
						}
						pc = 0
					}
					if pc != want.PC && !(want.PC == 0x786 && (pc == 0x3d9a || pc == 0x3e64 || pc == 0xb08c || pc == 0xb098 || pc == 0x51dc)) {
						t.Fatalf("PC%x want%x", pc, want.PC)
					}
					if frame.D != want.D {
						t.Fatalf("Dgot%08x want%08x", frame.D, want.D)
					}
					for i, v := range state.A {
						if v.Address != want.A[i] {
							t.Fatalf("A%d got%x want%x", i, v.Address, want.A[i])
						}
					}
					if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(p.Chip) != want.ChipHash || fileFrameHash(p.PointerData[:15260]) != want.PointerHash {
						t.Fatalf("stage%d BSS%v CODE%v chip%v pointer%v", expected, fileFrameHash(fileFrameMemoryBytes(t, m)) == want.BSSHash, fileFrameHash(code) == want.CodeHash, fileFrameHash(p.Chip) == want.ChipHash, fileFrameHash(p.PointerData[:15260]) == want.PointerHash)
					}
					if calls != len(want.Calls) || sounds != len(want.Sounds) {
						t.Fatal("native child/audio operation missing")
					}
					calls, sounds = 0, 0
				}
				check()
				irq := func(x, y uint8, left bool) {
					if _, e := p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame); e != nil {
						t.Fatal(e)
					}
					vals := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
					for i, v := range vals {
						_ = cm.Write16(0xa2a+i*2, v)
					}
				}
				for i, event := range f.Input.Events {
					expected = i + 1
					if event.Action > 0 || event.HasPower {
						start := 0xab4e + int(int16(binary.BigEndian.Uint16(code[0xab4e:])))
						width := int(binary.BigEndian.Uint16(code[0xab54:]))
						count := 0
						found := false
						x, y := 0, 0
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
						if event.HasPower {
							row, col := event.Power/6, event.Power%6
							x = 30 + row*32 + col*16 + 16
							y = 30 + col*8 + 8
							found = true
						}
						if !found {
							t.Fatal("action missing")
						}
						for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
							dx := max(-100, min(100, x*2-int(p.Input.Mouse.PositionX)))
							dy := max(-100, min(100, y*2-int(p.Input.Mouse.PositionY)))
							irq(uint8(int(p.Input.Mouse.CounterX)+dx), uint8(int(p.Input.Mouse.CounterY)+dy), false)
						}
						irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), true)
					} else if event.VBlank {
						irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), false)
					}
					for _, key := range event.Keys {
						_ = cm.Write32(0x68a, 0x100000+uint32(f.Frames[expected-1].PC))
						if e := p.Input.KeyboardInterrupt(key); e != nil {
							t.Fatal(e)
						}
					}
					check()
				}
			})
		}
	}
}
