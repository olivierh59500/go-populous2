package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldNativeMagnetFollowerFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_magnet_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			frameContextFixture
			Frames []struct {
				D                 [8]uint32
				Hash              string
				Boundary, ErrorPC uint32
			}
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 496 {
		t.Fatal("native magnet follower corpus incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeNativeFollowerMagnetFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	heroes, err := DecodeNativeFollowerHeroFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range corpus.Cases {
		raw := frameContextInitial(f.Input)
		w := installNativeFixtureWorld(t, raw, nil)
		frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
		cb := w.nativeMagnetFrameCallbacks(&frame, &heroes)
		boundary := uint32(0xf00000 - 0x100000)
		w.nativeCallDepth++
		if f.Input.Stages[0] == 0x140f0 {
			err = rules.Home(52, cb)
		} else {
			boundary, err = rules.Tick(52, cb)
		}
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(f.Input.Name, err)
		}
		want := f.Frames[0]
		if want.ErrorPC != 0 || frame.D != want.D || boundary != want.Boundary {
			t.Fatalf("%s magnet continuation differs: D%x/%x boundary%x/%x", f.Input.Name, frame.D, want.D, boundary, want.Boundary)
		}
		all := nativeFixtureWorldImage(w, raw)
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
			t.Fatalf("%s magnet retained BSS differs: %s/%s", f.Input.Name, got, want.Hash)
		}
	}
}
