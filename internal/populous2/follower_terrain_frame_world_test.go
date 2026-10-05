package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Unlike the earlier World-backed inline callbacks, this proof exercises the
// production terrain binding used by the parent follower dispatcher.
func TestWorldNativeFollowerTerrainFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_terrain_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []terrainFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 1324 {
		t.Fatalf("native terrain World corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeFollowerTerrainFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	states, faults := map[uint8]int{}, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) != 1 {
				t.Fatal("native terrain body fixture must contain one complete prefix")
			}
			initial := frameContextInitial(f.Input)
			at := cleanupRecordAddress(NativeRecordReference(f.Input.Actor))
			states[initial[at+22]]++
			w := aiFixtureWorld(t, initial)
			copy(w.NativeRedrawBytes[:], initial[0xeb90:])
			w.hydrateNativeRuntimeGraph()
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			w.nativeCallDepth++
			boundary, err := rules.Tick(NativeRecordReference(f.Input.Actor), w.nativeTerrainFrameCallbacks(&frame))
			w.nativeCallDepth--
			want := f.Frames[0]
			if want.ErrorPC == 0 {
				if err != nil {
					t.Fatal(err)
				}
				if boundary != want.Boundary {
					t.Fatalf("native terrain World boundary differs: got%x want%x", boundary, want.Boundary)
				}
			} else {
				faults++
				if err == nil || want.ErrorPC != 0x119b0 {
					t.Fatalf("native terrain World address-fault prefix changed: %x/%v", want.ErrorPC, err)
				}
			}
			if frame.D != want.D {
				t.Fatalf("native terrain World all8D differ: got%x want%x", frame.D, want.D)
			}
			all := aiFixtureWorldBytes(w, initial)
			copy(all[0xeb90:], w.NativeRedrawBytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
				t.Fatalf("native terrain World complete BSS differs: got%s want%s", got, want.Hash)
			}
		})
	}
	if states[0x16] == 0 || states[0x36] == 0 || states[0x3c] == 0 || faults != 12 {
		t.Fatalf("native terrain World branch coverage incomplete: states%v faults%d", states, faults)
	}
}
