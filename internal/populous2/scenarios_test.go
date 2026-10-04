package populous2

import "testing"

func TestNativeScenarioOptionBitNumberingAndPolarity(t *testing.T) {
	properties := func(r ScenarioRules) [10]bool {
		return [10]bool{r.BuildAnywhere, r.BuildAnywhereAtSeaLevel, r.ForbidEnemyTerrain,
			r.ForbidRaise, r.ForbidLower, r.FatalWater, r.HideEnemyOnMap,
			r.DisableRightClickSprog, r.HideDisastersOnMap, r.ShallowSwamps}
	}
	for bit := 0; bit < 16; bit++ {
		raw := uint16(1) << bit
		rules := DecodeScenarioRules(raw)
		if rules.Raw != raw {
			t.Fatalf("option bit %d was not preserved", bit)
		}
		for option, set := range properties(rules) {
			if set != (option == bit) {
				t.Fatalf("raw %04x: displayed option %d is %t", raw, option, set)
			}
		}
	}
	all := DecodeScenarioRules(0xffff)
	for option, set := range properties(all) {
		if !set {
			t.Fatalf("displayed option %d absent from all-bit word", option)
		}
	}
	if all.Raw != 0xffff {
		t.Fatal("unidentified option bits were discarded")
	}
}

func TestScenarioOptionsComeFromIndividualCampaignTemplate(t *testing.T) {
	var players [2]PlayerOptions
	players[0].Parameters[6] = 0x0023
	players[1].Parameters[6] = 0x01dc
	blue, red := players[0].ScenarioRules(), players[1].ScenarioRules()
	if !blue.BuildAnywhere || !blue.BuildAnywhereAtSeaLevel || !blue.FatalWater ||
		blue.ForbidRaise || blue.ForbidLower || blue.ForbidEnemyTerrain || blue.DisableRightClickSprog {
		t.Fatalf("native first-world option word 35 decoded incorrectly: %+v", blue)
	}
	if red.BuildAnywhere || red.BuildAnywhereAtSeaLevel || !red.ForbidEnemyTerrain ||
		!red.ForbidRaise || !red.ForbidLower || red.FatalWater ||
		!red.HideEnemyOnMap || !red.DisableRightClickSprog || !red.HideDisastersOnMap {
		t.Fatalf("independent opposing option word decoded incorrectly: %+v", red)
	}
	if players[0].Parameters[6] != 0x0023 || players[1].Parameters[6] != 0x01dc {
		t.Fatal("decoding altered the campaign data")
	}
}

func TestScenarioUnknownBitsDoNotInventDisplayedRules(t *testing.T) {
	known := DecodeScenarioRules(0x0023)
	unknown := DecodeScenarioRules(0xfc23)
	if unknown.Raw != 0xfc23 {
		t.Fatal("unlabelled native bits were not retained")
	}
	unknown.Raw = known.Raw
	if unknown != known {
		t.Fatal("an unlabelled bit was assigned an invented rule")
	}
}

func TestOriginalFirstCampaignWorldHasNativeConstructionPermissions(t *testing.T) {
	b := testBundle(t)
	rules := b.Levels[0].Players[0].ScenarioRules()
	if rules.Raw != 35 || !rules.BuildAnywhere || !rules.BuildAnywhereAtSeaLevel || !rules.FatalWater {
		t.Fatalf("original DOEGAC rules were not retained: %+v", rules)
	}
	if rules.ForbidRaise || rules.ForbidLower || rules.ForbidEnemyTerrain || rules.DisableRightClickSprog {
		t.Fatal("an absent first-world prohibition was invented")
	}
}
