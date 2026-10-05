package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeCampaignSoftwareSpriteAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/campaign_selection_children_software_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Cases []struct {
			X, Y, Seed int
			InputD, D  [8]uint32
			InputA, A  [7]uint32
			BitmapHash string
		}
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 96 {
		t.Fatal("software corpus changed")
	}
	for _, f := range corpus.Cases {
		t.Run(fmt.Sprintf("x%d-y%d-d%d", f.X, f.Y, f.Seed), func(t *testing.T) {
			ram := make([]byte, 0xa20000)
			for i := 0; i < 32000; i++ {
				ram[0xa10000+i] = byte(i*37 + f.Seed*51 + 11)
			}
			for i := 0; i < 200; i++ {
				ram[0x700000+i] = byte(i*53 + f.Seed*19 + 7)
			}
			m := commandFrameBacking(ram)
			c := NativeFrameRegisterContext{D: f.InputD}
			a := [7]NativeRequesterAddress{}
			for i, v := range f.InputA {
				a[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			if e = campaignChildSoftwareSprite(NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Frame: &c}, RAM: m}, &a); e != nil {
				t.Fatal(e)
			}
			if c.D != f.D {
				t.Fatalf("Dgot%08x want%08x", c.D, f.D)
			}
			for i, v := range a {
				if v.Address != f.A[i] {
					t.Fatalf("A%d got%x want%x", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(ram[0xa10000:0xa17d00]) != f.BitmapHash {
				t.Fatal("literalsoftwarepixels differ")
			}
		})
	}
}
