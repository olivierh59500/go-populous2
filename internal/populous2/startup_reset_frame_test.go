package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type startupResetFrameFixture struct {
	Input struct {
		Name                string
		Entry               int
		Mode, Profile, Free uint16
		Transport           uint8
		Startup             int32
		Retries             int
		D                   [8]uint32
		Reset               bool
	}
	Calls []struct {
		Routine           int
		D, AfterD         [8]uint32
		A, AfterA         [7]uint32
		BSSHash, CodeHash string
		Cursor            uint16
	}
	D                 [8]uint32
	A                 [7]uint32
	BSSHash, CodeHash string
	Cursor            uint16
}

func TestNativeStartupResetFramesAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/startup_reset_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []startupResetFrameFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 420 {
		t.Fatalf("startup reset CPU coverage changed:%d", len(catalog.Cases))
	}
	for _, f := range catalog.Cases {
		for _, suspended := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, suspended), func(t *testing.T) {
				raw := make([]byte, 0x11280)
				for i := range raw {
					raw[i] = byte(i*37 + (i >> 5) + 11)
				}
				m := commandFrameBacking(raw)
				code := fileFrameRelocatedCode(t)
				cm := commandFrameBacking(code)
				_ = m.Write16(0xeb44, f.Input.Mode)
				_ = m.Write16(0xeb42, f.Input.Profile)
				_ = m.Write16(0xf0e, f.Input.Free)
				_ = m.Write8(0xeb5e, f.Input.Transport)
				_ = m.Write32(0x22, 0x500408)
				_ = cm.Write16(0xa2a, 0x120)
				c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				a := [7]NativeRequesterAddress{}
				for i := range a {
					a[i] = NativeRequesterAddress{Address: 0x700000 + uint32(i)*0x1000, Absolute: true}
				}
				cb := NativeStartupResetFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &c}
				if f.Input.Reset {
					if e = ResetNativeStartup10F1A(cb, &a); e != nil {
						t.Fatal(e)
					}
				} else {
					s := NativeStartupResetFrameState{Entry: f.Input.Entry, A: a}
					index := 0
					cb.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
						if index >= len(f.Calls) {
							return NativeCommandFrameResult{}, fmt.Errorf("unexpected startup child%x", call.Routine)
						}
						want := f.Calls[index]
						if call.Routine != want.Routine || call.Frame.D != want.D {
							return NativeCommandFrameResult{}, fmt.Errorf("startup child%d registers differ:got%x/%08x want%x/%08x", index, call.Routine, call.Frame.D, want.Routine, want.D)
						}
						for i, v := range *call.A {
							if v.Address != want.A[i] {
								return NativeCommandFrameResult{}, fmt.Errorf("startup child%d A%d differs:%x/%x", index, i, v.Address, want.A[i])
							}
						}
						cursor, e := cm.Read16(0xa2a)
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						if fileFrameHash(raw) != want.BSSHash || cursor != want.Cursor || fileFrameHash(code) != want.CodeHash {
							return NativeCommandFrameResult{}, fmt.Errorf("startup child%d complete BSS/CODE prefix differs", index)
						}
						if suspended && *phase == 0 {
							*phase = 1
							return NativeCommandFrameResult{}, nil
						}
						*phase = 0
						call.Frame.D = want.AfterD
						for i, v := range want.AfterA {
							(*call.A)[i] = NativeRequesterAddress{Address: v, Absolute: true}
						}
						if e = m.Write16(0xf0e, uint16(call.Routine)); e != nil {
							return NativeCommandFrameResult{}, e
						}
						if e = m.Write8(0xc810+index%7, uint8(call.Routine)); e != nil {
							return NativeCommandFrameResult{}, e
						}
						if e = cm.Write16(0xa2a, uint16(call.Routine)); e != nil {
							return NativeCommandFrameResult{}, e
						}
						index++
						return NativeCommandFrameResult{Complete: true, Zero: call.Frame.D[0] == 0}, nil
					}
					for turn := 0; ; turn++ {
						if turn > len(f.Calls)+2 {
							t.Fatal("startup child did not resume")
						}
						step, e := s.Advance(cb)
						if e != nil {
							t.Fatal(e)
						}
						if step.Complete {
							break
						}
						if !step.Waiting {
							t.Fatal("startup suspension did not retain real boundary")
						}
					}
					if index != len(f.Calls) {
						t.Fatal("source startup child missing")
					}
					a = s.A
				}
				cursor, e := cm.Read16(0xa2a)
				if e != nil {
					t.Fatal(e)
				}
				if c.D != f.D {
					t.Fatalf("source startup registers differ:got%08x want%08x", c.D, f.D)
				}
				for i, v := range a {
					if v.Address != f.A[i] {
						t.Fatalf("source startup A%d differs:%x/%x", i, v.Address, f.A[i])
					}
				}
				if fileFrameHash(raw) != f.BSSHash || cursor != f.Cursor || fileFrameHash(code) != f.CodeHash {
					t.Fatal("source startup complete BSS/CODE result differs")
				}
			})
		}
	}
}

func TestNativeStartupMissingChildDoesNotCompleteOrReplayReset(t *testing.T) {
	raw := make([]byte, 0x11280)
	binary.BigEndian.PutUint16(raw[0xeb44:], 2)
	binary.BigEndian.PutUint16(raw[0xf0e:], 9)
	c := NativeFrameRegisterContext{D: [8]uint32{0x12340001, 1, 2, 3, 4, 5, 6, 7}}
	cb := NativeStartupResetFrameCallbacks{Code: commandFrameBacking(fileFrameRelocatedCode(t)), Memory: commandFrameBacking(raw), Frame: &c}
	s := NativeStartupResetFrameState{Entry: 0x10ad8}
	step, e := s.Advance(cb)
	if e == nil || step.Complete || s.PC != 0x10aee || binary.BigEndian.Uint16(raw[0xf0e:]) != 0 {
		t.Fatal("missing constructor child fabricated completion or lost nativeprefix")
	}
	before, d := fileFrameHash(raw), c.D
	step, again := s.Advance(cb)
	if again != e || step.Complete || fileFrameHash(raw) != before || c.D != d {
		t.Fatal("failed startup child replayed an earlier source operation")
	}
}
