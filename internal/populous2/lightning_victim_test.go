package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type nativeLightningVictimState struct {
	Kind, Owner, Flags, State uint8
	Animation                 uint16
	Population                int32
	Reference                 uint16
	Calls                     []string
	Total                     uint32
}
type nativeLightningVictimFixture struct {
	Input struct {
		Name, Mode                string
		Kind, State, Flags, Owner uint8
		Hero                      int
		Population                int32
		BoltOwner, BoltKind       uint8
		Animation                 uint16
		Ticks                     int
	}
	Initial nativeLightningVictimState
	Trace   []nativeLightningVictimState
}

func TestLightningVictimsAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/lightning_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		VictimFixtures []nativeLightningVictimFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.VictimFixtures) != 205 {
		t.Fatal("native lightning victim catalog incomplete")
	}
	rules, err := DecodeLightningRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range catalog.VictimFixtures {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			initial := fixture.Initial
			v := LightningVictim{Kind: initial.Kind, Owner: initial.Owner, Flags: initial.Flags, State: initial.State, Animation: int(initial.Animation), Population: initial.Population, EffectReference: NativeRecordReference(initial.Reference)}
			if fixture.Input.Hero >= 0 {
				v.HeroType = uint16(fixture.Input.Hero * 2)
			}
			var pool [NativeEffectCapacity]NativeEffectActor
			pool[0] = NativeEffectActor{Active: true, Kind: 42, X: 8320, Y: 8320, State: 24}
			var state LightningState
			for _, expected := range fixture.Trace {
				calls := []string{}
				if fixture.Input.Mode == "bind" {
					cb := LightningCallbacks{Random: func() int { return 0 }, Head: func(int, int) NativeRecordReference { return 52 }, Record: func(NativeRecordReference) (LightningVictim, bool) { return v, true }, SetRecord: func(_ NativeRecordReference, record LightningVictim) { v = record }}
					if err := rules.Tick(&state, &pool, 0, cb); err != nil {
						t.Fatal(err)
					}
				} else {
					_, err := rules.TickVictim(&v, LightningVictimCallbacks{
						BoltAlive: func(NativeRecordReference) bool { return int8(fixture.Input.BoltOwner) > 0 },
						Cleanup: func(retained bool) {
							calls = append(calls, "124a2")
							v.Population = 0
							if retained {
								calls = append(calls, "14654")
							} else {
								// Native town cleanup restores farms, detaches hero
								// metadata, and unlinks the record inside $124a2.
								// Those are caller-owned operations in this helper.
								calls = append(calls, "135ca", "14654", "125da")
								v.Owner = 0
							}
						},
						Remove: func() { calls = append(calls, "125da"); v.Owner = 0; v.Population = 0 },
						ReformTown: func() {
							calls = append(calls, "12bd8")
							if v.Flags&2 != 0 {
								v.Kind, v.State, v.Animation = 2, 2, 0
							} else {
								calls = append(calls, "13352")
								v.Kind, v.State, v.Flags = 4, 6, 0
							}
						},
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				got := nativeLightningVictimState{Kind: v.Kind, Owner: v.Owner, Flags: v.Flags, State: v.State, Animation: uint16(v.Animation), Population: v.Population, Reference: uint16(v.EffectReference)}
				want := expected
				want.Calls = nil
				want.Total = 0
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go %+v; native %+v", got, want)
				}
				// The original callbacks invoke additional leader/farm helpers. Verify
				// the caller-owned cleanup/reform boundary without claiming their work.
				if fixture.Input.Mode != "totals" && !reflect.DeepEqual(calls, expected.Calls) {
					t.Fatalf("Go callbacks%v; native%v", calls, expected.Calls)
				}
			}
			checked++
		})
	}
	if checked != 205 {
		t.Fatal("not every native victim case was replayed")
	}
}
