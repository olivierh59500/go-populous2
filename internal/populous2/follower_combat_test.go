package populous2

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestFollowerCombatAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_combat_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Input struct {
				Name, Phase string
				Seed        uint32
			}
			Before, After         []string
			RNG                   uint32
			Exit                  string
			Cleanup, CleanupModes []uint16
			Winner, Loser         uint16
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 79 {
		t.Fatal("native combat catalog incomplete")
	}
	rules, err := DecodeFollowerCombatRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := entryFixtureMemory{}
			for index, raw := range fixture.Before {
				data, err := hex.DecodeString(raw)
				if err != nil || len(data) != 52 {
					t.Fatal("invalid native record fixture")
				}
				copy(m.bytes[0x76f4+index*52:], data)
			}
			seed := fixture.Input.Seed
			cleanup, modes := []uint16{}, []uint16{}
			winner, loser := NativeRecordReference(0), NativeRecordReference(0)
			cb := FollowerCombatCallbacks{
				Read: m.read, Write: m.write,
				Random: func() int {
					if seed == 0 {
						seed = 0x00bc614e
					}
					seed *= 0xbb40e62d
					return int(seed >> 8 & 0x7fff)
				},
				Cleanup: func(ref NativeRecordReference, mode uint16) error {
					cleanup = append(cleanup, uint16(ref))
					modes = append(modes, mode)
					a, err := m.read(ref)
					if err != nil {
						return err
					}
					a.Owner = 0
					a.Motion.Population = 0
					return m.write(ref, a)
				},
				Win: func(w, l NativeRecordReference) error { winner, loser = w, l; return nil },
			}
			var step FollowerCombatStep
			if fixture.Input.Phase == "aggressor" {
				step, err = rules.TickAggressor(52, cb)
			} else {
				step, err = rules.TickDefender(52, cb)
			}
			if err != nil {
				t.Fatal(err)
			}
			for index, raw := range fixture.After {
				got := hex.EncodeToString(m.bytes[0x76f4+index*52 : 0x76f4+(index+1)*52])
				if got != raw {
					t.Fatalf("record%d: Go%s native%s", index, got, raw)
				}
			}
			if seed != fixture.RNG || uint16(winner) != fixture.Winner || uint16(loser) != fixture.Loser || !reflect.DeepEqual(cleanup, fixture.Cleanup) || !reflect.DeepEqual(modes, fixture.CleanupModes) {
				t.Fatalf("native RNG/callback boundary differs %+v", step)
			}
			if fixture.Exit == "123b4" && !step.CurrentTotal || fixture.Exit == "12462" && !step.NextFollower || fixture.Exit == "1131c" && !step.RedispatchSearch {
				t.Fatalf("native dispatch exit%s differs %+v", fixture.Exit, step)
			}
			if fixture.Exit == "1298c" && (step.Winner != winner || step.Loser != loser || step.NextFollower != (loser == 52)) {
				t.Fatalf("native winner dispatch differs %+v", step)
			}
		})
	}
}

func TestFollowerCombatMissingExternalWinnerIsError(t *testing.T) {
	r, err := DecodeFollowerCombatRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	records := map[NativeRecordReference]FollowerEntryActor{52: {Owner: 1, Weapon: 3, Contact30: 104, Motion: FollowerMotionActor{State: 14, Animation: 0x1c8, Population: 1000}}, 104: {Owner: 2, Weapon: 7, Contact30: 52, Motion: FollowerMotionActor{State: 16, Population: 1}}}
	cb := FollowerCombatCallbacks{Read: func(ref NativeRecordReference) (FollowerEntryActor, error) {
		a, ok := records[ref]
		if !ok {
			return a, fmt.Errorf("unknown record")
		}
		return a, nil
	}, Write: func(ref NativeRecordReference, a FollowerEntryActor) error { records[ref] = a; return nil }, Random: func() int { return 0 }}
	if _, err := r.TickAggressor(52, cb); err == nil {
		t.Fatal("native winner was replaced by a silent generic result")
	}
}
