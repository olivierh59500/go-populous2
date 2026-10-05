package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type startupConstructionFixture struct {
	Input struct {
		Name                 string
		Entry                int
		Mode, Profile, World uint16
		Transport            uint8
		D                    [8]uint32
		Record               int
		Cancel               bool
	}
	Calls []struct {
		Routine           int
		D, AfterD         [8]uint32
		A, AfterA         [7]uint32
		BSSHash, CodeHash string
	}
	D                 [8]uint32
	A                 [7]uint32
	CCR               uint16
	BSSHash, CodeHash string
}

func TestNativeStartupConstructionAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/startup_reset_frame_construction_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []startupConstructionFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 336 {
		t.Fatalf("native constructor corpus changed:%d", len(catalog.Cases))
	}
	bundle := testBundle(t)
	base := resourceFrameInitialRAM(t)
	copy(base[0x300000:], bundle.Raw["conquest.pak"])
	rr, e := DecodeNativeResourceFrameRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		for _, suspended := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, suspended), func(t *testing.T) {
				ram := append([]byte(nil), base...)
				raw := ram[0x200000:0x211280]
				for i := range raw {
					raw[i] = byte(i*13 + (i >> 4) + 23)
				}
				code := ram[0x100000:0x13fa2c]
				m, cm := commandFrameBacking(raw), commandFrameBacking(code)
				_ = m.Write16(0xeb44, f.Input.Mode)
				_ = m.Write16(0xeb42, f.Input.Profile)
				_ = m.Write16(0xeb46, f.Input.World)
				_ = m.Write8(0xeb5e, f.Input.Transport)
				_ = m.Write16(0xeb22, 2)
				_ = m.Write16(0xeb2a, 0x7123)
				_ = m.Write32(0x3ac, 1<<12)
				_ = cm.Write16(0xa2a, 0x120)
				record := append([]byte(nil), code[0x20630:0x2072a]...)
				if f.Input.Record != 0 {
					for i := range record {
						record[i] = byte(i*17 + f.Input.Record*31)
					}
					record[182], record[183] = 0, 2
				}
				copy(code[0x20630:], record)
				copy(code[0x20536:], record)
				c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				a := [7]NativeRequesterAddress{}
				for i := range a {
					a[i] = NativeRequesterAddress{Address: 0x700000 + uint32(i)*0x1000, Absolute: true}
				}
				cb := NativeStartupResetFrameCallbacks{Code: cm, Memory: m, RAM: commandFrameBacking(ram), CodeBase: 0x100000, Frame: &c}
				index := 0
				var power *NativeStartupPowerFrameState
				cb.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if call.Routine == 0x11078 {
						if power == nil {
							power = &NativeStartupPowerFrameState{A: *call.A}
						}
						nested := cb
						nested.Frame = call.Frame
						step, e := power.Advance(nested)
						*call.A = power.A
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						if !step.Complete {
							return NativeCommandFrameResult{}, nil
						}
						power = nil
						return NativeCommandFrameResult{Complete: true}, nil
					}
					if index >= len(f.Calls) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected constructor child%x", call.Routine)
					}
					want := f.Calls[index]
					if call.Routine != want.Routine || call.Frame.D != want.D {
						return NativeCommandFrameResult{}, fmt.Errorf("constructor child%d D differs:%x/%08x expected%x/%08x", index, call.Routine, call.Frame.D, want.Routine, want.D)
					}
					for i, v := range *call.A {
						if v.Address != want.A[i] {
							return NativeCommandFrameResult{}, fmt.Errorf("constructor child%d A%d differs:%x/%x", index, i, v.Address, want.A[i])
						}
					}
					if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash {
						return NativeCommandFrameResult{}, fmt.Errorf("constructor child%d complete RAM prefix differs", index)
					}
					if suspended && *phase == 0 {
						*phase = 1
						return NativeCommandFrameResult{}, nil
					}
					*phase = 0
					if call.Routine == 0x19cd0 {
						input, e := NewNativeInputState(bundle.Executable)
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						input.Mouse.Image = binary.BigEndian.Uint16(code[0xa2a:])
						rs := NativeResourceFrameState{}
						step, e := rs.Advance(&rr, NativeResourceFrameCallbacks{RAM: cb.RAM, CodeBase: cb.CodeBase, Frame: call.Frame, Input: &input})
						if e != nil || !step.Complete {
							return NativeCommandFrameResult{}, fmt.Errorf("actual cached conquest child:%v", e)
						}
						if call.Frame.D != want.AfterD {
							return NativeCommandFrameResult{}, fmt.Errorf("cached source resource D differs")
						}
					} else {
						call.Frame.D = want.AfterD
						for i, v := range want.AfterA {
							(*call.A)[i] = NativeRequesterAddress{Address: v, Absolute: true}
						}
						if call.Routine == 0x11044 {
							copy(code[0x20536:], record)
							copy(raw[0xe8fe:], record[:58])
							copy(raw[0xea38:], record[58:116])
						}
					}
					index++
					return NativeCommandFrameResult{Complete: true, Zero: call.Routine == 0x3cba && f.Input.Cancel}, nil
				}
				switch f.Input.Entry {
				case 0x10df2:
					if e = LoadNativeStartupTemplates(cb, &a); e != nil {
						t.Fatal(e)
					}
				case 0x10e90:
					if e = CompileNativeStartupChoices(cb, &a); e != nil {
						t.Fatal(e)
					}
				case 0x11078:
					s := NativeStartupPowerFrameState{A: a}
					for turn := 0; ; turn++ {
						if turn > 4 {
							t.Fatal("power resource did not resume")
						}
						step, e := s.Advance(cb)
						if e != nil {
							t.Fatal(e)
						}
						if step.Complete {
							break
						}
					}
					a = s.A
				case 0x10d6a:
					s := NativeStartupConstructionFrameState{A: a}
					for turn := 0; ; turn++ {
						if turn > 8 {
							t.Fatal("constructor children did not resume")
						}
						step, e := s.Advance(cb)
						if e != nil {
							t.Fatal(e)
						}
						if step.Complete {
							if !step.FlagsKnown || step.Zero != (f.CCR&4 != 0) || step.Negative != (f.CCR&8 != 0) {
								t.Fatal("constructor terminal CCR differs")
							}
							break
						}
					}
					a = s.A
				default:
					t.Fatal("unexpected construction entry")
				}
				if index != len(f.Calls) || c.D != f.D {
					t.Fatalf("constructor source child/register outputs differ:%08x/%08x", c.D, f.D)
				}
				for i, v := range a {
					if v.Address != f.A[i] {
						t.Fatalf("constructor A%d differs:%x/%x", i, v.Address, f.A[i])
					}
				}
				if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash {
					t.Fatal("constructor completeBSS/CODE outputs differ")
				}
			})
		}
	}
}
