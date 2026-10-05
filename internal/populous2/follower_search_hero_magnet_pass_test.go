package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldSearchHeroMagnetCompletePassAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_search_hero_magnet_pass_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []frameContextFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 112 {
		t.Fatal("native search/hero/magnet full-pass corpus incomplete")
	}
	b := testBundle(t)
	pass, err := DecodeNativeFollowerPassRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	motion, err := DecodeFollowerMotionFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := DecodeNativeFollowerEntryFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	aftermath, err := DecodeNativeFollowerAftermathFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	heroes, err := DecodeNativeFollowerHeroFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	magnet, err := DecodeNativeFollowerMagnetFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := DecodeFollowerDecisionFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeFollowerCombatFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}

	for _, fixture := range catalog.Cases {
		raw := frameContextInitial(fixture.Input)
		w := installNativeFixtureWorld(t, raw, nil)
		frame := NativeFrameRegisterContext{D: fixture.Input.D, AddressBase: 0x200000}
		state := NativeFollowerPassState{}
		bindings := NativeFollowerFrameBindings{Motion: &motion, Entry: &entry, Aftermath: &aftermath, Hero: &heroes, Magnet: &magnet, Decision: &decision, Combat: &combat, MapPoint: func(_ uint16, c *NativeFrameRegisterContext) error { _, err := PlanNativeMapPoint(c); return err }}
		w.nativeCallDepth++
		err := w.tickNativeFollowerFrame(&pass, &frame, &state, bindings)
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		want := fixture.Frames[0]
		if want.ErrorPC != 0 {
			t.Fatal("original search/hero/magnet source fault")
		}
		if frame.D != want.D {
			t.Fatalf("%s composed search/hero/magnet registers differ: %x/%x", fixture.Input.Name, frame.D, want.D)
		}
		all := nativeFixtureWorldImage(w, raw)
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
			t.Fatalf("%s composed search/hero/magnet BSS differs: %s/%s", fixture.Input.Name, got, want.Hash)
		}
	}
}
