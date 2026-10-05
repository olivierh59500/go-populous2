package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestWorldNativeTownFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_town_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		BSSBytes int
		Cases    []townFrameFixture
	}
	if err := json.Unmarshal(data, &corpus); err != nil || corpus.BSSBytes != 0x11280 || len(corpus.Cases) != 820 {
		t.Fatalf("native town World corpus incomplete: %v", err)
	}
	b := testBundle(t)
	for _, f := range corpus.Cases {
		_, raw := townFrameFixtureMemory(t, f.Input)
		w := installNativeFixtureWorld(t, raw, nil)
		w.setNativeBirthBlockWord(f.Input.PoolBlocked)
		rules, err := DecodeNativeFollowerTownFrameRules(b.Executable, b.Raw[fmt.Sprintf("land%d.dat", f.Input.Land)])
		if err != nil {
			t.Fatal(err)
		}
		state := NativeTownFrameState{Property13550: f.Input.PropertyCache, Scratch136E8: f.Input.Scratch}
		frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
		w.nativeCallDepth++
		step, err := rules.Tick(NativeRecordReference(f.Input.Slot*52), w.nativeTownFrameCallbacks(&frame, &state))
		w.nativeCallDepth--
		if f.Trap != "" {
			if err == nil || !strings.Contains(err.Error(), "DIVU zero") {
				t.Fatalf("%s original town trap differs: %v", f.Input.Name, err)
			}
		} else if err != nil {
			t.Fatal(f.Input.Name, err)
		}
		if frame.D != f.D || state.Property13550 != f.PropertyCache || state.Scratch136E8 != f.Scratch || state.MinimapVariant != f.MinimapVariant {
			t.Fatalf("%s native town World registers/cache differ", f.Input.Name)
		}
		all := nativeFixtureWorldImage(w, raw)
		binary.BigEndian.PutUint16(all[0xdc2:], w.nativeBirthBlockWord())
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != f.RawHash {
			t.Fatalf("%s native town World BSS differs: %s/%s", f.Input.Name, got, f.RawHash)
		}
		if f.Trap == "" && step.Redispatch != (f.Exit == "1131c") {
			t.Fatal(f.Input.Name, "native town World continuation differs")
		}
	}
}
