package engine

import "testing"

func TestCombatUsesOneAggressorQuotientForBothLosses(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 1000, Walking)
	d := addFollower(w, 21, 20, 1, 500, Walking)
	w.Followers[a].Weapons = 3
	w.Followers[d].Weapons = 7
	w.beginBattle(a, d)
	beforeRandom := w.random
	w.stepBattle(a)
	if w.Followers[a].Population != 920 || w.Followers[d].Population != 460 {
		t.Fatalf("source damage differs: %d/%d", w.Followers[a].Population, w.Followers[d].Population)
	}
	if w.random == beforeRandom {
		t.Fatal("combat did not advance shared RNG")
	}
	aPopulation, dPopulation := w.Followers[a].Population, w.Followers[d].Population
	w.stepBattle(d)
	if w.Followers[a].Population != aPopulation || w.Followers[d].Population != dPopulation {
		t.Fatal("defender incorrectly applied damage a second time")
	}
}

func TestMutualCombatDeathDoesNotAwardBattle(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 10, Walking)
	d := addFollower(w, 21, 20, 1, 10, Walking)
	w.beginBattle(a, d)
	w.stepBattle(a)
	if w.Followers[a].State != Inactive || w.Followers[d].State != Inactive || w.Players[0].BattlesWon != 0 || w.Players[1].BattlesWon != 0 {
		t.Fatal("mutual death winner or stale follower")
	}
}

func TestBattleNumericalReferenceCases(t *testing.T) {
	// Expected losses are scalar results of the original aggressor routine,
	// including its word-quotient overflow boundary, with weapons three/seven.
	for _, test := range []struct{ population, remaining int }{
		{1000, 920}, {6553600, 6553590}, {2147483647, 2147024892},
	} {
		w := testFlatWorld()
		a := addFollower(w, 20, 20, 0, test.population, Walking)
		d := addFollower(w, 21, 20, 1, 1000000, Walking)
		w.Followers[a].Weapons = 3
		w.Followers[d].Weapons = 7
		w.beginBattle(a, d)
		w.stepBattle(a)
		if got := w.Followers[a].Population; got != test.remaining {
			t.Fatalf("population %d: %d", test.population, got)
		}
	}
}
