package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func cleanupFixtureBytes(m *cleanupMemory) []byte {
	all := append([]byte(nil), m.lower[:]...)
	all = append(all, m.records.Bytes[:]...)
	return append(all, m.globals.Bytes[:]...)
}

func TestWorldPlagueCreationAgainstOriginalMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/plague_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []plagueNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range catalog.Cases {
		if f.Input.Mode != "create" {
			continue
		}
		checked++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := plagueNativeMemory(f.Input)
			raw := cleanupFixtureBytes(m)
			w := installNativeFixtureWorld(t, raw, nil)
			w.nativeCallDepth++
			step, err := w.PlagueRules.Create(f.Input.Owner, f.Input.X, f.Input.Y, PlagueCallbacks{Memory: w.nativeCleanupMemory(), Cleanup: w.cleanupNativeFollower})
			w.nativeCallDepth--
			if err != nil || step.Admitted != f.Admitted {
				t.Fatalf("World plague admission: %v", err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(nativeFixtureWorldImage(w, raw))); got != f.Hash {
				t.Fatal("World plague full BSS differs")
			}
		})
	}
	if checked != 386 {
		t.Fatalf("World plague cases%d", checked)
	}
}

func TestWorldArmageddonAgainstCompleteOriginalBody(t *testing.T) {
	data, err := os.ReadFile("testdata/armageddon_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []armageddonFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := armageddonFixtureMemory(f.Input)
			raw := cleanupFixtureBytes(m)
			w := installNativeFixtureWorld(t, raw, nil)
			s := w.Core.Snapshot()
			s.RNG = f.Input.Seed
			w.Core = legacy.WorldFromSnapshot(s, w.Core.Rules)
			w.nativeCallDepth++
			cb := ArmageddonCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Cleanup: w.cleanupNativeFollower, Sound: w.nativeEntryCallbacks().Sound, Debit: func(uint16, uint16) error { return nil },
				Convert: func(ref NativeRecordReference, hero uint16) error {
					return w.HeroArt.Convert(ref, heroIDs[hero/2], HeroCreationCallbacks{Memory: w.nativeCleanupMemory(), ClearLeader: w.clearNativeLeader, ClearFarms: w.clearNativeFarms, Sound: w.nativeEntryCallbacks().Sound})
				}}
			if f.Input.Mode == "command" {
				_, err = w.ArmageddonRules.Command(f.Input.Owner, cb)
			} else {
				_, err = w.ArmageddonRules.Cast(cb)
			}
			w.nativeCallDepth--
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(nativeFixtureWorldImage(w, raw))); got != f.Hash || w.Core.Snapshot().RNG != f.RNG {
				t.Fatal("World Armageddon full BSS/RNG differs")
			}
		})
	}
	if len(catalog.Cases) != 720 {
		t.Fatal("World armageddon fixture count incomplete")
	}
}

func TestNativeArmageddonRecastAndSavedContinuation(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Magnets[0].Mana = 1000000
	if !w.Cast(0, Armageddon, Target{}) {
		t.Fatal("global conversion rejected")
	}
	if w.Core.War || w.NativeRaiseEnabled != 1 {
		t.Fatal("legacy war/countdown retained")
	}
	for index, p := range w.Core.Peeps {
		if p.Population > 0 && !w.Heroes[index].Active {
			t.Fatal("allocated eligible follower not converted")
		}
	}
	rng := w.Core.Snapshot().RNG
	mana := w.Core.Magnets[0].Mana
	records := w.RecordImage
	if !w.Cast(0, Armageddon, Target{}) || w.Core.Snapshot().RNG != rng || w.Core.Magnets[0].Mana != mana-w.ManaCost(0, Armageddon) || w.RecordImage != records {
		t.Fatal("native admitted recast changed body or omitted debit")
	}
	copy, err := ReadSave(b, bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 300 {
		w.Tick()
		copy.Tick()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("native Armageddon saved World continuation differs")
	}
}
