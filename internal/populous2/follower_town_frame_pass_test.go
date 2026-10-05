package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldTownCompletePassAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_town_frame_pass_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			frameContextFixture
			Frames []struct {
				D                                        [8]uint32
				Hash                                     string
				ErrorPC                                  uint32
				PropertyCache, OuterFlag, MinimapVariant uint16
			}
			Input struct {
				frameContextInput
				Landscape int
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 64 {
		t.Fatal("native town full-pass corpus incomplete")
	}
	b := testBundle(t)
	pass, err := DecodeNativeFollowerPassRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	aftermath, err := DecodeNativeFollowerAftermathFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		raw := frameContextInitial(f.Input.frameContextInput)
		w := installNativeFixtureWorld(t, raw, nil)
		town, err := DecodeNativeFollowerTownFrameRules(b.Executable, b.Raw[fmt.Sprintf("land%d.dat", f.Input.Landscape)])
		if err != nil {
			t.Fatal(err)
		}
		townState := NativeTownFrameState{}

		frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
		state := NativeFollowerPassState{}
		bindings := NativeFollowerFrameBindings{Town: town, TownState: &townState, Aftermath: &aftermath, MapPoint: func(_ uint16, c *NativeFrameRegisterContext) error {
			_, err := PlanNativeMapPoint(c)
			return err
		}}
		w.nativeCallDepth++
		err = w.tickNativeFollowerFrame(&pass, &frame, &state, bindings)
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(f.Input.Name, err)
		}
		want := f.Frames[0]
		if want.ErrorPC != 0 {
			t.Fatal(f.Input.Name, "original full pass fault")
		}
		if frame.D != want.D {
			t.Fatalf("%s town full-pass registers differ: %x/%x", f.Input.Name, frame.D, want.D)
		}
		if townState.Property13550 != want.PropertyCache || townState.OuterFlag13350 != want.OuterFlag || state.MinimapVariant != want.MinimapVariant {
			t.Fatal(f.Input.Name, "native town pass mutable CODE differs")
		}
		all := nativeFixtureWorldImage(w, raw)
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
			t.Fatalf("%s town full-pass retained BSS differs: %s/%s", f.Input.Name, got, want.Hash)
		}
	}
}
