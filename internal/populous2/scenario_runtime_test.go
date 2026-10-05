package populous2

import (
	"bytes"
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestScenarioTerrainAdmissionDoesNotRequireNearbyFollowers(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = nil
	w.Core.MapWho = [4096]uint16{}
	if !w.Sculpt(0, 32, 32, true) {
		t.Fatal("DOEGAC unrestricted edit required nearby followers")
	}
	w.Rules[0] = DecodeScenarioRules(1 << 1)
	if w.Sculpt(0, 32, 32, true) {
		t.Fatal("sea-only rule raised a high vertex")
	}
	w.Core.Alt = [65 * 65]int{}
	if !w.Sculpt(0, 32, 32, true) || w.Core.Alt[32+32*65] != 1 {
		t.Fatal("sea-only rule rejected zero-height vertex")
	}
	if !w.Sculpt(0, 32, 32, false) {
		t.Fatal("sea-only lowering rejected height one")
	}
	w.Rules[0] = DecodeScenarioRules(0)
	before := encodeSnapshot(t, w)
	if w.Sculpt(0, 32, 32, true) || !bytes.Equal(before, encodeSnapshot(t, w)) {
		t.Fatal("disabled construction edited terrain")
	}
}

func TestScenarioProhibitionsAndEnemyPropagationAreAtomic(t *testing.T) {
	w := flatGroundWorld(t)
	w.Rules[0] = DecodeScenarioRules(1 | 1<<3)
	if w.Sculpt(0, 32, 32, true) {
		t.Fatal("forbid-raise ignored")
	}
	w.Rules[0] = DecodeScenarioRules(1 | 1<<4)
	if w.Sculpt(0, 32, 32, false) {
		t.Fatal("forbid-lower ignored")
	}
	w.Rules[0] = DecodeScenarioRules(1 | 1<<2)
	w.Core.MapBlk[32+32*64] = legacy.FarmBlock + 1
	before := encodeSnapshot(t, w)
	if w.Sculpt(0, 32, 32, true) || !bytes.Equal(before, encodeSnapshot(t, w)) {
		t.Fatal("enemy terrain edit changed heights, farms, or mana")
	}
	w.Rules[1] = DecodeScenarioRules(1)
	w.Core.Magnets[1].Mana = 100000
	if !w.Sculpt(1, 32, 32, true) {
		t.Fatal("same-owner permitted edit rejected")
	}
}

func TestScenarioFatalWaterAppliesIndependentlyPerSide(t *testing.T) {
	w := flatGroundWorld(t)
	w.Level.Players[1].Parameters[5] = 7
	w.Rules[0] = DecodeScenarioRules(1 << 5)
	w.Rules[1] = DecodeScenarioRules(0)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove | legacy.InWater}, {Player: 1, Population: 100, AtPos: 2001, Flags: legacy.OnMove | legacy.InWater}}
	w.Core.MapBlk[2000] = 0
	w.Core.MapBlk[2001] = 0
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].Population != 0 || w.Core.Peeps[1].Population <= 0 || w.Core.Peeps[1].Population >= 100 {
		t.Fatal("one global fatal-water rule applied to both sides")
	}
}

func TestScenarioStartingManaUsesEachNativeTemplateWord(t *testing.T) {
	bundle := *testBundle(t)
	bundle.Levels = append([]Level(nil), bundle.Levels...)
	bundle.Levels[0].Players[0].Parameters[4] = 1234
	bundle.Levels[0].Players[1].Parameters[4] = 65535
	w, err := NewWorld(&bundle, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if w.Core.Magnets[0].Mana != 1234 || w.Core.Magnets[1].Mana != 65535 {
		t.Fatal("starting mana used an inherited default or scaled the native word")
	}
	w.Core.Magnets[0].Mana = 91
	restored, err := Restore(&bundle, w.Snapshot())
	if err != nil || restored.Core.Magnets[0].Mana != 91 {
		t.Fatalf("loading reset earned/spent mana to the scenario initial word: %v", err)
	}
}

func TestScenarioWaterAttritionUsesVictimWordWithoutDoubling(t *testing.T) {
	w := flatGroundWorld(t)
	w.Level.Players[0].Parameters[5] = 1
	w.Level.Players[1].Parameters[5] = 7
	w.Rules = [2]ScenarioRules{}
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove | legacy.InWater}, {Player: 1, Population: 100, AtPos: 2001, Flags: legacy.OnMove | legacy.InWater}}
	w.Core.MapBlk[2000], w.Core.MapBlk[2001] = 0, 0
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].Population != 99 || w.Core.Peeps[1].Population != 93 {
		t.Fatalf("nonfatal water used another camp or doubled native attrition: %+v", w.Core.Peeps)
	}
	restored, err := Restore(testBundle(t), w.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	for player, options := range restored.Level.Players {
		if restored.Core.FollowerAttrition(player, true) != options.FollowerAttrition() {
			t.Fatal("save restoration lost scenario attrition binding")
		}
	}
}

func TestScenarioSprogVisibilityAndShallowSwampUseCorrectSide(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.InTown}}
	w.Rules[0] = DecodeScenarioRules(1<<7 | 1<<6 | 1<<8)
	if w.Sprog(0, 2000%64, 2000/64) || w.Core.Peeps[0].ForceEmigration {
		t.Fatal("disabled sprog accepted")
	}
	if !w.FollowerVisibleOnMap(0, 0) || w.FollowerVisibleOnMap(0, 1) || !w.FollowerVisibleOnMap(1, 0) || w.EffectVisibleOnMap(0) || !w.EffectVisibleOnMap(1) {
		t.Fatal("visibility used actor options rather than observer options")
	}
	w.Rules[0] = DecodeScenarioRules(1 << 4)
	if !w.Sprog(0, 2000%64, 2000/64) {
		t.Fatal("lower prohibition blocked enabled right-click release")
	}
	w.Core.Peeps[0].Flags = legacy.OnMove
	w.Core.Peeps[0].Player = 1
	w.Rules[1] = DecodeScenarioRules(1 << 9)
	w.Marks[2000] = Mark{Spell: Swamp, Player: 0, Life: 1, Persistent: true, NativeTile: 168}
	tickGroundPrepassForTest(t, w)
	if w.Core.Peeps[0].Population != 0 || w.Marks[2000].NativeTile != 0 {
		t.Fatal("shallow swamp did not use victim-side rule")
	}
}

func TestScenarioOptionsSurviveSave(t *testing.T) {
	w := flatGroundWorld(t)
	w.Rules[0] = DecodeScenarioRules(0x0441)
	w.Rules[1] = DecodeScenarioRules(0x8202)
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Rules != w.Rules || restored.Core.WaterFatalForPlayer(0) != w.Rules[0].FatalWater {
		t.Fatal("save lost separate option words, unknown bits or runtime binding")
	}
}

func TestComputerTerrainCommandsUseNativeRulesAndPrice(t *testing.T) {
	w := flatGroundWorld(t)
	w.Rules[1] = DecodeScenarioRules(1 | 1<<3)
	w.Core.Magnets[1].Mana = 10000
	if w.Core.TerrainCommand(1, 32, 32, true) {
		t.Fatal("computer terrain command ignored native prohibition")
	}
	w.Rules[1] = DecodeScenarioRules(1)
	before := w.Core.Magnets[1].Mana
	if !w.Core.TerrainCommand(1, 32, 32, true) || w.Core.Magnets[1].Mana != before-w.ManaCost(1, RaiseLower) {
		t.Fatal("computer command used inherited terrain price")
	}
}
