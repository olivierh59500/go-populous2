package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type startupPreludeFixture struct {
	Input struct {
		Name                string
		Entry               int
		Mode, Profile, Free uint16
		Transport           uint8
		Thresholds          [3]uint16
		Startup             int32
		D                   [8]uint32
	}
	Calls []struct {
		Routine                         int
		D, AfterD                       [8]uint32
		A, AfterA                       [7]uint32
		BSSHash, CodeHash, GraphicsHash string
		Cursor                          uint16
	}
	CCR                             uint16
	D                               [8]uint32
	A                               [7]uint32
	BSSHash, CodeHash, GraphicsHash string
	Cursor                          uint16
	Hardware                        []NativeFrameHardwareWrite
}

func TestNativeInitialStartupPreludeAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/startup_reset_frame_prelude_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []startupPreludeFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 36 {
		t.Fatalf("initial startup corpus changed:%d", len(catalog.Cases))
	}
	base := resourceFrameInitialRAM(t)
	copy(base[0x600000:], testBundle(t).Raw["qaz.pak"])
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			ram := append([]byte(nil), base...)
			raw := ram[0x200000:0x211280]
			for i := range raw {
				raw[i] = byte(i*37 + (i >> 5) + 11)
			}
			code := ram[0x100000:0x13fa2c]
			m, cm := commandFrameBacking(raw), commandFrameBacking(code)
			_ = m.Write16(0xeb44, f.Input.Mode)
			_ = m.Write16(0xeb42, f.Input.Profile)
			_ = m.Write16(0xf0e, f.Input.Free)
			_ = m.Write8(0xeb5e, f.Input.Transport)
			_ = m.Write32(0x22, 0x500408)
			_ = m.Write32(0xdbe, 0x501408)
			_ = cm.Write16(0xa2a, 0x120)
			for i, v := range f.Input.Thresholds {
				_ = cm.Write16(0x1a590+i*2, v)
			}
			c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			s := NativeStartupResetFrameState{Entry: 0x10a10}
			for i := range s.A {
				s.A[i] = NativeRequesterAddress{Address: 0x700000 + uint32(i)*0x1000, Absolute: true}
			}
			hardware := []NativeFrameHardwareWrite{}
			index := 0
			cb := NativeStartupResetFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, RAM: commandFrameBacking(ram), Frame: &c, Hardware: func(w NativeFrameHardwareWrite) error { hardware = append(hardware, w); return nil }}
			cb.Call = func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
				if index >= len(f.Calls) {
					return NativeCommandFrameResult{}, fmt.Errorf("unexpected initial startup child%x", call.Routine)
				}
				want := f.Calls[index]
				if call.Routine != want.Routine || call.Frame.D != want.D {
					return NativeCommandFrameResult{}, fmt.Errorf("initial child%d D differs:got%x/%08x want%x/%08x", index, call.Routine, call.Frame.D, want.Routine, want.D)
				}
				for i, v := range *call.A {
					if v.Address != want.A[i] {
						return NativeCommandFrameResult{}, fmt.Errorf("initial child%d A%d differs:%x/%x", index, i, v.Address, want.A[i])
					}
				}
				cursor, e := cm.Read16(0xa2a)
				if e != nil {
					return NativeCommandFrameResult{}, e
				}
				if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(ram[0x600000:0x61e0dc]) != want.GraphicsHash || cursor != want.Cursor {
					return NativeCommandFrameResult{}, fmt.Errorf("initial child%d complete RAM prefix differs", index)
				}
				call.Frame.D = want.AfterD
				for i, v := range want.AfterA {
					(*call.A)[i] = NativeRequesterAddress{Address: v, Absolute: true}
				}
				_ = m.Write16(0xf0e, uint16(call.Routine))
				_ = m.Write8(0xc810+index%7, uint8(call.Routine))
				_ = cm.Write16(0xa2a, uint16(call.Routine))
				index++
				return NativeCommandFrameResult{Complete: true, Zero: call.Frame.D[0] == 0}, nil
			}
			step, e := s.Advance(cb)
			if e != nil {
				t.Fatal(e)
			}
			if !step.Complete || c.D != f.D || index != len(f.Calls) {
				t.Fatal("initial startup completion/registers/children differ")
			}
			if !step.FlagsKnown || step.Zero != (f.CCR&4 != 0) || step.Negative != (f.CCR&8 != 0) {
				t.Fatal("source initial startup terminal CCR differs")
			}
			for i, a := range s.A {
				if a.Address != f.A[i] {
					t.Fatal("initial startup address registers differ")
				}
			}
			if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(ram[0x600000:0x61e0dc]) != f.GraphicsHash || binary.BigEndian.Uint16(code[0xa2a:]) != f.Cursor {
				t.Fatal("initial startup complete RAM differs")
			}
			if !reflect.DeepEqual(hardware, f.Hardware) {
				t.Fatalf("initial source hardware writes differ:%+v/%+v", hardware, f.Hardware)
			}
			// Completed retained state cannot prepare/transcode ICONS a second time.
			graphics, registers := fileFrameHash(ram[0x600000:0x61e0dc]), c.D
			step, e = s.Advance(cb)
			if e != nil || !step.Complete || graphics != fileFrameHash(ram[0x600000:0x61e0dc]) || c.D != registers || len(hardware) != 1 {
				t.Fatal("initial source prelude repeated after completion")
			}
		})
	}
}
