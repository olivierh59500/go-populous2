package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldCombatWinnerCompletePassAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_combat_win_pass_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			frameContextFixture
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
		t.Fatal("native combat full-pass corpus incomplete")
	}
	b := testBundle(t)
	pass, err := DecodeNativeFollowerPassRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeFollowerCombatFrameRules(b.Executable)
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
		w.FollowerWin, err = DecodeFollowerWinRules(b.Executable, b.Landscapes[f.Input.Landscape])
		if err != nil {
			t.Fatal(err)
		}
		frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
		state := NativeFollowerPassState{}
		bindings := NativeFollowerFrameBindings{Combat: &combat, Aftermath: &aftermath, MapPoint: func(_ uint16, c *NativeFrameRegisterContext) error {
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
			t.Fatalf("%s combat full-pass registers differ: %x/%x", f.Input.Name, frame.D, want.D)
		}
		all := nativeFixtureWorldImage(w, raw)
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
			t.Fatalf("%s combat full-pass retained BSS differs: %s/%s", f.Input.Name, got, want.Hash)
		}
	}
}
