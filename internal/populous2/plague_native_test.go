package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type plagueNativeInput struct {
	Name, Mode     string
	Owner          uint16
	X, Y           uint8
	Initial        []nativeHeroPatch
	Links          []uint16
	Source, Target uint16
}
type plagueNativeCall struct {
	Kind            string
	Reference, Mode uint16
}
type plagueNativeFixture struct {
	Input    plagueNativeInput
	Hash     string
	Changes  []nativeHeroChange
	Admitted bool
	Calls    []plagueNativeCall
	RNG      uint32
}

func plagueNativeMemory(input plagueNativeInput) *cleanupMemory {
	m := &cleanupMemory{}
	for i := range 4096 {
		m.putByte(0xf44+i*4, 0xa8)
		m.putByte(0xf45+i*4, 15)
		m.putByte(0x4f44+i, 0x77)
	}
	for owner := 0; owner < 3; owner++ {
		god, marker := 0xe76a+owner*314, 0xe740+owner*14
		m.putWord(god+10, uint16(marker-0x76c0))
		m.putWord(god+8, input.Source)
		m.putWord(god+0x44, 120)
		m.putByte(marker, 20)
		m.putByte(marker+12, uint8(owner))
		m.putWord(marker+6, uint16(10+owner)<<8|128)
		m.putWord(marker+8, 0x0a80)
	}
	for _, p := range input.Initial {
		switch p.Width {
		case 1:
			m.putByte(p.Address, uint8(p.Value))
		case 2:
			m.putWord(p.Address, uint16(p.Value))
		case 4:
			if err := m.write32(p.Address, p.Value); err != nil {
				m.err = err
			}
		}
	}
	refs := append([]uint16{0x7080, 0x708e, 0x709c}, input.Links...)
	for _, ref := range refs {
		if err := m.insert(NativeRecordReference(ref)); err != nil {
			m.err = err
		}
	}
	return m
}

func TestPlagueAgainstOriginal68000CreatorRuntimeAndInheritance(t *testing.T) {
	data, err := os.ReadFile("testdata/plague_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []plagueNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1360 {
		t.Fatal("native plague fixture catalog incomplete")
	}
	b := testBundle(t)
	r, err := DecodePlagueRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if r.Damage != 0 || len(r.Frames) == 0 {
		t.Fatal("native plague table values differ")
	}
	found := false
	for _, action := range b.Actions {
		if action.Command == 78 {
			if action.Spell != Plague || action.Handler != 0x17bba {
				t.Fatal("native plague command identity differs")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("native plague command78 missing")
	}
	counts := map[string]int{}
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := plagueNativeMemory(f.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			before := append([]byte(nil), m.lower[:]...)
			before = append(before, m.records.Bytes[:]...)
			before = append(before, m.globals.Bytes[:]...)
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []plagueNativeCall{}
			link := func(ref NativeRecordReference) error {
				calls = append(calls, plagueNativeCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}
			unlink := func(ref NativeRecordReference) error {
				calls = append(calls, plagueNativeCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}
			farms := func(ref NativeRecordReference, tile uint8) error {
				calls = append(calls, plagueNativeCall{Kind: "farms", Reference: uint16(ref), Mode: uint16(tile)})
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}
			cb := PlagueCallbacks{Memory: memory, Cleanup: func(ref NativeRecordReference, mode uint16) error {
				calls = append(calls, plagueNativeCall{Kind: "cleanup", Reference: uint16(ref), Mode: mode})
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: unlink, Insert: link, ClearFarms: farms})
				return err
			}}
			source, target := NativeRecordReference(f.Input.Source), NativeRecordReference(f.Input.Target)
			switch f.Input.Mode {
			case "create":
				step, err := r.Create(f.Input.Owner, f.Input.X, f.Input.Y, cb)
				if err != nil {
					t.Fatal(err)
				}
				if step.Admitted != f.Admitted {
					t.Fatal("native plague creator admission differs")
				}
			case "tick":
				if _, err := r.Tick(source, cb); err != nil {
					t.Fatal(err)
				}
			case "merge":
				if err := r.InheritMerge(source, target, cb); err != nil {
					t.Fatal(err)
				}
			case "birth":
				if err := r.InheritBirth(source, target, cb); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatal("unknown native plague boundary")
			}
			if stormFixtureHash(m) != f.Hash {
				t.Fatal("complete native plague raw image differs")
			}
			if f.RNG != 4311 {
				t.Fatal("original plague unexpectedly consumes randomness")
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Fatalf("native plague external callback order differs: got%+v want%+v", calls, f.Calls)
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
				t.Fatal("native plague changed byte ranges differ")
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"create": 386, "tick": 360, "merge": 96, "birth": 518}) {
		t.Fatalf("native plague scope coverage differs: %v", counts)
	}
}
