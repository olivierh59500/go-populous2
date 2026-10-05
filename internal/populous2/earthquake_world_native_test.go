package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldEarthquakeAgainstOriginalPoolAndMap(t *testing.T) {
	data, err := os.ReadFile("testdata/earthquake_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []quakeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range catalog.Cases {
		if fixture.Input.Full || fixture.Input.ParentOnly || fixture.Input.EdgeParent {
			continue
		}
		checked++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m, core := quakeFixtureMemory(fixture)
			w := lightningWorld(t, fixture.Input.Seed)
			w.Core = core
			w.initializeActorGraph()
			for pos := range w.Occupancy.Grid.Cells {
				at := 0xf44 + pos*4
				w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: m.bytes[at], Tile: m.bytes[at+1], Head: NativeRecordReference(m.word(at + 2))}
			}
			copy(w.RecordImage.Bytes[:], m.bytes[NativeRecordImageStart:NativeRecordImageEnd])
			copy(w.NativeGlobals.Bytes[:], m.bytes[NativeRecordImageEnd:NativeRuntimeImageEnd])
			w.SetEffectView(30, 30)
			w.hydrateNativeRuntimeRecords()
			assert := func(frame quakeFrameFixture) {
				at := 0xc800 - NativeRecordImageStart
				if fmt.Sprintf("%x", sha256.Sum256(w.RecordImage.Bytes[at:at+8000])) != frame.PoolSHA256 || worldNativeGridHash(w) != frame.GridSHA256 || nativeDirectHeightHash(w.Core.Alt) != frame.AltSHA256 {
					t.Fatalf("Worldnativequakepool/map/heights differ at tick%d", frame.Tick)
				}
				if w.Core.Snapshot().RNG != frame.RNG {
					t.Fatal("Worldquake native RNG differs")
				}
			}
			result, err := w.EarthquakeRules.Create(fixture.Input.Owner, uint8(fixture.Input.X), uint8(fixture.Input.Y), fixture.Input.Direction, fixture.Input.Strength, 0xeb56, w.quakeCallbacks())
			if err != nil {
				t.Fatal(err)
			}
			if result.Allocated {
				w.NativeEnvironment[(result.Address-0xc800)/32] = NativeEnvironmentQuake
			}
			w.hydrateNativeRuntimeRecords()
			assert(fixture.Creation)
			for _, frame := range fixture.Trace {
				w.tickNativeEffects()
				assert(frame)
			}
		})
	}
	if checked != 31 {
		t.Fatalf("World quake original comparison coverage differs: %d", checked)
	}
}
