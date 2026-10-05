package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type campaignRecordFixture struct {
	World, Seed       int
	InputD, D         [8]uint32
	InputA, A         [7]uint32
	CCR               uint16
	BSSHash, CodeHash string
	Calls             [][8]uint32
}

func TestNativeCampaignRecordFrameAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/campaign_frame_record_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []campaignRecordFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 3000 {
		t.Fatal("native campaign corpus changed")
	}
	bundle := testBundle(t)
	rules, e := DecodeNativeResourceFrameRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range corpus.Cases {
		t.Run(fmt.Sprintf("world%d-context%d", f.World, f.Seed), func(t *testing.T) {
			host, e := NewNativeHunkMemory(bundle.Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
			if e != nil {
				t.Fatal(e)
			}
			ram := host.Memory()
			raw, e := host.Span(0x200000, 0x11280)
			if e != nil {
				t.Fatal(e)
			}
			code, e := host.Span(0x100000, 0x3fa2c)
			if e != nil {
				t.Fatal(e)
			}
			campaign, e := host.Span(0x600000, 50000)
			if e != nil {
				t.Fatal(e)
			}
			copy(campaign, bundle.Raw["conquest.pak"])
			for i := range raw {
				raw[i] = byte(i*11 + f.Seed*31 + 17)
			}
			cm := commandFrameBacking(code)
			m := commandFrameBacking(raw)
			_ = cm.Write32(0x1a502, 0x600000)
			_ = cm.Write16(0xa2a, uint16(f.Seed*0x120))
			_ = m.Write32(0x3ac, 1<<12)
			for side := 0; side < 2; side++ {
				_ = m.Write16(0xe8be+side*314, []uint16{2, 4, 18}[(f.World+f.Seed+side)%3])
			}
			_ = m.Write16(0xeb44, []uint16{2, 4, 6}[(f.World+f.Seed)%3])
			_ = m.Write16(0xeb42, uint16(f.World%2+1))
			_ = m.Write16(0xeb46, uint16(f.World))
			frame := NativeFrameRegisterContext{D: f.InputD, AddressBase: 0x200000}
			state := NativeCampaignRecordFrameState{}
			for i, v := range f.InputA {
				state.A[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			input := NativeInputState{}
			input.Mouse.Image = uint16(f.Seed * 0x120)
			resource := NativeResourceFrameState{}
			calls := 0
			cb := NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame}, RAM: ram, Child: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if call.Routine != 0x19cd0 || calls >= len(f.Calls) || frame.D != f.Calls[calls] {
					return NativeCommandFrameResult{}, fmt.Errorf("actual resource input differs")
				}
				calls++
				step, e := resource.Advance(&rules, NativeResourceFrameCallbacks{RAM: ram, CodeBase: 0x100000, Frame: &frame, Input: &input})
				return NativeCommandFrameResult{Complete: step.Complete}, e
			}}
			step, e := state.Advance(cb)
			if e != nil {
				t.Fatal(e)
			}
			if !step.Complete || !step.FlagsKnown || step.Zero != (f.CCR&4 != 0) || step.Negative != (f.CCR&8 != 0) {
				t.Fatal("native campaign terminal flags differ")
			}
			if frame.D != f.D {
				t.Fatalf("full D differs got%08x want%08x", frame.D, f.D)
			}
			for i, v := range state.A {
				if v.Address != f.A[i] {
					t.Fatalf("A%d differs got%x want%x", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash {
				t.Fatalf("raw campaign BSS%v CODE%v", fileFrameHash(raw) == f.BSSHash, fileFrameHash(code) == f.CodeHash)
			}
		})
	}
}
