package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The fixture input and comparison structures are shared with the original
// Storm proof because both controllers use the genuine $16542 victim scan.
func fireRainFixtureMemory(input nativeStormInput) *cleanupMemory {
	setup := input
	setup.Mode = "create"
	setup.Initial = append([]nativeHeroPatch(nil), input.Initial...)
	setup.Links = append([]uint16(nil), input.Links...)
	if input.Mode == "tick" {
		patches := []nativeHeroPatch{}
		for i := range 32 {
			patches = append(patches, nativeHeroPatch{Address: 0xc800 + i, Width: 1, Value: 0})
		}
		patches = append(patches, nativeHeroPatch{Address: 0xc800, Width: 1, Value: 0x2c}, nativeHeroPatch{Address: 0xc80c, Width: 1, Value: uint32(uint8(input.Owner))}, nativeHeroPatch{Address: 0xc816, Width: 1, Value: 0x1c}, nativeHeroPatch{Address: 0xc806, Width: 2, Value: uint32(input.X)<<8 | 128}, nativeHeroPatch{Address: 0xc808, Width: 2, Value: uint32(input.Y)<<8 | 128}, nativeHeroPatch{Address: 0xc80a, Width: 2, Value: 0x81c}, nativeHeroPatch{Address: 0xc818, Width: 2, Value: 24})
		setup.Initial = append(patches, setup.Initial...)
		phase := uint8(0x1c)
		for _, patch := range input.Initial {
			if patch.Address == 0xc816 && patch.Width == 1 {
				phase = uint8(patch.Value)
			}
		}
		if phase != 0x1c {
			setup.Links = append(setup.Links, 0x5140)
		}
	}
	return stormFixtureMemory(setup)
}

func TestFireRainAgainstCompleteOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/fire_rain_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeStormFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 809 {
		t.Fatal("native fire rain fixture catalog incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeFireRainRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	storm, err := DecodeStormRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if rules.BaseCount != 32 || len(rules.Frames) != 26 {
		t.Fatal("native fire rain tables differ")
	}
	found := false
	for _, action := range b.Actions {
		if action.Command == 38 {
			if action.Spell != FireRain || action.Handler != 0x178d8 {
				t.Fatal("native fire rain action identity differs")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("native fire rain command38 missing")
	}
	creators, updates := 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := fireRainFixtureMemory(f.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			before := append([]byte(nil), m.lower[:]...)
			before = append(before, m.records.Bytes[:]...)
			before = append(before, m.globals.Bytes[:]...)
			mem := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return m.records.ReadFollowerEntry(ref) }
			write := func(ref NativeRecordReference, a FollowerEntryActor) error {
				old, err := read(ref)
				if err != nil {
					return err
				}
				_, err = m.records.PatchFollowerEntry(ref, old, a)
				return err
			}
			rng, draws, damage := f.Input.Seed, 0, 0
			calls := []nativeStormCall{}
			link := func(ref NativeRecordReference) error {
				calls = append(calls, nativeStormCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}
			unlink := func(ref NativeRecordReference) error {
				calls = append(calls, nativeStormCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}
			farms := func(ref NativeRecordReference, tile uint8) error {
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}
			destroy := func(ref NativeRecordReference) error {
				calls = append(calls, nativeStormCall{Kind: "destroy", Reference: uint16(ref)})
				_, err := combat.Destroy(ref, TownCombatCallbacks{Read: read, Write: write, ClearFarms: farms, Cleanup: func(ref NativeRecordReference, mode uint16) error {
					_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: mem, Unlink: unlink, Insert: link, ClearFarms: farms})
					return err
				}})
				return err
			}
			cb := FireRainCallbacks{Memory: mem, Link: link, Unlink: unlink, Random: func() uint16 {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				return uint16(rng >> 8 & 0x7fff)
			},
				Scorch: func(ref NativeRecordReference) error {
					calls = append(calls, nativeStormCall{Kind: "scorch", Reference: uint16(ref)})
					return storm.Scorch(ref, mem)
				},
				Damage: func(ref NativeRecordReference) (uint16, error) {
					damage++
					calls = append(calls, nativeStormCall{Kind: "damage", Reference: uint16(ref)})
					return storm.Damage(ref, StormCallbacks{Memory: mem, DestroyTown: destroy})
				},
			}
			if f.Input.Mode == "create" {
				creators++
				step, err := rules.Create(f.Input.Owner, f.Input.X, f.Input.Y, cb)
				if err != nil {
					t.Fatal(err)
				}
				if step.Admitted != f.Admitted || step.Attempts != f.Attempts || step.RandomDraws != f.RandomDraws || step.PoolFull != (!f.Admitted) {
					t.Fatalf("original rain allocation/admission counters differ: %+v want%v/%d/%d", step, f.Admitted, f.Attempts, f.RandomDraws)
				}
			} else {
				if len(f.Trace) == 0 {
					t.Fatal("native rain runtime trace empty")
				}
				for _, snapshot := range f.Trace {
					updates++
					_, err := rules.Tick(0x5140, cb)
					if err != nil {
						t.Fatal(err)
					}
					if stormFixtureHash(m) != snapshot.Hash || rng != snapshot.RNG || draws != snapshot.RandomDraws || damage != snapshot.DamageScans {
						t.Fatalf("native rain update%d complete image/RNG differs", snapshot.Update)
					}
				}
			}
			if stormFixtureHash(m) != f.Hash || rng != f.RNG || draws != f.RandomDraws || damage != f.DamageScans {
				t.Fatal("complete original fire rain image/RNG/counters differ")
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Fatalf("native rain ordered external calls differ: got%+v want%+v", calls, f.Calls)
			}
			after := append([]byte(nil), m.lower[:]...)
			after = append(after, m.records.Bytes[:]...)
			after = append(after, m.globals.Bytes[:]...)
			changes := []nativeHeroChange{}
			for i, v := range before {
				if v != after[i] {
					changes = append(changes, nativeHeroChange{Address: i, Value: after[i]})
				}
			}
			if !reflect.DeepEqual(changes, f.Changes) {
				t.Fatal("original rain changed byte ranges differ")
			}
		})
	}
	if creators != 384 || updates != 1450 {
		t.Fatalf("native fire rain coverage differs: %d creators/%d updates", creators, updates)
	}
}

func TestFireRainNativeFallingLayerOffsets(t *testing.T) {
	r, err := DecodeFireRainRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{-112, -104, -98, -92, -86, -80, -74, -68, -62, -56, -50, -44, -38, -32, -26, -20, -14, -8}
	for i, y := range want {
		frame, ok := r.Frames[0x81c+i*4]
		if !ok || len(frame.Layers) == 0 || frame.Layers[0].X != 1 || frame.Layers[0].Y != y {
			t.Fatalf("native fire rain falling layer%d differs", i)
		}
	}
}
