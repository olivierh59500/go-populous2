package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestRuntimeWorldHydrationPreservesAllNativeCleanupImages(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_cleanup_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []cleanupFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 49 {
		t.Fatal("native cleanup hydration catalog incomplete", err)
	}
	town, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := cleanupFixtureMemory(t, fixture)
			_, err := CleanupFollower(52, FollowerCleanupRegisters{D0: fixture.Input.Mode, D1: 0xaabbccdd, D2: 0x98765432}, FollowerCleanupCallbacks{
				Memory: FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32},
				Unlink: m.unlink, Insert: m.insert,
				ClearFarms: func(ref NativeRecordReference, tile uint8) error { return town.ClearFarms(ref, tile, m.town(t)) },
			})
			if err != nil {
				t.Fatal(err)
			}
			w := &World{Core: &legacy.World{Peeps: []legacy.Peep{{Population: 999, MovementSpeed: 99}}}, RecordImage: m.records, NativeGlobals: m.globals}
			for index := range w.Occupancy.Grid.Cells {
				at := 0xf44 + index*4
				w.Occupancy.Grid.Cells[index] = NativeOccupancyCell{Header: m.lower[at], Tile: m.lower[at+1], Head: NativeRecordReference(binary.BigEndian.Uint16(m.lower[at+2:]))}
			}
			// Membership was changed by the original primitives, not hydration.
			for _, cell := range w.Occupancy.Grid.Cells {
				for ref := cell.Head; ref != 0; {
					entry, ok := w.Occupancy.entry(ref)
					if !ok {
						t.Fatal("native cleanup chain outside actor/marker pools")
					}
					entry.Linked = true
					ref = NativeRecordReference(m.word(cleanupRecordAddress(ref) + 2))
				}
			}
			records, globals, grid := w.RecordImage, w.NativeGlobals, w.Occupancy.Grid
			linked := w.Occupancy.Followers[0].Linked
			w.hydrateNativeRuntimeRecords()
			if w.RecordImage != records || w.NativeGlobals != globals || w.Occupancy.Grid != grid || w.Occupancy.Followers[0].Linked != linked {
				t.Fatal("hydration projected stale Go fields into native storage/graph")
			}
			actor, err := records.ReadFollowerEntry(52)
			if err != nil || w.NativeEntries[0].Actor != actor || w.NativeFollowers[0].Actor != actor.Motion {
				t.Fatal("complete cleanup motion/owner/flags fields were not hydrated")
			}
			if w.Core.Peeps[0].Population != 0 || w.nativeRuntimeFollowerReserved(0) != (actor.Owner != 0) || w.NativeEntries[0].Managed != (actor.Owner != 0) {
				t.Fatal("retained population-zero cleanup actor lost allocation/dispatch ownership")
			}
			if w.Core.Magnets[fixture.Input.Owner-1].Carried != 0 && fixture.Input.Flags&1 != 0 {
				t.Fatal("cleanup leader removal was overwritten by Core projection")
			}
			if err := w.Occupancy.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRuntimeHydrationKeepsNativeDeathRuinWinnerAndHeroOwnership(t *testing.T) {
	for _, state := range []uint8{0x18, 0x40, 0x28, 0x30, 0x42, 0x24} {
		t.Run(fmt.Sprintf("state%02x", state), func(t *testing.T) {
			w := &World{Core: &legacy.World{Peeps: []legacy.Peep{{Population: 123, AtPos: 1, Flags: legacy.OnMove}}}}
			a := FollowerEntryActor{Owner: 1, Motion: FollowerMotionActor{Kind: 0x12, State: state, X: 0x20a0, Y: 0x2140, Next: 0, Previous: 0, Speed: 20, Animation: 0x178}}
			if state == 0x28 || state == 0x30 {
				a.Motion.Kind, a.Byte1 = 4, 18
			}
			if state == 0x42 || state == 0x24 {
				a.Motion.Kind, a.Motion.Population = 2, 1000
			}
			if state == 0x24 {
				a.Motion.Flags, a.Hero40 = 2, 2
			}
			if _, err := w.RecordImage.PatchFollowerEntry(52, FollowerEntryActor{}, a); err != nil {
				t.Fatal(err)
			}
			w.Occupancy.Followers[0].Linked = true
			before := w.RecordImage
			w.hydrateNativeRuntimeRecords()
			if w.RecordImage != before || w.NativeEntries[0].Actor.Motion.State != state || !w.NativeEntries[0].Managed || !w.nativeRuntimeFollowerReserved(0) || !w.Occupancy.Followers[0].Linked {
				t.Fatal("special native state was reset, freed, or unmapped")
			}
			if w.NativeFollowers[0].Active || w.Core.Peeps[0].AtPos != 32+33*64 {
				t.Fatal("special state was dispatched through ordinary motion or lost position")
			}
		})
	}
}

