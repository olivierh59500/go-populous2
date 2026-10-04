package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

// The original routines were run independently with both owner records. The
// death cases stop before their animation helper, so those traces establish
// the decision to die rather than the full subsequent actor transition.
func TestScenarioOptionsAgainstOriginal68000Routines(t *testing.T) {
	data, err := os.ReadFile("testdata/scenario_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Mode             string `json:"mode"`
			Player           int    `json:"player"`
			Options          uint16 `json:"options"`
			Height           int    `json:"height"`
			Near             bool   `json:"near"`
			Mana             uint16 `json:"mana"`
			Attrition        uint16 `json:"attrition"`
			Population       int    `json:"population"`
			NativeAdmission  int    `json:"native_admission"`
			NativeMana       int    `json:"native_mana"`
			NativeAttrition  int    `json:"native_attrition"`
			NativePopulation uint32 `json:"native_population"`
			NativeStop       string `json:"native_stop"`
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 248 {
		t.Fatal("native scenario fixture catalog is incomplete")
	}
	for index, fixture := range catalog.Fixtures {
		t.Run(fmt.Sprintf("%s-%d", fixture.Mode, index), func(t *testing.T) {
			if fixture.Mode == "admission" {
				rules := DecodeScenarioRules(fixture.Options)
				if rules.TerrainEditAllowed(fixture.Height, true) != (fixture.NativeAdmission <= 0) || rules.TerrainEditAllowed(fixture.Height, false) != (fixture.NativeAdmission <= 1) {
					t.Fatalf("height admission differs from native D3=%d", fixture.NativeAdmission)
				}
				return
			}
			w := flatGroundWorld(t)
			w.Level.Players[fixture.Player].Parameters[4] = fixture.Mana
			w.Level.Players[fixture.Player].Parameters[5] = fixture.Attrition
			w.Rules[fixture.Player] = DecodeScenarioRules(fixture.Options)
			if fixture.Mode == "initialize-blue-fields" {
				w.initializeScenarioBalances()
				if w.Core.Magnets[fixture.Player].Mana != fixture.NativeMana || w.Core.FollowerAttrition(fixture.Player, false) != fixture.NativeAttrition {
					t.Fatal("zero-extended native template initialization differs")
				}
				return
			}
			pos := 2000
			peep := legacy.Peep{Player: byte(fixture.Player), Population: fixture.Population, AtPos: pos, Flags: legacy.IAmWaiting}
			if fixture.Mode == "water-attrition" {
				peep.Flags = legacy.OnMove | legacy.InWater
				w.Core.MapBlk[pos] = 0
			}
			w.Core.Peeps = []legacy.Peep{peep}
			w.Core.TickWithComputer([2]bool{})
			population := w.Core.Peeps[0].Population
			if fixture.Mode == "walking-attrition" {
				if int32(fixture.NativePopulation) <= 0 {
					if population != 0 {
						t.Fatal("native land attrition death decision differs")
					}
				} else if uint32(population) != fixture.NativePopulation {
					t.Fatal("native land attrition survivor population differs")
				}
			} else if fixture.NativeStop == "11d6a" {
				if population != 0 {
					t.Fatal("native fatal-water/attrition death decision differs")
				}
			} else if uint32(population) != fixture.NativePopulation {
				t.Fatal("native water attrition survivor population differs")
			}
		})
	}
}
