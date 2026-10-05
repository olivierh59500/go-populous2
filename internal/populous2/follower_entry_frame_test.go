package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type entryFrameFixture struct {
	entryFixture
	Input struct {
		entryFixtureInput
		D [8]uint32
	}
	D    [8]uint32
	Hash string
}

func nativeEntryFrameFixtures(t *testing.T) []entryFrameFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/follower_entry_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []entryFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 150 {
		t.Fatalf("native full-entry corpus differs: %d cases", len(catalog.Fixtures))
	}
	return catalog.Fixtures
}

func TestNativeEntryFullFrameAgainstOriginalCPU(t *testing.T) {
	fixtures := nativeEntryFrameFixtures(t)
	b := testBundle(t)
	rules, err := DecodeNativeFollowerEntryFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := DecodeFollowerEntryRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	towns, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		m := entryMemory(fixture.Input.entryFixtureInput)
		// The original selected pointer is an actual BSS address; this raw
		// adapter keeps the same reference/address convention in its backing.
		if fixture.Input.Leader {
			m.putLong(0xf36, 0x200000+0x76f4)
		}
		frame := NativeFrameRegisterContext{D: fixture.Input.D, AddressBase: 0x200000}
		state := NativeFollowerEntryFrameState{}
		memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
		townCB := townCombatEvaluatorCallbacks(m)
		farms := func(ref NativeRecordReference, tile uint8) error { return evaluator.ClearFarms(ref, tile, townCB) }
		legacyCB := m.callbacks(t, fixture.entryFixture)
		legacyCB.ClearFarms = farms
		legacyCB.EvaluateTown = func(ref NativeRecordReference) (int, error) {
			stage, err := evaluator.Evaluate(ref, 17, townCB)
			return int(stage), err
		}
		legacyCB.ClearHeroLinks = func(ref NativeRecordReference) error {
			var image NativeRecordImage
			copy(image.Bytes[:], m.bytes[NativeRecordImageStart:NativeRecordImageEnd])
			_, err := ClearEntryHeroLinks(&image, ref)
			copy(m.bytes[NativeRecordImageStart:NativeRecordImageEnd], image.Bytes[:])
			return err
		}
		legacyCB.Selected = func() NativeRecordReference {
			value := m.long(0xf36)
			if value == 0 {
				return 0
			}
			return NativeRecordReference(uint16(value - 0x200000 - 0x76c0))
		}
		legacyCB.Select = func(ref NativeRecordReference) error {
			m.putLong(0xf36, 0x200000+uint32(cleanupRecordAddress(ref)))
			return nil
		}
		cb := NativeFollowerEntryFrameCallbacks{Memory: memory, Frame: &frame, State: &state,
			Merge: func(source, target NativeRecordReference, c *NativeFrameRegisterContext) error {
				return entry.MergeWithFrame(source, target, c, legacyCB)
			},
			Battle: func(source, target NativeRecordReference, c *NativeFrameRegisterContext) error {
				_, err := PrepareFollowerContactWithFrame(source, target, c, FollowerContactCallbacks{Memory: memory, ClearFarms: farms, Sound: func(uint16) error { return nil }})
				return err
			},
			ReformTown: func(ref NativeRecordReference) error {
				_, err := towns.Reform(ref, ref, 17, TownCombatCallbacks{Read: m.read, Write: m.write, ClearFarms: farms, EvaluateTown: legacyCB.EvaluateTown, Cleanup: func(ref NativeRecordReference, mode uint16) error {
					_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: func(ref NativeRecordReference) error { return aftermathUnlink(m, ref) }, Insert: func(ref NativeRecordReference) error { m.insert(cleanupRecordAddress(ref)); return nil }, ClearFarms: farms})
					return err
				}})
				return err
			},
		}
		boundary, err := rules.Enter(52, cb)
		if err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		wantBoundary := map[string]uint32{"render": 0x123b4, "next-follower": 0x12462, "new-town-update": 0x11738}[fixture.Exit]
		if boundary != wantBoundary || frame.D != fixture.D || state.WallObserved != fixture.WallObserved {
			t.Fatalf("%s entry continuation differs: boundary%x/%x D%x/%x wall%d/%d", fixture.Input.Name, boundary, wantBoundary, frame.D, fixture.D, state.WallObserved, fixture.WallObserved)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[:])); got != fixture.Hash {
			for _, record := range fixture.Records {
				at := cleanupRecordAddress(NativeRecordReference(record.Reference))
				t.Logf("record%x actual%x expected%s", record.Reference, m.bytes[at:at+len(record.Raw)/2], record.Raw)
			}
			t.Fatalf("%s raw BSS differs: %s / %s", fixture.Input.Name, got, fixture.Hash)
		}
	}
}

func TestWorldEntryFullFrameAgainstOriginalCPU(t *testing.T) {
	rules, err := DecodeNativeFollowerEntryFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range nativeEntryFrameFixtures(t) {
		m := entryMemory(fixture.Input.entryFixtureInput)
		if fixture.Input.Leader {
			m.putLong(0xf36, 0x200000+0x76f4)
		}
		raw := append([]byte(nil), m.bytes[:]...)
		w := installNativeFixtureWorld(t, raw, nil)
		frame := NativeFrameRegisterContext{D: fixture.Input.D, AddressBase: 0x200000}
		state := NativeFollowerEntryFrameState{}
		w.nativeCallDepth++
		boundary, err := rules.Enter(52, w.nativeEntryFrameCallbacks(&frame, &state))
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		wantBoundary := map[string]uint32{"render": 0x123b4, "next-follower": 0x12462, "new-town-update": 0x11738}[fixture.Exit]
		if boundary != wantBoundary || frame.D != fixture.D || state.WallObserved != fixture.WallObserved {
			t.Fatalf("World%s continuation differs: boundary%x/%x D%x/%x wall%d/%d", fixture.Input.Name, boundary, wantBoundary, frame.D, fixture.D, state.WallObserved, fixture.WallObserved)
		}
		all := nativeFixtureWorldImage(w, raw)
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != fixture.Hash {
			t.Fatalf("World%s BSS differs: %s/%s", fixture.Input.Name, got, fixture.Hash)
		}
	}
}
