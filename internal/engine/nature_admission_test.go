package engine

import "testing"

func TestNatureCastAdmissionUsesEarnedManaAndScenarioFlags(t *testing.T) {
	for _, power := range []PowerID{Trees, Flowers, Swamp, Fungus} {
		w := &World{}
		for owner := range w.Players {
			w.Level.Players[owner].Powers[power] = true
		}
		before := *w
		if err := w.Cast(0, power, PowerTarget{X: 32, Y: 32}); err == nil {
			t.Fatal("unfunded power was admitted", power)
		}
		if *w != before {
			t.Fatal("unfunded cast changed game state", power)
		}
		w.Players[0].Mana = 100000
		w.Level.Players[0].Powers[power] = false
		before = *w
		if err := w.Cast(0, power, PowerTarget{X: 32, Y: 32}); err == nil {
			t.Fatal("disabled power was admitted", power)
		}
		if *w != before {
			t.Fatal("disabled cast changed game state", power)
		}
	}
}

func TestNatureNoOpChargePolicyMatchesEachPower(t *testing.T) {
	for _, power := range []PowerID{Trees, Flowers, Swamp, Fungus} {
		w := &World{} // All-water terrain has no eligible target parcels.
		w.Level.Players[0].Powers[power] = true
		w.Players[0].Mana = 100000
		price := w.PowerCost(0, power)
		if err := w.Cast(0, power, PowerTarget{X: 32, Y: 32}); err != nil {
			t.Fatal("admitted no-op cast failed", power, err)
		}
		expected := 100000 - price
		if power == Trees {
			expected = 100000
		}
		if w.Players[0].Mana != expected {
			t.Fatal("wrong no-op mana charge", power, w.Players[0].Mana, expected)
		}
	}
}
