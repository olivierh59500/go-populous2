package engine

import "testing"

func TestLightningPlacementAndDismissalRemainFreeWithoutMana(t *testing.T) {
	w := &World{}
	w.Level.Players[0].Powers[Lightning] = true
	if err := w.Cast(0, Lightning, PowerTarget{X: 32, Y: 32}); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mana != 0 || w.Air.MarkerSlots[0] == 0 {
		t.Fatal("lightning marker spent mana or did not appear")
	}
	if err := w.Cast(0, Lightning, PowerTarget{Lower: true}); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mana != 0 {
		t.Fatal("dismissal spent mana")
	}
}

func TestAdmittedWhirlwindSpendsItsCost(t *testing.T) {
	w := &World{}
	w.Level.Players[0].Powers[Whirlwind] = true
	w.Players[0].Mana = 50000
	price := w.PowerCost(0, Whirlwind)
	if err := w.Cast(0, Whirlwind, PowerTarget{X: 32, Y: 32}); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mana != 50000-price {
		t.Fatal("whirlwind debit differs")
	}
}
