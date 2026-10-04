package populous2

import "testing"

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
