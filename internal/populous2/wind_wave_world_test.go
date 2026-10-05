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

func installNativeFixtureWorld(t *testing.T, raw []byte, heights []uint8) *World {
	t.Helper()
	w := lightningWorld(t, 4311)
	copy(w.RecordImage.Bytes[:], raw[NativeRecordImageStart:NativeMagnetImageStart])
	copy(w.NativeGlobals.Bytes[:], raw[NativeMagnetImageStart:NativeRuntimeImageEnd])
	if len(raw) >= 0xeb90 {
		copy(w.NativeCommandBytes[:], raw[0xeb18:0xeb90])
	}
	for pos := range w.Occupancy.Grid.Cells {
		a := 0xf44 + pos*4
		w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: raw[a], Tile: raw[a+1], Head: NativeRecordReference(uint16(raw[a+2])<<8 | uint16(raw[a+3]))}
		w.NativeOverlays[pos] = raw[0x4f44+pos]
		w.Marks[pos] = Mark{Life: 1, Persistent: true, NativeTile: raw[a+1], NativeCodeValid: true}
	}
	for i, h := range heights {
		w.Core.Alt[i] = int(h)
	}
	access := w.runtimeMemory().RecordAccess()
	for _, cell := range w.Occupancy.Grid.Cells {
		for ref, count := cell.Head, 0; ref != 0; count++ {
			if count > 1053 {
				t.Fatal("cyclic native fixture")
			}
			entry, ok := w.Occupancy.entry(ref)
			if !ok {
				t.Fatal("unknown native fixture actor")
			}
			r, _ := access.Record(ref)
			entry.Record, entry.Linked = r, true
			ref = r.Next
		}
	}
	return w
}

func nativeFixtureWorldImage(w *World, initial []byte) []byte {
	all := append([]byte(nil), initial...)
	for pos, cell := range w.Occupancy.Grid.Cells {
		a := 0xf44 + pos*4
		all[a], all[a+1], all[a+2], all[a+3] = cell.Header, cell.Tile, uint8(cell.Head>>8), uint8(cell.Head)
	}
	copy(all[0x4f44:0x5f44], w.NativeOverlays[:])
	copy(all[NativeRecordImageStart:NativeMagnetImageStart], w.RecordImage.Bytes[:])
	copy(all[NativeMagnetImageStart:], w.NativeGlobals.Bytes[:])
	if len(all) >= 0xeb90 {
		copy(all[0xeb18:0xeb90], w.NativeCommandBytes[:])
	}
	return all
}

func TestWorldHurricaneAgainstCompleteNativeMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/hurricane_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []hurricaneFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	updates := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := hurricaneMemory(f)
			w := installNativeFixtureWorld(t, m.bytes[:], nil)
			w.nativeCallDepth++
			created, err := w.HurricaneRules.Create(f.Input.Owner, f.Input.X, f.Input.Y, f.Input.Direction, w.hurricaneCallbacks())
			if err != nil {
				t.Fatal(err)
			}
			if created.Admitted != f.Admitted {
				t.Fatal("World native wind admission differs")
			}
			if created.Admitted {
				location, _ := LocateNativeRecord(created.Reference)
				w.NativeEnvironment[location.Index] = NativeEnvironmentHurricane
			}
			check := func(want hurricaneFixtureFrame) {
				all := nativeFixtureWorldImage(w, m.bytes[:])
				if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
					t.Fatalf("World wind full BSS differs at update%d", want.Tick)
				}
			}
			check(f.Creation)
			for _, frame := range f.Trace {
				updates++
				if _, err := w.HurricaneRules.Tick(created.Reference, w.hurricaneCallbacks()); err != nil {
					t.Fatal(err)
				}
				check(frame)
			}
			w.nativeCallDepth--
		})
	}
	if len(catalog.Cases) != 47 || updates != 586 {
		t.Fatalf("World hurricane coverage: %d cases/%d updates", len(catalog.Cases), updates)
	}
}

func TestWorldWindPushesWithoutRadiusDamageAndKeepsSave(t *testing.T) {
	w := lightningWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 1, Population: 1000, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	beforeMana := w.Core.Magnets[0].Mana
	if !w.Cast(0, Wind, Target{X: 32, Y: 32, Direction: 2}) {
		t.Fatal("wind rejected")
	}
	if w.Core.Magnets[0].Mana != beforeMana-w.ManaCost(0, Wind) || len(w.Effects) != 0 || w.Occupancy.Effects[0].Linked {
		t.Fatal("wind used generic or linked controller")
	}
	if _, ok := w.EnvironmentalEffectFrame(0); ok {
		t.Fatal("wind rendered recycled record art")
	}
	for range 20 {
		w.tickNativeEffects()
	}
	a, _ := w.RecordImage.ReadFollowerEntry(52)
	// A rightward crossing reinserts the actor into a later scanned column,
	// so that actor receives one further push during the same sweep.
	if a.Motion.Population != 1000 || uint16(a.Motion.X) != 32*256+128+21*16 {
		t.Fatalf("native wind push/population differs: %+v", a.Motion)
	}
	copy, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 90 {
		w.tickNativeEffects()
		copy.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) || w.NativeEffects[0].Active {
		t.Fatal("wind terminal/save continuation differs")
	}
}

