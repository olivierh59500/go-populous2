package engine

import "testing"

func TestFireAdmissionCannotSpendUnavailableMana(t *testing.T) {
	for _, power := range []PowerID{FireColumn, FireRain} {
		w := &World{}
		w.Level.Players[0].Powers[power] = true
		before := *w
		if err := w.Cast(0, power, PowerTarget{X: 32, Y: 32}); err == nil || *w != before {
			t.Fatal("unfunded fire cast changed world", power, err)
		}
		w.Players[0].Mana = 1000000
		for i := 0; i < EffectCapacity; i++ {
			w.allocateEffect(EffectWhirlwind, 1)
		}
		mana := w.Players[0].Mana
		if err := w.Cast(0, power, PowerTarget{X: 32, Y: 32}); err == nil {
			t.Fatal("exhausted shared pool admitted fire power", power)
		}
		if w.Players[0].Mana != mana {
			t.Fatal("failed reservation spent mana", power)
		}
	}
}

func TestAdmittedFireCastSpendsItsExperienceAdjustedCost(t *testing.T) {
	for _, power := range []PowerID{FireColumn, FireRain} {
		w := &World{}
		w.Level.Players[0].Powers[power] = true
		w.Players[0].Mana = 1000000
		w.Players[0].Experience[Fire] = 128
		price := w.PowerCost(0, power)
		if err := w.Cast(0, power, PowerTarget{X: 32, Y: 32}); err != nil {
			t.Fatal(power, err)
		}
		if w.Players[0].Mana != 1000000-price {
			t.Fatal("wrong fire debit", power, w.Players[0].Mana, price)
		}
		count := 0
		for _, slot := range w.effects.Slots {
			if slot.Kind == EffectFireColumn || slot.Kind == EffectFireRain {
				count++
			}
		}
		if count == 0 {
			t.Fatal("paid cast produced no effect", power)
		}
	}
}
