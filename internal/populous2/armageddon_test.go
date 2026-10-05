package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type armageddonActor struct {
	Slot                                    int
	Owner, Kind, State, Flags, Stage, Speed uint8
	Population                              uint32
}
type armageddonInput struct {
	Name, Mode     string
	Owner, Enabled uint16
	Seed           uint32
	Actors         []armageddonActor
}
type armageddonFixture struct {
	Input       armageddonInput
	Hash        string
	Changes     []nativeHeroChange
	Calls       []heroCreationCall
	RNG         uint32
	RandomDraws int
}

func armageddonFixtureMemory(input armageddonInput) *cleanupMemory {
	m := &cleanupMemory{}
	for i := range 4096 {
		m.putByte(0xf44+i*4, 0xa8)
		m.putByte(0xf45+i*4, 15)
		m.putByte(0x4f44+i, 0x77)
	}
	for owner := 0; owner < 3; owner++ {
		god, marker := 0xe76a+owner*314, 0xe740+owner*14
		m.putWord(god+10, uint16(marker-0x76c0))
		m.putWord(god+0x44, 120)
		if err := m.write32(god, 1000000); err != nil {
			m.err = err
		}
		for j := range 6 {
			m.putByte(god+0x52+j, uint8(24+j*32))
		}
		m.putByte(marker, 20)
		m.putByte(marker+12, uint8(owner))
		m.putWord(marker+6, uint16(10+owner)<<8|128)
		m.putWord(marker+8, 0x0a80)
		if err := m.insert(NativeRecordReference(marker - 0x76c0)); err != nil {
			m.err = err
		}
	}
	for _, a := range input.Actors {
		at := 0x76c0 + a.Slot*52
		for j := range 52 {
			m.putByte(at+j, uint8(j*13+7))
		}
		m.putWord(at+2, 0)
		m.putWord(at+4, 0)
		m.putByte(at, a.Kind)
		m.putByte(at+1, a.Stage)
		m.putByte(at+12, a.Owner)
		m.putByte(at+13, a.Flags)
		m.putByte(at+22, a.State)
		m.putByte(at+18, a.Speed)
		if err := m.write32(at+26, a.Population); err != nil {
			m.err = err
		}
		m.putWord(at+6, 0x2087)
		m.putWord(at+8, 0x2193)
		m.putWord(at+34, 0)
		m.putWord(at+36, 0)
		m.putWord(at+40, 0)
		m.putWord(at+42, 0)
		if a.Owner <= 2 {
			m.putWord(0xe76a+int(a.Owner)*314+8, uint16(a.Slot*52))
		}
		if err := m.insert(NativeRecordReference(a.Slot * 52)); err != nil {
			m.err = err
		}
	}
	m.putWord(0xf12, input.Enabled)
	return m
}

func TestArmageddonAgainstOriginalCompleteGlobalBody(t *testing.T) {
	data, err := os.ReadFile("testdata/armageddon_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []armageddonFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 720 {
		t.Fatal("native armageddon fixture catalog incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeArmageddonRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	heroes, err := DecodeHeroRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range b.Actions {
		if a.Command == 72 {
			if a.Spell != Armageddon || a.Handler != 0x17b02 {
				t.Fatal("native armageddon command identity differs")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("native armageddon command72 missing")
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := armageddonFixtureMemory(f.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			before := append([]byte(nil), m.lower[:]...)
			before = append(before, m.records.Bytes[:]...)
			before = append(before, m.globals.Bytes[:]...)
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []heroCreationCall{}
			link := func(ref NativeRecordReference) error {
				calls = append(calls, heroCreationCall{Kind: "link", Ref: uint16(ref)})
				return m.insert(ref)
			}
			unlink := func(ref NativeRecordReference) error {
				calls = append(calls, heroCreationCall{Kind: "unlink", Ref: uint16(ref)})
				return m.unlink(ref)
			}
			farms := func(ref NativeRecordReference, tile uint8) error {
				calls = append(calls, heroCreationCall{Kind: "farms", Ref: uint16(ref), Value: uint16(tile)})
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}
			sound := func(raw uint16) error { calls = append(calls, heroCreationCall{Kind: "sound", Value: raw}); return nil }
			leader := func(ref NativeRecordReference) error {
				calls = append(calls, heroCreationCall{Kind: "leader", Ref: uint16(ref)})
				_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: memory, Unlink: unlink, Insert: link})
				return err
			}
			rng, draws := f.Input.Seed, 0
			cb := ArmageddonCallbacks{Memory: memory, Sound: sound, Random: func() uint16 {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				return uint16(rng >> 8 & 0x7fff)
			},
				Cleanup: func(ref NativeRecordReference, mode uint16) error {
					calls = append(calls, heroCreationCall{Kind: "cleanup", Ref: uint16(ref), Value: mode})
					_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: unlink, Insert: link, ClearFarms: farms})
					return err
				},
				Convert: func(ref NativeRecordReference, hero uint16) error {
					calls = append(calls, heroCreationCall{Kind: "convert", Ref: uint16(ref), Value: hero})
					if hero > 6 || hero&1 != 0 {
						t.Fatal("armageddon chooses outside first four heroes")
					}
					return heroes.Convert(ref, heroIDs[hero/2], HeroCreationCallbacks{Memory: memory, ClearLeader: leader, ClearFarms: farms, Sound: sound})
				},
				Debit: func(owner, power uint16) error {
					calls = append(calls, heroCreationCall{Kind: "debit", Ref: owner, Value: power})
					return nil
				},
			}
			var step ArmageddonStep
			if f.Input.Mode == "command" {
				step, err = rules.Command(f.Input.Owner, cb)
			} else {
				step, err = rules.Cast(cb)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !step.Admitted || step.AlreadyEnabled != (f.Input.Enabled != 0) || step.Enabled != (f.Input.Enabled == 0) || step.RandomDraws != f.RandomDraws || draws != f.RandomDraws {
				t.Fatal("native armageddon admission/global state/counters differ")
			}
			if stormFixtureHash(m) != f.Hash || rng != f.RNG {
				t.Fatal("complete native armageddon global BSS/RNG differs")
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Fatalf("native armageddon ordered callbacks differ: got%+v want%+v", calls, f.Calls)
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
				t.Fatal("native armageddon changed byte ranges differ")
			}
		})
	}
}
