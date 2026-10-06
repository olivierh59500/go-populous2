package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeFollowerDirectFounderTownTailAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_town_direct_tail_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Input struct {
				frameContextInput
				Landscape int
			}
			Frames []struct {
				D                                        [8]uint32
				Hash                                     string
				ErrorPC                                  uint32
				PropertyCache, OuterFlag, MinimapVariant uint16
			}
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 16 {
		t.Fatal("native direct town corpus incomplete", err)
	}
	bundle := testBundle(t)
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			raw := frameContextInitial(f.Input.frameContextInput)
			w := installNativeFixtureWorld(t, raw, nil)
			town, err := DecodeNativeFollowerTownFrameRules(bundle.Executable, bundle.Raw[fmt.Sprintf("land%d.dat", f.Input.Landscape)])
			if err != nil {
				t.Fatal(err)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := NativeFollowerPassState{}
			townState := NativeTownFrameState{}
			w.nativeCallDepth++
			flow, err := w.nativeFollowerFrameContinuation(52, 0x11738, &frame, &state, NativeFollowerFrameBindings{Town: town, TownState: &townState})
			w.nativeCallDepth--
			want := f.Frames[0]
			if err != nil || want.ErrorPC != 0 || flow != NativeFollowerCount || frame.D != want.D {
				t.Fatal("direct founder-town tail differs", flow, frame.D, want.D, err)
			}
			if fileFrameHash(nativeFixtureWorldImage(w, raw)) != want.Hash {
				t.Fatal("direct founder-town raw memory differs")
			}
			if townState.Property13550 != want.PropertyCache || townState.OuterFlag13350 != want.OuterFlag || state.MinimapVariant != want.MinimapVariant {
				t.Fatal("direct town mutable state differs")
			}
		})
	}
}
