package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type hudInputFixture struct {
	Input struct {
		Name    string
		Entry   int
		D       [8]uint32
		A       [7]uint32
		Patches [][3]uint32
		Clobber bool
	}
	D                 [8]uint32
	A                 [7]uint32
	CCR               uint16
	BSSHash, CodeHash string
	Calls             []struct {
		Routine int
		D       [8]uint32
		A       [7]uint32
		BSSHash string
	}
}

func TestNativeGameplayHUDInputAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/gameplay_hud_input_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []hudInputFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 2463 {
		t.Fatal("native HUDcorpus changed")
	}
	rules, e := DecodeNativeGameplayHUDInputRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range corpus.Cases {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, pending), func(t *testing.T) {
				host, e := NewNativeHunkMemory(testBundle(t).Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
				if e != nil {
					t.Fatal(e)
				}
				ram := host.Memory()
				raw, _ := host.Span(0x200000, 0x11280)
				code, _ := host.Span(0x100000, 0x3fa2c)
				for i := range raw {
					raw[i] = byte(i*13 + (i >> 5) + 11)
				}
				m, cm := commandFrameBacking(raw), commandFrameBacking(code)
				for _, p := range []nativeHeroPatch{{0xeb6a, 4, 0x20eb56}, {0xeb42, 2, 1}, {0xf3a, 2, 0}, {0x140, 2, 1}, {0x142, 2, 0}, {0x146, 2, 0}, {0xf0e, 2, 0}, {0xad, 1, 0}, {0x73, 1, 0}, {0xe8a4, 4, 0xffffffff}} {
					renderFramePatch(m, p)
				}
				for i := 0; i < 36; i++ {
					_ = m.Write8(0xe914+i, 1)
				}
				for _, p := range f.Input.Patches {
					renderFramePatch(m, nativeHeroPatch{Address: int(p[0]), Width: int(p[1]), Value: p[2]})
				}
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeGameplayHUDInputState{Entry: f.Input.Entry}
				for i, v := range f.Input.A {
					state.A[i] = NativeRequesterAddress{Address: v, Absolute: true}
				}
				calls := 0
				cb := NativeGameplayHUDInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Code: cm, Memory: m, RAM: ram, CodeBase: 0x100000, Frame: &frame}}
				cb.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if calls >= len(f.Calls) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpectedHUDchild%x", call.Routine)
					}
					want := f.Calls[calls]
					if call.Routine != want.Routine || frame.D != want.D || fileFrameHash(raw) != want.BSSHash {
						return NativeCommandFrameResult{}, fmt.Errorf("HUDchild%d exactinput differs at%x", calls, call.Routine)
					}
					for i, v := range *call.A {
						if v.Address != want.A[i] {
							return NativeCommandFrameResult{}, fmt.Errorf("HUDchildA%d got%x want%x", i, v.Address, want.A[i])
						}
					}
					if pending && *phase == 0 {
						*phase = 1
						return NativeCommandFrameResult{}, nil
					}
					if f.Input.Clobber {
						for i := range frame.D {
							frame.D[i] ^= 0x567800 + uint32(i)*0x11111
						}
						for i := range *call.A {
							call.A[i].Address ^= 0x10200 + uint32(i)*0x100
						}
					}
					calls++
					return NativeCommandFrameResult{Complete: true}, nil
				}
				var step NativeGameplayHUDInputStep
				for attempts := 0; attempts < 32; attempts++ {
					if f.Input.Entry == 0x14768 {
						e = NativeGameplayHUDCost(&rules, cb, &state.A)
						step = NativeGameplayHUDInputStep{Complete: e == nil, FlagsKnown: true, Zero: frame.D[0] == 0, Negative: false}
					} else if f.Input.Entry == 0x147e0 {
						var allowed bool
						allowed, e = NativeGameplayHUDAdmission(&rules, cb, &state.A)
						_ = allowed
						step = NativeGameplayHUDInputStep{Complete: e == nil, FlagsKnown: true, Zero: uint16(frame.D[4]) == 0, Negative: int16(frame.D[4]) < 0}
					} else if f.Input.Entry == 0x2854 || f.Input.Entry == 0x2940 {
						step, e = RunNativeGameplayHUDScan(f.Input.Entry, cb, &state.A)
					} else {
						step, e = state.Advance(&rules, cb)
					}
					if e != nil {
						t.Fatal(e)
					}
					if step.Complete {
						break
					}
				}
				if !step.Complete || !step.FlagsKnown || step.Zero != (f.CCR&4 != 0) || step.Negative != (f.CCR&8 != 0) {
					t.Fatal("nativeHUDCCR/completion differs")
				}
				if frame.D != f.D {
					t.Fatalf("fullDgot%08x want%08x", frame.D, f.D)
				}
				for i, v := range state.A {
					if v.Address != f.A[i] {
						t.Fatalf("A%d got%x want%x", i, v.Address, f.A[i])
					}
				}
				if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash {
					t.Fatalf("rawHUD BSS%v CODE%v", fileFrameHash(raw) == f.BSSHash, fileFrameHash(code) == f.CodeHash)
				}
				if calls != len(f.Calls) {
					t.Fatal("sourceHUDchild operation missing")
				}
			})
		}
	}
}
