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

func TestWorldHeroCreationAgainstCompleteNativeMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/hero_creation_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []heroCreationFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := heroCreationFixtureMemory(fixture.Input)
			w := lightningWorld(t, 4311)
			w.RecordImage, w.NativeGlobals = m.records, m.globals
			for pos := range w.Occupancy.Grid.Cells {
				at := 0xf44 + pos*4
				w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: m.byte(at), Tile: m.byte(at + 1), Head: NativeRecordReference(m.word(at + 2))}
				w.NativeOverlays[pos] = m.byte(0x4f44 + pos)
				w.Marks[pos] = Mark{Life: 1, Persistent: true, NativeTile: m.byte(at + 1), NativeCodeValid: true}
			}
			access := w.runtimeMemory().RecordAccess()
			for _, cell := range w.Occupancy.Grid.Cells {
				for ref := cell.Head; ref != 0; {
					entry, ok := w.Occupancy.entry(ref)
					if !ok {
						t.Fatal("unknown native fixture reference")
					}
					record, _ := access.Record(ref)
					entry.Record, entry.Linked = record, true
					ref = record.Next
				}
			}
			w.nativeCallDepth++
			cb := HeroCreationCallbacks{Memory: w.nativeCleanupMemory(), ClearLeader: w.clearNativeLeader, ClearFarms: w.clearNativeFarms, Sound: w.nativeEntryCallbacks().Sound}
			if fixture.Input.Direct {
				err = w.HeroArt.Convert(52, heroIDs[fixture.Input.Hero], cb)
			} else {
				_, err = w.HeroArt.Create(heroIDs[fixture.Input.Hero], uint16(fixture.Input.Owner), cb)
			}
			w.nativeCallDepth--
			if err != nil {
				t.Fatal(err)
			}
			var all [NativeRuntimeImageEnd]byte
			copy(all[:NativeRecordImageStart], m.lower[:])
			for pos, cell := range w.Occupancy.Grid.Cells {
				at := 0xf44 + pos*4
				all[at], all[at+1], all[at+2], all[at+3] = cell.Header, cell.Tile, uint8(cell.Head>>8), uint8(cell.Head)
			}
			copy(all[0x4f44:0x5f44], w.NativeOverlays[:])
			copy(all[NativeRecordImageStart:NativeMagnetImageStart], w.RecordImage.Bytes[:])
			copy(all[NativeMagnetImageStart:], w.NativeGlobals.Bytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(all[:])); got != fixture.Hash {
				t.Fatal("World hero creation differs from full original BSS")
			}
		})
	}
}

func TestWorldHeroCreationKeepsNativeLegAndSavedContinuation(t *testing.T) {
	b := testBundle(t)
	for _, id := range heroIDs {
		t.Run(spellName(b, id), func(t *testing.T) {
			w, err := NewWorld(b, 0, true)
			if err != nil {
				t.Fatal(err)
			}
			index := w.Core.Magnets[0].Carried - 1
			ref := nativeActorReference(NativeFollowerPool, index)
			w.Core.Magnets[0].Mana = 1000000
			if err := w.runNativeFollowerCall(func() error {
				m := w.nativeCleanupMemory()
				if err := m.Write16(cleanupRecordAddress(ref)+14, 19); err != nil {
					return err
				}
				if err := m.Write16(cleanupRecordAddress(ref)+16, 23); err != nil {
					return err
				}
				return m.Write16(cleanupRecordAddress(ref)+20, 17)
			}); err != nil {
				t.Fatal(err)
			}
			before, _ := w.RecordImage.ReadFollowerEntry(ref)
			cost := w.ManaCost(0, id)
			if !w.Cast(0, id, Target{}) {
				t.Fatal("hero rejected")
			}
			cues := w.TakeEffectSoundCues()
			if len(cues) != 1 || cues[0] != int(w.HeroArt.Sounds[heroIndex(id)]/10) {
				t.Fatal("native conversion cue was omitted or duplicated")
			}
			a, _ := w.RecordImage.ReadFollowerEntry(ref)
			if a.Motion.State != 0x24 || a.Motion.Kind != 2 || a.Motion.Flags&3 != 2 || a.Motion.VX != before.Motion.VX || a.Motion.VY != before.Motion.VY || a.Motion.Timer != before.Motion.Timer || w.Core.Magnets[0].Carried != 0 || w.Core.Magnets[0].Mana != 1000000-cost || w.Core.Peeps[index].Status != legacy.KnightStatus {
				t.Fatal("native conversion fields or debit differ")
			}
			if w.Core.Magnets[0].GoTo != w.Core.Peeps[index].AtPos {
				t.Fatal("native marker did not relocate to former leader")
			}
			copy, err := ReadSave(b, bytes.NewReader(encodeSnapshot(t, w)))
			if err != nil {
				t.Fatal(err)
			}
			for range 180 {
				w.Core.TickWithComputer([2]bool{})
				copy.Core.TickWithComputer([2]bool{})
			}
			if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
				t.Fatal("hero saved continuation differs")
			}
		})
	}
}

func TestOrdinaryMotionPreservesEntryLeaderAndDiseaseFlags(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	index := w.Core.Magnets[0].Carried - 1
	ref := nativeActorReference(NativeFollowerPool, index)
	w.initializeEntryRecord(index)
	w.NativeEntries[index].Actor.Motion.Flags |= 16
	a, err := w.readEntryRecord(ref)
	if err != nil || a.Motion.Flags&17 != 17 {
		t.Fatal("fractional motion discarded leader or disease flags")
	}
}
