package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldVolcanoAgainstOriginalCompleteEruption(t *testing.T) {
	data, err := os.ReadFile("testdata/volcano_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []volcanoFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range catalog.Cases {
		if fixture.Input.Occupied > 0 {
			continue
		}
		checked++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			v := newVolcanoTestMemory(t, fixture)
			w := lightningWorld(t, fixture.Input.Seed)
			state := v.core.Snapshot()
			state.RNG = fixture.Input.Seed
			w.Core = legacy.WorldFromSnapshot(state, v.core.Rules)
			w.Marks = [4096]Mark{}
			w.Scenery = [SceneryCapacity]SceneryActor{}
			w.Walls = WallState{}
			w.initializeActorGraph()
			for pos := range w.Occupancy.Grid.Cells {
				at := 0xf44 + pos*4
				w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: v.m.byte(at), Tile: v.m.byte(at + 1), Head: NativeRecordReference(v.m.word(at + 2))}
			}
			w.RecordImage = v.m.records
			w.NativeGlobals = v.m.globals
			w.hydrateNativeRuntimeRecords()
			comparison := -1
			verify := func(snapshot volcanoSnapshot) {
				poolAt := 0xc800 - NativeRecordImageStart
				source := cleanupRecordAddress(NativeRecordReference(fixture.Reference)) - NativeRecordImageStart
				if fmt.Sprintf("%x", sha256.Sum256(w.RecordImage.Bytes[poolAt:poolAt+8000])) != snapshot.PoolHash || worldNativeGridHash(w) != snapshot.GridHash || nativeDirectHeightHash(w.Core.Alt) != snapshot.HeightHash || hex.EncodeToString(w.RecordImage.Bytes[source:source+32]) != snapshot.SourceRaw || w.Core.Snapshot().RNG != snapshot.RNG {
					t.Fatalf("World volcano comparison%d phase%d differs: pool%v map%v height%v source%v RNG%v", comparison, snapshot.Phase, fmt.Sprintf("%x", sha256.Sum256(w.RecordImage.Bytes[poolAt:poolAt+8000])) == snapshot.PoolHash, worldNativeGridHash(w) == snapshot.GridHash, nativeDirectHeightHash(w.Core.Alt) == snapshot.HeightHash, hex.EncodeToString(w.RecordImage.Bytes[source:source+32]) == snapshot.SourceRaw, w.Core.Snapshot().RNG == snapshot.RNG)
				}
			}
			ref, ok, err := w.VolcanoRules.Create(fixture.Input.Owner, fixture.Input.X, fixture.Input.Y, w.volcanoCallbacks())
			if err != nil {
				t.Fatal(err)
			}
			if ok != fixture.Accepted {
				t.Fatal("native volcano allocation differs")
			}
			if _, _, err := v.volcano.Create(fixture.Input.Owner, fixture.Input.X, fixture.Input.Y, v.callbacks()); err != nil {
				t.Fatal(err)
			}
			if ok {
				location, _ := LocateNativeRecord(ref)
				w.NativeEnvironment[location.Index] = NativeEnvironmentVolcano
			}
			w.hydrateNativeRuntimeRecords()
			verify(fixture.Initial)
			for index, snapshot := range fixture.Trace {
				comparison = index
				if _, err := v.volcano.Tick(ref, v.callbacks()); err != nil {
					t.Fatal(err)
				}
				if err := w.runNativeFollowerCall(func() error { _, err := w.VolcanoRules.Tick(ref, w.volcanoCallbacks()); return err }); err != nil {
					t.Fatal(err)
				}
				if worldNativeGridHash(w) != snapshot.GridHash {
					for pos, cell := range w.Occupancy.Grid.Cells {
						at := 0xf44 + pos*4
						native := NativeOccupancyCell{Header: v.m.byte(at), Tile: v.m.byte(at + 1), Head: NativeRecordReference(v.m.word(at + 2))}
						if cell != native {
							t.Fatalf("first World/native map difference at comparison%d pos%d: got%+v native%+v", index, pos, cell, native)
						}
					}
				}
				verify(snapshot)
			}
		})
	}
	if checked != 24 {
		t.Fatalf("World complete eruption coverage differs: %d", checked)
	}
}
