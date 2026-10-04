package populous

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOlympianChildAttributesAgainstNativeEmigration(t *testing.T) {
	data, err := os.ReadFile("../populous2/testdata/initial_emigration_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			TownStage, TownWeapon int
			Actor                 struct{ SearchIndex, Weapon, Speed int }
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Fixtures) != 19 {
		t.Fatalf("invalid native emigration catalog: %v", err)
	}
	for _, fixture := range catalog.Fixtures {
		w := &World{Peeps: []Peep{{Weapons: fixture.TownWeapon, IQ: 99, MovementSpeed: uint8(fixture.Actor.Speed)}}}
		w.initializeOlympianChild(0, fixture.TownStage)
		child := w.Peeps[0]
		if child.Weapons != fixture.Actor.Weapon || child.IQ != fixture.Actor.SearchIndex || int(child.MovementSpeed) != fixture.Actor.Speed {
			t.Fatalf("stage %d: child attributes differ from original emigration", fixture.TownStage)
		}
	}
}
