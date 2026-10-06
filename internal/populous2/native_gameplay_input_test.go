package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type gameplayInputFixture struct {
	Input struct {
		Name     string
		D        [8]uint32
		A        [7]uint32
		Patches  []nativeHeroPatch
		Seed     uint32
		Zero     bool
		Preserve bool
	}
	D                 [8]uint32
	A                 [7]uint32
	Exit              int
	BSSHash, CodeHash string
	Calls             []struct {
		Routine int
		D       [8]uint32
		A       [7]uint32
		BSSHash string
	}
}

func TestNativeGameplayInputControllerAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/gameplay_input_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []gameplayInputFixture }
	if err = json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 602 {
		t.Fatal("gameplay input source corpus incomplete", err)
	}
	keys, err := DecodeNativeInputRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range corpus.Cases {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, pending), func(t *testing.T) {
				host := nativeHostTestMemory(t)
				ram := host.Memory()
				bss := nativeOffsetMemory(ram, 0x200000)
				code := nativeOffsetMemory(ram, 0x100000)
				input := NativeInputState{}
				m := input.Memory(bss)
				physical := ram
				physical.Read8 = func(at int) (uint8, error) {
					if at >= 0x200000 && at < 0x211280 {
						return m.Read8(at - 0x200000)
					}
					return ram.Read8(at)
				}
				physical.Write8 = func(at int, v uint8) error {
					if at >= 0x200000 && at < 0x211280 {
						return m.Write8(at-0x200000, v)
					}
					return ram.Write8(at, v)
				}
				physical = nativeByteAddressMemory(physical.Read8, physical.Write8)
				for _, p := range []nativeHeroPatch{{0xeb6a, 4, 0x20eb56}, {0xf0c, 2, 23}, {0x138, 2, 319}, {0x13a, 2, 199}, {0x5f44, 2, 24}, {0x5f46, 2, 25}, {0xeb18, 2, 2}, {0xeb42, 2, 1}} {
					renderFramePatch(m, p)
				}
				if err := code.Write32(0xe458, 0x00c00048); err != nil {
					t.Fatal(err)
				}
				for _, p := range f.Input.Patches {
					renderFramePatch(m, p)
				}
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeGameplayInputState{}
				for i, v := range f.Input.A {
					state.A[i] = NativeRequesterAddress{Address: v, Absolute: true}
				}
				calls := 0
				cb := NativeGameplayInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Code: code, Memory: m, RAM: physical, CodeBase: 0x100000, Frame: &frame}, Input: &input, Keys: keys}
				cb.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if calls >= len(f.Calls) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected child%x", call.Routine)
					}
					want := f.Calls[calls]
					if call.Routine != want.Routine || call.Frame.D != want.D || fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash {
						return NativeCommandFrameResult{}, fmt.Errorf("child%d source input differs at%x: D%08x/%08x", calls, call.Routine, call.Frame.D, want.D)
					}
					for i, a := range *call.A {
						if a.Address != want.A[i] {
							return NativeCommandFrameResult{}, fmt.Errorf("child A%d differs%x/%x", i, a.Address, want.A[i])
						}
					}
					if pending && *phase == 0 {
						*phase = 1
						return NativeCommandFrameResult{}, nil
					}
					for i := 0; i < len(call.Frame.D) && !f.Input.Preserve; i++ {
						call.Frame.D[i] ^= f.Input.Seed + uint32(i)*0x111111
					}
					for i := 0; i < len(*call.A) && !f.Input.Preserve; i++ {
						call.A[i].Address ^= (f.Input.Seed & 255) + uint32(i)*0x10
					}
					calls++
					return NativeCommandFrameResult{Complete: true, Zero: f.Input.Zero}, nil
				}
				var step NativeGameplayInputStep
				for resume := 0; resume < 32 && !step.Complete; resume++ {
					step, err = state.Advance(cb)
					if err != nil {
						t.Fatal(err)
					}
				}
				if !step.Complete || frame.D != f.D || calls != len(f.Calls) || step.ExitRequested != (f.Exit == 0x1bda) {
					t.Fatalf("source completion differs:%+v D%08x/%08x children%d/%d", step, frame.D, f.D, calls, len(f.Calls))
				}
				for i, a := range state.A {
					if a.Address != f.A[i] {
						t.Fatalf("terminal A%d differs%x/%x", i, a.Address, f.A[i])
					}
				}
				if fileFrameHash(fileFrameMemoryBytes(t, m)) != f.BSSHash {
					t.Fatal("source final BSS differs")
				}
				bytes, err := host.Span(0x100000, 0x3fa2c)
				if err != nil {
					t.Fatal(err)
				}
				if fileFrameHash(bytes) != f.CodeHash {
					t.Fatal("source final CODE differs")
				}
			})
		}
	}
}

func TestNativeGameplayInputMissingChildRetainsPrefix(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if err := h.Memory.BSS.Write16(0x132, 1); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write8(0x32+0x5f, 1); err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	state := NativeGameplayInputState{}
	_, err := state.Advance(NativeGameplayInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &frame, Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase}, Input: &h.Session.Presentation.Input})
	if err == nil || state.Finished || state.PC != 0x11dc {
		t.Fatal("missing sound/panel child acknowledged", state.PC, err)
	}
	if frame.D[0] != 0x1cc {
		t.Fatal("source prefix did not preserve assigned cue register")
	}
}
