package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestScenarioObserverAndVictimRulesAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/scenario_mixed_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Mode             string `json:"mode"`
			Player           int    `json:"player"`
			Observer         int    `json:"observer"`
			Options          uint16 `json:"options"`
			OtherOptions     uint16 `json:"other_options"`
			Population       int    `json:"population"`
			Attrition        uint16 `json:"attrition"`
			NativeStop       string `json:"native_stop"`
			NativeTile       uint8  `json:"native_tile"`
			NativePopulation int    `json:"native_population"`
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 400 {
		t.Fatal("native mixed-side fixture catalog is incomplete")
	}
	for index, fixture := range catalog.Fixtures {
		t.Run(fmt.Sprintf("%s-%d", fixture.Mode, index), func(t *testing.T) {
			w := flatGroundWorld(t)
			w.Rules[fixture.Player] = DecodeScenarioRules(fixture.Options)
			w.Rules[fixture.Player^1] = DecodeScenarioRules(fixture.OtherOptions)
			switch fixture.Mode {
			case "minimap-enemy":
				if w.FollowerVisibleOnMap(fixture.Observer, fixture.Player) != (fixture.NativeStop == "123e8") {
					t.Fatal("enemy-map visibility differs from native observer rule")
				}
			case "minimap-disaster":
				if w.EffectVisibleOnMap(fixture.Observer) != (fixture.NativeStop == "15af6") {
					t.Fatal("disaster-map visibility differs from native observer rule")
				}
			case "shallow-swamp":
				pos := 2000
				w.Marks[pos] = Mark{Spell: Swamp, Player: fixture.Player ^ 1, Life: 1, Persistent: true, NativeTile: 168}
				w.Core.Peeps = []legacy.Peep{{Player: byte(fixture.Player), Population: fixture.Population, AtPos: pos, Flags: legacy.OnMove}}
				w.applyGroundEffects()
				// The original fixture's untouched tile is 51. Only the native
				// shallow rule changes it to 15 before the death helper.
				if (w.Marks[pos].NativeTile == 0) != (fixture.NativeTile == 15) {
					t.Fatal("swamp persistence differs from native victim-side rule")
				}
			case "water-attrition":
				pos := 2000
				w.Level.Players[fixture.Player].Parameters[5] = fixture.Attrition
				w.Core.MapBlk[pos] = 0
				w.Core.Peeps = []legacy.Peep{{Player: byte(fixture.Player), Population: fixture.Population, AtPos: pos, Flags: legacy.OnMove | legacy.InWater}}
				w.Core.TickWithComputer([2]bool{})
				got := w.Core.Peeps[0].Population
				if fixture.NativeStop == "11d6a" {
					if got != 0 {
						t.Fatal("native fatal-water decision used observer options")
					}
				} else if got != fixture.NativePopulation {
					t.Fatal("native nonfatal-water survivor population differs")
				}
			default:
				t.Fatalf("unknown native fixture mode %q", fixture.Mode)
			}
		})
	}
}
