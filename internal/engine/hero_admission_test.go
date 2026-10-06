package engine

import "testing"

func TestAllHeroCommandsUseLeaderAndPreserveAdmissionState(t *testing.T) {
	for _, power := range []PowerID{Perseus, Adonis, Heracles, Odysseus, Achilles, Helen} {
		w := &World{}
		w.Level.Players[0].Powers[power] = true
		w.Players[0].Mana = 1000000
		before := *w
		if err := w.Cast(0, power, PowerTarget{}); err == nil {
			t.Fatal("missing leader admitted hero", power)
		}
		if *w != before {
			t.Fatal("rejected hero changed simulation", power)
		}
		w.Followers[1] = Follower{State: Walking, Owner: 0, X: 32, Y: 32, Population: 1000, MovementSpeed: 20}
		w.Players[0].Leader = 1
		price := w.PowerCost(0, power)
		if err := w.Cast(0, power, PowerTarget{}); err != nil {
			t.Fatal(power, err)
		}
		kind, _ := HeroKindForPower(power)
		if w.Followers[1].Hero.Kind != kind || w.Players[0].Leader != 0 || w.Players[0].Mana != 1000000-price {
			t.Fatal("hero creation or debit missing", power)
		}
	}
}
