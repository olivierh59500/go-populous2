package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// World replays complete native wave passes, including both earlier and later
// newborn slots and actual Core shore lowering. Map/pool/height checks cover
// every byte in those domains; the native scratch/dirty counters are separate
// from this adapter and are covered by the standalone full-BSS proof.
func TestWorldTsunamiAgainstOriginalPoolMapAndHeights(t *testing.T) {
	data, err := os.ReadFile("testdata/tsunami_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeTsunamiFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked, updates := 0, 0
	for _, f := range catalog.Cases {
		if f.Input.Mode != "pass" && !strings.HasPrefix(f.Input.Name, "shore-height") {
			continue
		}
		checked++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := tsunamiFixtureMemory(f.Input)
			var raw [NativeRuntimeImageEnd]byte
			copy(raw[:NativeRecordImageStart], m.lower[:])
			copy(raw[NativeRecordImageStart:NativeMagnetImageStart], m.records.Bytes[:])
			copy(raw[NativeMagnetImageStart:], m.globals.Bytes[:])
			w := installNativeFixtureWorld(t, raw[:], f.Input.Alt)
			w.nativeCallDepth++
			for _, trace := range f.Trace {
				updates++
				if f.Input.Mode == "pass" {
					for index := range NativeEffectCapacity {
						owner, _ := w.RecordImage.Read8(nativeActorReference(NativeEffectPool, index), 12)
						if owner != 0 {
							if _, err := w.TsunamiRules.Tick(nativeActorReference(NativeEffectPool, index), w.tsunamiCallbacks()); err != nil {
								t.Fatal(err)
							}
						}
					}
				} else {
					if _, err := w.TsunamiRules.Tick(nativeActorReference(NativeEffectPool, f.Input.Parent), w.tsunamiCallbacks()); err != nil {
						t.Fatal(err)
					}
				}
				all := nativeFixtureWorldImage(w, raw[:])
				if got := fmt.Sprintf("%x", sha256.Sum256(all[0xc800:0xe740])); got != trace.PoolHash {
					t.Fatalf("World native wave pool differs at update%d", trace.Update)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(all[0xf44:0x4f44])); got != trace.GridHash {
					t.Fatalf("World native wave map differs at update%d", trace.Update)
				}
				if got := nativeDirectHeightHash(w.Core.Alt); got != trace.AltHash {
					t.Fatalf("World native wave heights differ at update%d", trace.Update)
				}
			}
			w.nativeCallDepth--
		})
	}
	if checked != 40 || updates == 0 {
		t.Fatalf("World tsunami coverage incomplete: cases%d updates%d", checked, updates)
	}
}
