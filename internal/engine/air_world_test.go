package engine

import "testing"

func lightningWorld() *World {
	w := testFlatWorld()
	w.random = 4311
	w.Level.Players[0].Powers[Lightning] = true
	w.Players[0].Mana = 10000
	return w
}

func TestLightningWorldPlacementActivationAndFullPoolCost(t *testing.T) {
	w := lightningWorld()
	beforeMana, beforeRandom := w.Players[0].Mana, w.random
	if err := w.PlaceLightning(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mana != beforeMana || w.random != beforeRandom {
		t.Fatal("marker placement spent mana or randomness")
	}
	if err := w.ActivateLightning(0); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mana != beforeMana-w.PowerCost(0, Lightning) || !w.Air.Bolts[1].Active || !w.Air.Bolts[2].Active {
		t.Fatal("strike command did not charge normal mana and create its base volley")
	}
	w = lightningWorld()
	beforeMana = w.Players[0].Mana
	if err := w.ActivateLightning(0); err != nil || w.Players[0].Mana != beforeMana-w.PowerCost(0, Lightning) {
		t.Fatal("missing marker incorrectly refunded the admitted activation")
	}
	w = lightningWorld()
	w.PlaceLightning(0, 32, 32)
	for range EffectCapacity - 1 {
		w.allocateEffect(EffectFireColumn, 1)
	}
	beforeMana, beforeRandom = w.Players[0].Mana, w.random
	if err := w.ActivateLightning(0); err != nil || w.Players[0].Mana != beforeMana-w.PowerCost(0, Lightning) || w.random != beforeRandom {
		t.Fatal("full-pool activation lost source debit or drew jitter for absent bolts")
	}
}

func TestLightningWorldVictimLosesPopulationAndRecoversAfterDismissal(t *testing.T) {
	w := lightningWorld()
	id := addFollower(w, 32, 32, 1, 1000, Walking)
	w.Air.Bolts[0] = LightningBolt{Active: true, Owner: 0, X: 32*256 + 128, Y: 32*256 + 128}
	w.effects.Slots[0] = EffectReservation{Kind: EffectLightning, Owner: 0}
	w.tickAirEffect(0)
	if w.AirVictims[id].Phase != LightningVictimWalkingHit || w.Followers[id].Population != 1000 {
		t.Fatal("bolt scan incorrectly skipped or instantly killed its victim")
	}
	w.stepFollower(id)
	if w.Followers[id].Population != 934 {
		t.Fatal("lightning did not apply gradual signed population damage")
	}
	w.Air.Bolts[0].Active = false
	w.stepFollower(id)
	if w.AirVictims[id].Phase != LightningVictimRecovery || w.Followers[id].Population != 872 {
		t.Fatal("ending a bolt did not begin the survivor recovery sequence")
	}
	for range 10 {
		w.stepFollower(id)
	}
	if w.AirVictims[id].Phase != LightningVictimNone || w.Followers[id].State != Walking || w.Followers[id].Population != 872 {
		t.Fatal("recovery did not return a surviving walker without further damage")
	}
}

func TestLightningWorldOdysseusAndConsecrationRetainRelationWithoutDamage(t *testing.T) {
	for _, protected := range []bool{false, true} {
		w := lightningWorld()
		id := addFollower(w, 32, 32, 1, 1000, Walking)
		if protected {
			w.Followers[id].Consecrated = true
		} else {
			w.Followers[id].Hero.Kind = HeroOdysseus
		}
		w.Air.Bolts[0] = LightningBolt{Active: true, Owner: 0, X: 32*256 + 128, Y: 32*256 + 128}
		w.effects.Slots[0] = EffectReservation{Kind: EffectLightning, Owner: 0}
		w.tickAirEffect(0)
		if w.AirVictims[id].Bolt != 1 || w.AirVictims[id].Phase != LightningVictimNone || w.Followers[id].Population != 1000 {
			t.Fatal("immune victim lost its source relation or entered damage state")
		}
	}
}

func TestLightningWorldDeadVictimRetainsOccupancyForFinalAnimation(t *testing.T) {
	w := lightningWorld()
	id := addFollower(w, 32, 32, 1, 1, Walking)
	w.AirVictims[id] = LightningVictimState{Phase: LightningVictimWalkingHit, Bolt: 1, Frames: 2}
	w.stepFollower(id)
	if w.Followers[id].Population != 0 || w.Followers[id].State != Ruin || w.Occupants[32+32*MapSize] != uint16(id) || w.AirVictims[id].Phase != LightningVictimDeath {
		t.Fatal("dead walker did not retain its image and occupancy")
	}
	w.stepFollower(id)
	if w.Followers[id].State == Inactive {
		t.Fatal("death image was removed early")
	}
	w.stepFollower(id)
	if w.Followers[id].State != Inactive || w.Occupants[32+32*MapSize] != 0 {
		t.Fatal("death image failed to clean up occupancy")
	}
}
