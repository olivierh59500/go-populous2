package engine

import "testing"

func TestBasaltCastChargesOnlyAnAdmittedSharedEffect(t *testing.T) {
	w := &World{}
	w.Level.Players[0].Powers[Basalt] = true
	w.Players[0].Mana = 5000
	price := w.PowerCost(0, Basalt)
	if err := w.Cast(0, Basalt, PowerTarget{X: 32, Y: 32, Direction: 1}); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mana != 5000-price || !w.Water.Painted[32+32*MapSize] {
		t.Fatal("admitted basalt debit or terrain missing")
	}
	mana := w.Players[0].Mana
	if err := w.Cast(0, Basalt, PowerTarget{X: 32, Y: 32, Direction: 1}); err == nil {
		t.Fatal("existing basalt accepted a new initial cast")
	}
	if w.Players[0].Mana != mana {
		t.Fatal("rejected basalt spent mana")
	}
	for tick := 0; tick < 30; tick++ {
		w.tickWaterEffects()
	}
	if cell := w.Cell(33, 32); cell.Code != 224 {
		t.Fatal("eastward basalt did not persist", cell.Code)
	}
}
