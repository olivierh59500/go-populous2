package engine

import "testing"

func TestPlagueAdmissionRequiresOpposingFollowers(t *testing.T) {
	w := &World{}
	w.Level.Players[0].Powers[Plague] = true
	w.Players[0].Mana = 1000000
	if err := w.Cast(0, Plague, PowerTarget{X: 32, Y: 32}); err == nil || w.Players[0].Mana != 1000000 {
		t.Fatal("empty plague cast spent mana", err)
	}
	w.Followers[1] = Follower{State: Walking, Owner: 1, X: 32, Y: 32, Population: 1000}
	price := w.PowerCost(0, Plague)
	if err := w.Cast(0, Plague, PowerTarget{X: 32, Y: 32}); err != nil {
		t.Fatal(err)
	}
	if !w.Followers[1].Disease.Infected || w.Followers[1].Population != 1000 || w.Players[0].Mana != 1000000-price {
		t.Fatal("plague admission, original zero damage or debit changed")
	}
}

func TestArmageddonAndBaptismUseGlobalAdmissionCosts(t *testing.T) {
	for _, power := range []PowerID{Armageddon, Baptism} {
		w := &World{}
		w.Level.Players[0].Powers[power] = true
		w.Players[0].Mana = 1000000
		price := w.PowerCost(0, power)
		if err := w.Cast(0, power, PowerTarget{X: 32, Y: 32}); err != nil {
			t.Fatal(power, err)
		}
		if w.Players[0].Mana != 1000000-price {
			t.Fatal("global or admitted no-op cost changed", power)
		}
		if power == Armageddon && !w.Armageddon {
			t.Fatal("Armageddon policy was not enabled")
		}
	}
}
