package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeCampaignIconsAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/campaign_frame_icons_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Cases []struct {
			Owner                       uint16
			Pattern, Seed               int
			InputD, D                   [8]uint32
			InputA, A                   [7]uint32
			Flags                       [36]byte
			BSSHash, CodeHash, ChipHash string
		}
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 144 {
		t.Fatal("native world-icon corpus changed")
	}
	bundle := testBundle(t)
	rules, e := DecodeNativeRenderFrameRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	base, e := NewNativeHunkMemory(bundle.Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
	if e != nil {
		t.Fatal(e)
	}
	if e = PrepareNativeResourceFramePlanes(base.Memory(), 0x1214b2, 0x2003be); e != nil {
		t.Fatal(e)
	}
	for _, f := range corpus.Cases {
		t.Run(fmt.Sprintf("owner%x-pattern%d-context%d", f.Owner, f.Pattern, f.Seed), func(t *testing.T) {
			host := &NativeHostMemory{}
			for _, region := range base.Regions {
				region.Bytes = append([]byte(nil), region.Bytes...)
				if e = host.MapRegion(region); e != nil {
					t.Fatal(e)
				}
			}
			raw, _ := host.Span(0x200000, 0x11280)
			code, _ := host.Span(0x100000, 0x3fa2c)
			chip, _ := host.Span(0x500000, 65032)
			ram := host.Memory()
			cm, m := commandFrameBacking(code), commandFrameBacking(raw)
			for i := range raw {
				raw[i] = byte(i*11 + f.Seed*31 + 17)
			}
			for i := 0x408; i < len(chip); i++ {
				chip[i] = byte(i*17 + f.Seed*71 + 3)
			}
			_ = m.Write32(0x1e, 0x508108)
			for i, v := range f.Flags {
				_ = ram.Write8(int(f.InputA[6])+0x70+i, v)
			}
			frame := NativeFrameRegisterContext{D: f.InputD, AddressBase: 0x200000}
			a := [7]NativeRequesterAddress{}
			for i, v := range f.InputA {
				a[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			cb := NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Bitmap: func(at uint32) ([]byte, error) { return host.Span(at, 32000) }}, RAM: ram}
			if e = DrawNativeCampaignIcons(&rules, cb, &a); e != nil {
				t.Fatal(e)
			}
			if frame.D != f.D {
				t.Fatalf("allD got%08x want%08x", frame.D, f.D)
			}
			for i, v := range a {
				if v.Address != f.A[i] {
					t.Fatalf("A%d got%x want%x", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(chip) != f.ChipHash {
				t.Fatalf("completeBSS%v CODE%v actualpixels%v", fileFrameHash(raw) == f.BSSHash, fileFrameHash(code) == f.CodeHash, fileFrameHash(chip) == f.ChipHash)
			}
		})
	}
}
