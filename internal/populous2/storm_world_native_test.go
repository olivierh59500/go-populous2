package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	legacy "go-populous2/internal/legacy"
	"os"
	"testing"
)

func TestWorldStormRuntimeAgainstCompleteNativeMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/storm_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeStormFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked, updates := 0, 0
	for _, fixture := range catalog.Cases {
		if fixture.Input.Mode != "tick" {
			continue
		}
		checked++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := stormFixtureMemory(fixture.Input)
			w := lightningWorld(t, fixture.Input.Seed)
			state := w.Core.Snapshot()
			state.Peeps = nil
			state.RNG = fixture.Input.Seed
			w.Core = legacy.WorldFromSnapshot(state, w.Core.Rules)
			w.RecordImage, w.NativeGlobals = m.records, m.globals
			w.NativeEntries = [legacy.MaxPeeps]NativeFollowerEntry{}
			for pos := range w.Occupancy.Grid.Cells {
				at := 0xf44 + pos*4
				w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: m.byte(at), Tile: m.byte(at + 1), Head: NativeRecordReference(m.word(at + 2))}
				w.NativeOverlays[pos] = m.byte(0x4f44 + pos)
				w.Marks[pos] = Mark{Life: 1, Persistent: true, NativeTile: m.byte(at + 1), NativeCodeValid: true}
			}
			access := w.runtimeMemory().RecordAccess()
			for pos, cell := range w.Occupancy.Grid.Cells {
				for ref, count := cell.Head, 0; ref != 0; count++ {
					if count > 1053 {
						t.Fatal("invalidfixturechain")
					}
					entry, ok := w.Occupancy.entry(ref)
					if !ok {
						t.Fatal("unknownfixturegraphref")
					}
					record, _ := access.Record(ref)
					entry.Record, entry.Linked = record, true
					_ = pos
					ref = record.Next
				}
			}
			w.NativeEnvironment[0] = NativeEnvironmentStorm
			w.hydrateNativeRuntimeRecords()
			hash := func() string {
				var all [NativeRuntimeImageEnd]byte
				copy(all[:NativeRecordImageStart], m.lower[:])
				for pos, cell := range w.Occupancy.Grid.Cells {
					at := 0xf44 + pos*4
					all[at], all[at+1], all[at+2], all[at+3] = cell.Header, cell.Tile, uint8(cell.Head>>8), uint8(cell.Head)
				}
				copy(all[0x4f44:0x5f44], w.NativeOverlays[:])
				copy(all[NativeRecordImageStart:NativeMagnetImageStart], w.RecordImage.Bytes[:])
				copy(all[NativeMagnetImageStart:], w.NativeGlobals.Bytes[:])
				return fmt.Sprintf("%x", sha256.Sum256(all[:]))
			}
			for _, trace := range fixture.Trace {
				updates++
				w.nativeCallDepth++
				_, err := w.StormRules.Tick(0x5140, w.stormCallbacks())
				w.nativeCallDepth--
				if err != nil {
					t.Fatal(err)
				}
				if hash() != trace.Hash || w.Core.Snapshot().RNG != trace.RNG {
					t.Fatalf("Worldstorm rawmemory/RNG differs at update%d", trace.Update)
				}
			}
		})
	}
	if checked != 546 || updates != 1541 {
		t.Fatalf("World storm native runtime coverage differs: cases%d updates%d", checked, updates)
	}
}
