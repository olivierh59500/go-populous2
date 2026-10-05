package populous2

import (
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestAdonisSplitsAfterBattleRatherThanAddingPopulation(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	index := w.Core.Magnets[0].Carried - 1
	w.Core.Magnets[0].Mana = 1000000
	if !w.Cast(0, Adonis, Target{}) {
		t.Fatal("hero cast rejected")
	}
	w.Core.Peeps[index].Population = 101
	before := len(w.Core.Peeps)
	w.Core.OnBattleWon(index, 0)
	if len(w.Core.Peeps) != before+1 || w.Core.Peeps[index].Population != 50 || w.Core.Peeps[before].Population != 50 || !w.Heroes[before].Active || w.Heroes[before].Spell != Adonis {
		t.Fatal("native Adonis split missing")
	}
	w.Core.Peeps[index].Population = 20
	if w.splitAdonis(index) {
		t.Fatal("twenty-person Adonis split")
	}
	restored, err := Restore(testBundle(t), w.Snapshot())
	if err != nil || restored.Core.OnBattleWon == nil {
		t.Fatalf("save lost hero combat binding: %v", err)
	}
}

func TestHelenCapturesWithoutChangingFaith(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove, Status: legacy.KnightStatus}, {Player: 1, Population: 100, AtPos: 2001, Flags: legacy.InTown}}
	w.Heroes[0] = Hero{Spell: Helen, Active: true, Player: 0, Population: 100}
	if !w.captureByHelen(0, 1) || w.Core.Peeps[1].Player != 1 || len(w.Heroes[0].Captives) != 1 {
		t.Fatal("Helen converted faith instead of abducting")
	}
	if !w.Core.SkipFollower(1) {
		t.Fatal("captive retained ordinary combat/movement")
	}
	w.followHelenCaptives()
	if w.Core.Peeps[1].AtPos != 2000 || w.Core.Peeps[1].Player != 1 {
		t.Fatal("captive did not follow with its own faith")
	}
	s := w.Snapshot()
	s.Heroes[0].Captives[0] = 0
	if w.Heroes[0].Captives[0] != 1 {
		t.Fatal("snapshot aliases captives")
	}
	s = w.Snapshot()
	restored, err := Restore(testBundle(t), s)
	if err != nil || !restored.Core.SkipFollower(1) {
		t.Fatalf("save lost capture bindings: %v", err)
	}
	w.Heroes[0].Active = false
	w.followHelenCaptives()
	if w.Core.SkipFollower(1) {
		t.Fatal("dead Helen retained captive control")
	}
}

func TestHelenWaterAndNativeSwampHeroTable(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove, Status: legacy.KnightStatus}, {Player: 0, Population: 100, AtPos: 2001, Flags: legacy.OnMove, Status: legacy.KnightStatus}}
	w.Heroes[0] = Hero{Spell: Helen, Active: true, Player: 0, Population: 100}
	w.Heroes[1] = Hero{Spell: Adonis, Active: true, Player: 0, Population: 100}
	w.bindHeroCombat()
	if !w.Core.CanHeroCrossWater(0) || w.Core.CanHeroCrossWater(1) {
		t.Fatal("native Helen water immunity missing")
	}
	w.Marks[2000] = Mark{Spell: Swamp, Player: 1, Life: 1, Persistent: true, NativeTile: 168}
	w.Marks[2001] = w.Marks[2000]
	tickGroundPrepassForTest(t, w)
	if w.Core.Peeps[0].Population != 0 || w.Core.Peeps[1].Population != 100 {
		t.Fatal("native table must retain Helen death and spare Adonis in swamp")
	}
}
