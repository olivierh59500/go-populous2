package engine

import "testing"

func TestFireWorldRetainsVictimsOccupancyUntilDeathAnimationFinishes(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 32, 32, 0, 100, Walking)
	b := addFollower(w, 32, 32, 1, 100, Walking)
	outside := addFollower(w, 33, 32, 1, 100, Walking)
	w.Players[0].Leader = a
	if hits := w.BurnFireCell(32, 32); hits != 2 {
		t.Fatalf("fire damage hit %d actors, want both allegiances", hits)
	}
	if w.Followers[outside].Population != 100 || w.Cell(32, 32).Code != 95 || w.NatureTownAllowed(0, 32, 32) {
		t.Fatal("fire crossed a parcel boundary or scorched land still supported a town")
	}
	for range 8 {
		w.stepFollower(a)
		w.stepFollower(b)
	}
	if w.Followers[a].State == Inactive || w.Followers[b].State == Inactive || w.Occupants[32+32*MapSize] == 0 {
		t.Fatal("dying actors released their allocation or occupancy too early")
	}
	w.stepFollower(a)
	w.stepFollower(b)
	if w.Followers[a].State != Inactive || w.Followers[b].State != Inactive || w.Occupants[32+32*MapSize] != 0 || w.Players[0].Leader != 0 {
		t.Fatal("completed fire death did not perform full follower cleanup")
	}
}

func TestFireWorldAllHeroImmunitiesAndTreeSpread(t *testing.T) {
	for _, kind := range []HeroKind{HeroPerseus, HeroAdonis, HeroHeracles, HeroOdysseus, HeroAchilles, HeroHelen} {
		w := testFlatWorld()
		id := addFollower(w, 32, 32, 0, 100, Walking)
		w.Followers[id].Hero.Kind = kind
		hits := w.BurnFireCell(32, 32)
		if kind == HeroAchilles {
			if hits != 0 || w.Followers[id].Population != 100 || w.FireDamage.Deaths[id].Mode != FireVictimAlive {
				t.Fatal("Achilles lost his fire immunity")
			}
		} else if hits != 1 || w.FireDamage.Deaths[id].Frames != 9 {
			t.Fatalf("hero %d did not retain the direct-fire death lifecycle", kind)
		}
		w = testFlatWorld()
		id = addFollower(w, 32, 32, 0, 100, Walking)
		w.Followers[id].Hero.Kind = kind
		if w.SpreadTreeFireCell(32, 32) != 0 || w.Followers[id].Population != 100 {
			t.Fatalf("neighboring tree fire incorrectly damaged hero %d", kind)
		}
	}
}

func TestFireWorldTownDestructionClearsOnlyItsOwnedFootprint(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 1000, Town)
	w.Followers[id].Stage = 18
	for _, d := range townFootprint {
		w.Farms[32+d[0]+(32+d[1])*MapSize] = 1
	}
	w.Farms[30+30*MapSize] = 2
	w.Farms[40+40*MapSize] = 1
	if w.BurnFireCell(32, 32) != 1 || w.FireDamage.Deaths[id].Mode != FireVictimTownRuin || w.FireDamage.Deaths[id].Frames != 13 {
		t.Fatal("stage eighteen town did not enter its original ruin animation")
	}
	for _, d := range townFootprint {
		at := 32 + d[0] + (32+d[1])*MapSize
		if at == 30+30*MapSize {
			if w.Farms[at] != 2 || w.Nature.Ground[at].Mark == GroundScorched {
				t.Fatal("neighboring enemy farm was cleared with the destroyed town")
			}
			continue
		}
		if w.Farms[at] != 0 || w.Nature.Ground[at].Mark != GroundScorched {
			t.Fatal("destroyed town retained owned farmland")
		}
	}
	if w.Farms[40+40*MapSize] != 1 {
		t.Fatal("town destruction crossed its exact support footprint")
	}
}

func TestFireWorldSharedPoolReusesVelocityAndKeepsAdmissionCostSeparate(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	w.Players[0].Mana = 123456
	w.effects.Slots[0].LastVelocityX, w.effects.Slots[0].LastVelocityY = 17, -19
	if err := w.CastFireColumn(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	e := w.Fire.Columns[0]
	if e.VX != 17 || e.VY != -19 || w.Players[0].Mana != 123456 {
		t.Fatal("fire creation lost shared recycled velocities or charged outside Cast")
	}
	for range EffectCapacity - 1 {
		w.allocateEffect(EffectStorm, 1)
	}
	before := w.Fire
	if err := w.CastFireColumn(0, 32, 32); err == nil || w.Fire != before || w.Players[0].Mana != 123456 {
		t.Fatal("full-pool admission changed existing effects or the mana ledger")
	}
}

func TestFireWorldLavaBurningDamagesPopulationGradually(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 1000, Walking)
	w.pushFireParcel(32, 32, 20, 0)
	if w.Followers[id].Population != 1000 || w.FireDamage.Deaths[id].Mode != FireVictimBurning {
		t.Fatal("lava entry killed a follower instead of beginning gradual burning")
	}
	w.stepFollower(id)
	if w.Followers[id].Population != 934 || w.Followers[id].State == Inactive {
		t.Fatal("lava burning did not apply population>>4 plus four")
	}
	for pass := 0; pass < 200 && w.Followers[id].State != Inactive; pass++ {
		w.stepFollower(id)
	}
	if w.Followers[id].State != Inactive || w.Occupants[32+32*MapSize] != 0 {
		t.Fatal("depleted burning follower retained occupancy")
	}
}