func TestWorldTsunamiStartsAtAdjacentWaterAndDebitsEmptyPool(t *testing.T) {
	w := oceanWorld(t, 4311)
	pos := 32 + 32*64
	w.writeNativeTownTile(32, 32, 15)
	if !w.Cast(0, Tsunami, Target{X: 32, Y: 32, Direction: 7}) {
		t.Fatal("dry origin rejected despite adjacent water")
	}
	for index := range 4 {
		if !w.NativeEffects[index].Active || w.NativeEnvironment[index] != NativeEnvironmentTsunami || !w.Occupancy.Effects[index].Linked {
			t.Fatal("cardinal front missing")
		}
		if _, ok := w.EnvironmentalEffectFrame(index); !ok {
			t.Fatal("original wave layers missing")
		}
	}
	if len(w.Effects) != 0 || w.Occupancy.Grid.Cells[pos].Head != 0 {
		t.Fatal("wave used directed generic origin")
	}
	for index := range w.NativeEffects {
		if !w.NativeEffects[index].Active {
			w.NativeEffects[index] = NativeEffectActor{Active: true, Kind: 0x20, Player: 0, Speed: 16, State: 8, Animation: 0x4c8, Life: 100}
		}
	}
	before := w.Core.Magnets[0].Mana
	if !w.Cast(0, Tsunami, Target{X: 40, Y: 40}) || w.Core.Magnets[0].Mana != before-w.ManaCost(0, Tsunami) {
		t.Fatal("native full-pool wave debit rejected")
	}
}

func TestWorldTsunamiNewbornCadenceAndSavedContinuation(t *testing.T) {
	w := oceanWorld(t, 4311)
	if !w.Cast(0, Tsunami, Target{X: 32, Y: 32}) {
		t.Fatal("wave rejected")
	}
	w.tickNativeEffects()
	count := 0
	for i, a := range w.NativeEffects {
		if !a.Active {
			continue
		}
		count++
		if a.State != 0x2a || w.NativeEnvironment[i] != NativeEnvironmentTsunami || a.Life != 200 {
			t.Fatal("wave newborn phase/lifetime differs")
		}
	}
	if count <= 4 {
		t.Fatal("lateral fronts did not propagate")
	}
	copy, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 80 {
		w.tickNativeEffects()
		copy.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("wave save continuation differs")
	}
}

func TestLegacyWindWaveMigrationDoesNotRecastOrDebit(t *testing.T) {
	w := oceanWorld(t, 4311)
	s := w.Snapshot()
	s.Version = 20
	s.Effects = []Effect{{Spell: Wind, Player: 0, X: 30, Y: 30, DX: 1, Life: 17}, {Spell: Tsunami, Player: 1, X: 32, Y: 32, DY: -1, Life: 15}}
	before, _ := json.Marshal(s)
	copy, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(s)
	if !bytes.Equal(before, after) || len(copy.Effects) != 0 || copy.Core.Magnets != s.Core.Magnets || copy.Core.Snapshot().RNG != s.Core.RNG || copy.NativeEnvironment[0] != NativeEnvironmentHurricane || copy.NativeEffects[0].Life != 17 || copy.NativeEnvironment[1] != NativeEnvironmentTsunami {
		t.Fatal("old wind/waves recast, debited or mutated input")
	}
	bad := copy.Snapshot()
	bad.Effects = []Effect{{Spell: Wind, Player: 0, X: 30, Y: 30, Life: 17}}
	if _, err := Restore(testBundle(t), bad); err == nil {
		t.Fatal("new save accepted provisional wind")
	}
}

func TestNativeWindWavesRunWithFollowersAndSavedWorld(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 100, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Magnets[0].Mana = 1000000
	if !w.Cast(0, Wind, Target{X: 10, Y: 10, Direction: 2}) {
		t.Fatal("wind rejected")
	}
	if !w.Cast(0, Tsunami, Target{X: 1, Y: 1}) {
		t.Fatal("wave rejected")
	}
	for range 50 {
		w.Tick()
	}
	copy, err := ReadSave(b, bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 150 {
		w.Tick()
		copy.Tick()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("mixed wind/waves/followers saved World diverged")
	}
}
