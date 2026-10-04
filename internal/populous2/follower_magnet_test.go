package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeMagnetInput struct {
	Name, Entry string
	Initial     []nativeHeroPatch
}
type nativeMagnetCall struct {
	Kind                  string
	Source, Target, Value uint16
}
type nativeMagnetFixture struct {
	Input      nativeMagnetInput
	Hash, Stop string
	Changes    []nativeHeroChange
	Calls      []nativeMagnetCall
	RNG        uint32
}

func magnetFixtureMemory(input nativeMagnetInput) *cleanupMemory {
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 1)
		m.putByte(0xf45+index*4, 15)
	}
	for _, record := range []struct {
		address    int
		flags      uint8
		population uint32
		x, y       uint16
	}{{0x76f4, 0, 1000, 0x2087, 0x2093}, {0x7728, 1, 500, 0x2241, 0x2157}} {
		a := record.address
		m.putByte(a, 2)
		m.putByte(a+12, 1)
		m.putByte(a+13, record.flags)
		m.putWord(a+6, record.x)
		m.putWord(a+8, record.y)
		m.putWord(a+10, 0x168)
		m.putWord(a+14, 17)
		m.putWord(a+16, 23)
		m.putByte(a+18, 16)
		m.putByte(a+19, 9)
		m.putWord(a+20, 6)
		m.putByte(a+22, 0x12)
		_ = m.write32(a+26, record.population)
		m.putWord(a+50, 4)
	}
	m.putByte(0xe74e+12, 1)
	m.putWord(0xe74e+6, 0x2225)
	m.putWord(0xe74e+8, 0x212f)
	m.putWord(0xe8a4+12, 16)
	m.putWord(0xe8a4+10, 0x708e)
	_ = m.write32(0xe8a4+20, 3)
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
	for _, ref := range []NativeRecordReference{52, 104, 0x708e} {
		if err := m.insert(ref); err != nil {
			m.err = err
		}
	}
	if m.byte(0x76f4+22) == 0x3a && m.word(0x76f4+10) == 0 {
		m.putWord(0x76f4+10, 0x168)
	}
	return m
}

// Complete native dispatch compares raw records, deity words and graph state,
// including real merges and retained cleanup. It does not use PlanWalkerStep.
func TestFollowerMagnetAgainstNativeDispatch(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_magnet_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeMagnetFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 663 {
		t.Fatal("native magnet catalog incomplete")
	}
	bundle := testBundle(t)
	rules, err := DecodeFollowerMagnetRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := DecodeFollowerEntryRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := magnetFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m, ref) }
			write := func(ref NativeRecordReference, a FollowerEntryActor) error { return winEntryWrite(m, ref, a) }
			calls := []nativeMagnetCall{}
			clear := func(ref NativeRecordReference, tile uint8) error {
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				calls = append(calls, nativeMagnetCall{Kind: "cleanup", Source: uint16(ref), Value: mode})
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: clear})
				return err
			}
			merge := func(source, target NativeRecordReference) error {
				calls = append(calls, nativeMagnetCall{Kind: "merge", Source: uint16(source), Target: uint16(target)})
				return entry.merge(source, target, FollowerEntryCallbacks{Read: read, Write: write, ClearHeroLinks: func(ref NativeRecordReference) error { _, err := ClearEntryHeroLinks(&m.records, ref); return err }, SetLeader: func(owner uint8, ref NativeRecordReference) error {
					return m.write16(heroGodAddress(owner)+8, uint16(ref))
				}, Selected: func() NativeRecordReference { return 0 }, Select: func(NativeRecordReference) error { return fmt.Errorf("unexpected selection") }, Unlink: m.unlink})
			}
			cb := FollowerMagnetCallbacks{Hero: FollowerHeroCallbacks{Memory: memory, RaiseEnabled: func() bool { return false }, Raise: func(uint8, uint8) error { return fmt.Errorf("unexpected direct raise") }}, Attrition: FollowerAttritionCallbacks{Read: read, Write: write, Cleanup: cleanup}, Merge: merge}
			var step FollowerMagnetStep
			var err error
			switch fixture.Input.Entry {
			case "decision":
				step, err = rules.Decide(52, cb)
			case "search":
				first, e := ApplyFollowerAttrition(52, uint32(m.word(0xe8a4+20))<<16|uint32(m.word(0xe8a4+22)), cb.Attrition)
				if e != nil {
					t.Fatal(e)
				}
				if first.Died {
					step.Redispatch = true
				} else {
					step, err = rules.Decide(52, cb)
				}
			default:
				step, err = rules.Tick(52, cb)
			}
			if err != nil {
				t.Fatal(err)
			}
			stop := ""
			switch {
			case step.Search:
				stop = "search"
			case step.Redispatch:
				stop = "redispatch"
			case step.CountPopulation:
				stop = "count"
			case step.NextFollower:
				stop = "next"
			}
			if stop != fixture.Stop {
				t.Fatalf("native dispatch target got%s want%s", stop, fixture.Stop)
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("native callback ordering got%v want%v", calls, fixture.Calls)
			}
			var image [NativeRuntimeImageEnd]byte
			copy(image[:NativeRecordImageStart], m.lower[:])
			copy(image[NativeRecordImageStart:NativeMagnetImageStart], m.records.Bytes[:])
			copy(image[NativeMagnetImageStart:], m.globals.Bytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(image[:])); got != fixture.Hash {
				for _, change := range fixture.Changes {
					if image[change.Address] != change.Value {
						t.Errorf("native byte%x got%x want%x", change.Address, image[change.Address], change.Value)
					}
				}
				t.Fatalf("complete native magnet BSS differs: got%s want%s", got, fixture.Hash)
			}
			if fixture.RNG != 4311 || m.err != nil {
				t.Fatal("native magnet fixture RNG or memory differs")
			}
		})
	}
}
