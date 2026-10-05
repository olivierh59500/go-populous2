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

func TestWorldForestRenewAgainstCompleteNativeMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/forest_renew_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeForestFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked, updates := 0, 0
	for _, f := range catalog.Cases {
		checked++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := forestFixtureMemory(f.Input)
			raw := cleanupFixtureBytes(m)
			w := installNativeFixtureWorld(t, raw, nil)
			s := w.Core.Snapshot()
			s.RNG = f.Input.Seed
			w.Core = legacy.WorldFromSnapshot(s, w.Core.Rules)
			w.nativeCallDepth++
			var err error
			check := func(hash string, rng uint32, clock uint16) {
				all := nativeFixtureWorldImage(w, raw)
				all[0xf42], all[0xf43] = uint8(clock>>8), uint8(clock)
				if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != hash || w.Core.Snapshot().RNG != rng {
					t.Fatal("World forest/renew full memory or RNG differs")
				}
			}
			switch f.Input.Mode {
			case "renew":
				_, err = w.RenewNative.Create(f.Input.Owner, f.Input.X, f.Input.Y, w.nativeRenewCallbacks(0))
			case "cast":
				_, err = w.ForestNative.Cast(uint8(f.Input.Owner), f.Input.X, f.Input.Y, w.nativeForestCallbacks())
			case "plant":
				_, err = w.ForestNative.Plant(f.Input.Owner, NativePackedTile(uint16(f.Input.Y)<<8|uint16(f.Input.X)), w.nativeForestCallbacks())
			case "age":
				for _, trace := range f.Trace {
					updates++
					clock := f.Input.Clock + uint16(trace.Update-1)
					w.Core.GameTurn = int(clock)
					for index := range SceneryCapacity {
						if _, err = w.ForestNative.Tick(nativeActorReference(NativeSceneryPool, index), clock, w.nativeForestCallbacks()); err != nil {
							t.Fatal(err)
						}
					}
					check(trace.Hash, trace.RNG, clock)
				}
			default:
				t.Fatal("unknown forest fixture mode")
			}
			if err != nil {
				t.Fatal(err)
			}
			clock := f.Input.Clock
			if f.Input.Mode == "age" && len(f.Trace) > 0 {
				clock += uint16(f.Trace[len(f.Trace)-1].Update - 1)
			}
			check(f.Hash, f.RNG, clock)
			w.nativeCallDepth--
		})
	}
	if checked != 4522 || updates != 4020 {
		t.Fatalf("World forest/renew coverage incomplete: %d cases %d updates", checked, updates)
	}
}

func TestNativeForestMetricAndRenewDebit(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	w.initializeActorGraph()
	before := w.Core.Magnets[0].Mana
	if !w.Cast(0, Trees, Target{X: 32, Y: 32}) {
		t.Fatal("native forest rejected")
	}
	count := 0
	for _, a := range w.Scenery {
		if a.Active {
			count++
		}
	}
	god, _ := NativeDeityAddress(1)
	metric, _ := w.runtimeMemory().Read16(god + 0x44)
	if metric != uint16(count) || w.Core.Magnets[0].Mana != before-w.ManaCost(0, Trees) {
		t.Fatal("native created-tree metric or debit differs")
	}
	for i := range w.Scenery {
		if !w.Scenery[i].Active {
			w.Scenery[i] = SceneryActor{Kind: SceneryTree, Active: true, X: i % 64, Y: i / 64, Age: 24, Animation: w.SceneryBank.Trees.Animations[0]}
		}
	}
	before = w.Core.Magnets[0].Mana
	if w.Cast(0, Trees, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before {
		t.Fatal("empty/full-pool forest cast debited")
	}
	if !w.Cast(0, Flowers, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before-w.ManaCost(0, Flowers) {
		t.Fatal("renew failed empty/occupied cast debit")
	}
}

func TestNativeForestFirePassAndSavedContinuation(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.allocateScenery(SceneryTree, 32, 32, w.SceneryBank.Trees.Animations[0], 0)
	w.allocateScenery(SceneryTree, 33, 32, w.SceneryBank.Trees.Animations[0], 0)
	w.Core.Peeps = []legacy.Peep{{Player: 1, Population: 100, AtPos: 32 + 31*64, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	w.Scenery[0].Removing, w.Scenery[0].Animation = true, 0xf10
	for range 12 {
		w.tickScenery()
	}
	copy, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 50 {
		w.tickScenery()
		copy.tickScenery()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("native fire/scenery saved continuation differs")
	}
	for _, a := range w.Scenery {
		if a.Active {
			t.Fatal("completed burn pass kept expired tree")
		}
	}
}
