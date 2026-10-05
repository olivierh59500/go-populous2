package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldNativeSiegeFollowerFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_siege_frame_native.json")
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
	if len(corpus.Cases) != 1796 {
		t.Fatal("native siege follower corpus incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeNativeFollowerSiegeFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range corpus.Cases {
		raw := frameContextInitial(f.Input)
		w := installNativeFixtureWorld(t, raw, nil)
		frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
		cb := NativeFollowerSiegeFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: &frame, Cleanup: w.nativeHeroFrameCallbacks(&frame).Cleanup, ReformTown: w.nativeEntryFrameCallbacks(&frame, &NativeFollowerEntryFrameState{}).ReformTown}
		w.nativeCallDepth++
		boundary, err := rules.Tick(52, cb)
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(f.Input.Name, err)
		}
		want := f.Frames[0]
		if want.ErrorPC != 0 || frame.D != want.D || boundary != want.Boundary {
			t.Fatalf("%s siege continuation differs: D%x/%x boundary%x/%x", f.Input.Name, frame.D, want.D, boundary, want.Boundary)
		}
		all := nativeFixtureWorldImage(w, raw)
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
			t.Fatalf("%s siege retained BSS differs: %s/%s", f.Input.Name, got, want.Hash)
		}
	}
}
