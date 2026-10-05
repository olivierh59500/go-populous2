package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldNeutralDispatcherCompletePassAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_neutral_pass_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []neutralFrameFixture }
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 28 {
		t.Fatal("native neutral dispatcher pass corpus incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeNativeFollowerNeutralFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	pass, err := DecodeNativeFollowerPassRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	aftermath, err := DecodeNativeFollowerAftermathFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range corpus.Cases {
		initial := frameContextInitial(f.Input)
		w := aiFixtureWorld(t, initial)
		frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
		state := NativeFollowerPassState{}
		bindings := NativeFollowerFrameBindings{Neutral: &rules, Aftermath: &aftermath}
		w.nativeCallDepth++
		err = w.tickNativeFollowerFrame(&pass, &frame, &state, bindings)
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(f.Input.Name, err)
		}
		want := f.Frames[0]
		if want.ErrorPC != 0 || frame.D != want.D {
			t.Fatalf("%s neutral dispatcher full registers differ: %x/%x", f.Input.Name, frame.D, want.D)
		}
		all := aiFixtureWorldBytes(w, initial)
		copy(all[0xeb90:], w.NativeRedrawBytes[:])
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
			t.Fatalf("%s neutral dispatcher retained BSS differs: %s/%s", f.Input.Name, got, want.Hash)
		}
	}
}