func TestRuntimeHydrationExtendsRawAllocatedCloneAndSynchronizesMarkers(t *testing.T) {
	w := &World{Core: &legacy.World{}}
	ref, _ := NativeWorldReference(NativeFollowerPool, 15)
	a := FollowerEntryActor{Owner: 2, Weapon: 7, Motion: FollowerMotionActor{Kind: 2, State: 2, ReturnState: 2, X: 0x1180, Y: 0x1280, Speed: 40, Population: 2000}}
	if _, err := w.RecordImage.PatchFollowerEntry(ref, FollowerEntryActor{}, a); err != nil {
		t.Fatal(err)
	}
	w.RecordImage.Write8(ref, 24, 12)
	memory := w.runtimeMemory()
	deity, _ := NativeDeityAddress(2)
	marker, _ := NativeMagnetReference(2)
	memory.Write32(deity, 123456)
	memory.Write16(deity+8, uint16(ref))
	memory.Write16(deity+10, uint16(marker))
	memory.Write16(cleanupRecordAddress(marker)+6, 0x2280)
	memory.Write16(cleanupRecordAddress(marker)+8, 0x2380)
	w.Occupancy.Magnets[2].Linked = true
	w.hydrateNativeRuntimeRecords()
	if len(w.Core.Peeps) != 16 || w.Core.Peeps[15].Population != 2000 || w.Core.Peeps[15].MovementSpeed != 40 || w.Core.Peeps[15].IQ != 12 || w.Core.Peeps[15].Player != 1 {
		t.Fatal("new raw clone was dropped or overwritten with inherited defaults")
	}
	if w.Core.Magnets[1].Mana != 123456 || w.Core.Magnets[1].Carried != 16 || w.Core.Magnets[1].GoTo != 34+35*64 || w.Occupancy.Magnets[2].Record.X != 0x2280 || !w.Occupancy.Magnets[2].Linked {
		t.Fatal("native deity/marker state did not hydrate the bridge")
	}
}

func TestRuntimeHydrationProjectsSceneryWallsAndRetiredEffectFields(t *testing.T) {
	w := &World{Core: &legacy.World{}, SceneryBank: testBundle(t).Scenery}
	tree := nativeActorReference(NativeSceneryPool, 0)
	w.RecordImage.Write8(tree, 0, 0x1e)
	w.RecordImage.Write8(tree, 1, 0xfd)
	w.RecordImage.Write8(tree, 12, 3)
	w.RecordImage.Write16(tree, 6, 0x2080)
	w.RecordImage.Write16(tree, 8, 0x2180)
	w.RecordImage.Write16(tree, 10, 0xf14)
	wall := nativeActorReference(NativeWallPool, 0)
	nextWall := nativeActorReference(NativeWallPool, 1)
	w.RecordImage.Write8(wall, 0, 0x1c)
	w.RecordImage.Write8(wall, 1, 8)
	w.RecordImage.Write8(wall, 12, 2)
	w.RecordImage.Write16(wall, 14, uint16(nextWall))
	fx := nativeActorReference(NativeEffectPool, 0)
	w.RecordImage.Write8(fx, 0, BasaltActorKind)
	w.RecordImage.Write8(fx, 22, 0x3a)
	w.RecordImage.Write16(fx, 24, 0)
	w.RecordImage.Write16(fx, 26, 4)
	records := w.RecordImage
	w.hydrateNativeRuntimeRecords()
	if w.RecordImage != records {
		t.Fatal("typed hydration normalized retained actor bytes")
	}
	a := w.Scenery[0]
	if !a.Active || !a.Removing || a.Kind != SceneryTree || a.Age != -3 || a.Animation != 0xf10 || a.Frame != 1 || a.X != 32 || a.Y != 33 {
		t.Fatalf("burning scenery hydration differs: %+v", a)
	}
	if a := w.Walls.Actors[0]; !a.Active || !a.Broken || a.Player != 1 || a.Variant != 8 || a.Next != 2 {
		t.Fatalf("wall fields/deity chain hydration differs: %+v", a)
	}
	if a := w.NativeEffects[0]; a.Active || a.Kind != BasaltActorKind || a.State != 0x3a || a.Life != 0 || w.BasaltState.Directions[0] != 4 {
		t.Fatalf("retired controller fields were discarded: %+v", a)
	}
}
