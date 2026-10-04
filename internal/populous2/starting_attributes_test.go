package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestStartingAttributesAgainstNativeAllocation(t *testing.T) {
	data, err := os.ReadFile("testdata/initial_allocations_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Owner int
			P3    uint16
			Actor struct{ SearchIndex, Weapon, Speed, Population int }
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Fixtures) != 10 {
		t.Fatalf("invalid allocation fixture catalog: %v", err)
	}
	for _, fixture := range catalog.Fixtures {
		bundle := *testBundle(t)
		bundle.Levels = append([]Level(nil), bundle.Levels...)
		options := &bundle.Levels[0].Players[fixture.Owner-1]
		options.Parameters[1], options.Parameters[2], options.Parameters[3] = uint16(fixture.Actor.Population), uint16(fixture.Actor.Speed), fixture.P3
		w, err := NewWorld(&bundle, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, follower := range w.Core.Peeps {
			if int(follower.Player)+1 == fixture.Owner && (follower.IQ != fixture.Actor.SearchIndex || follower.Weapons != fixture.Actor.Weapon || int(follower.MovementSpeed) != fixture.Actor.Speed || follower.Population != fixture.Actor.Population) {
				t.Fatal("initial follower attributes differ from the native allocator")
			}
		}
	}
}

func TestStartingSearchIndexAndWeaponsUseSeparateNativeFields(t *testing.T) {
	for _, word := range []uint16{0, 1, 7, 255, 0xab07, 0xffff} {
		bundle := *testBundle(t)
		bundle.Levels = append([]Level(nil), bundle.Levels...)
		bundle.Levels[0].Players[0].Parameters[3] = word
		bundle.Levels[0].Players[1].Parameters[3] = word ^ 0x00ff
		w, err := NewWorld(&bundle, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, follower := range w.Core.Peeps {
			want := int(uint8(bundle.Levels[0].Players[follower.Player].Parameters[3]))
			if follower.IQ != 2 || follower.Weapons != want {
				t.Fatalf("template %04x owner %d: search=%d weapons=%d; want search2 weapons%d", word, follower.Player, follower.IQ, follower.Weapons, want)
			}
		}
		restored, err := Restore(&bundle, w.Snapshot())
		if err != nil {
			t.Fatal(err)
		}
		for index, follower := range w.Core.Peeps {
			if restored.Core.Peeps[index].IQ != follower.IQ || restored.Core.Peeps[index].Weapons != follower.Weapons {
				t.Fatal("saved initial search/weapon fields were recombined")
			}
		}
	}
}
