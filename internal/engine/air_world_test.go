package engine

import (
	"encoding/json"
	"os"
	"testing"
)

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

func TestWhirlwindWorldCarriesFractionalPositionAndLandsWithoutLosingPopulation(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	id := addFollower(w, 32, 32, 1, 1234, Walking)
	w.Followers[id].Weapons = 7
	w.Air.Whirlwinds[0] = WhirlwindEffect{Active: true, Owner: 0, X: 32*256 + 128, Y: 32*256 + 128, Life: 200}
	w.effects.Slots[0] = EffectReservation{Kind: EffectWhirlwind, Owner: 0}
	w.liftAirFollowers(0, 32, 32)
	if w.Air.Carry[id].Phase != AirCarryFlying || w.Followers[id].State != Airborne || w.Followers[id].Weapons != 1 || w.Followers[id].Population != 1234 {
		t.Fatal("whirlwind lift changed identity/population or omitted airborne state")
	}
	w.Air.Whirlwinds[0].X, w.Air.Whirlwinds[0].Y = 34*256+211, 31*256+17
	rng := w.random
	w.stepFollower(id)
	if w.Followers[id].positionX != 34*256+211 || w.Followers[id].positionY != 31*256+17 || w.random != rng || w.Occupants[34+31*MapSize] != uint16(id) {
		t.Fatal("airborne transport lost source fractions, occupancy or consumed randomness")
	}
	w.releaseAirFollowers(0, 34, 31)
	if w.Air.Carry[id].Phase != AirCarryLanding || w.Followers[id].Population != 1234 || w.Followers[id].Owner != 1 {
		t.Fatal("drop did not retain its follower identity")
	}
	for range 6 {
		w.stepFollower(id)
		if w.Followers[id].State != Airborne {
			t.Fatal("landing completed before its seventh dispatch")
		}
	}
	w.stepFollower(id)
	if w.Followers[id].State != Walking || w.Air.Carry[id].Phase != AirCarryNone || w.Followers[id].Weapons != 1 || w.Followers[id].Population != 1234 {
		t.Fatal("landing did not return a normal walker with retained weapon and population")
	}
}

func TestWhirlwindWorldImmunityFarmReleaseAndWaterBirth(t *testing.T) {
	w := testFlatWorld()
	ordinary := addFollower(w, 32, 32, 0, 1000, Town)
	w.Followers[ordinary].Stage = 18
	w.repaintFarms()
	hero := addFollower(w, 33, 32, 1, 1000, Walking)
	w.Followers[hero].Hero.Kind = HeroOdysseus
	protected := addFollower(w, 31, 32, 1, 1000, Walking)
	w.Followers[protected].Consecrated = true
	w.liftAirFollowers(0, 32, 32)
	w.liftAirFollowers(0, 33, 32)
	w.liftAirFollowers(0, 31, 32)
	if w.Air.Carry[ordinary].Phase != AirCarryFlying || w.Air.Carry[hero].Phase != AirCarryNone || w.Air.Carry[protected].Phase != AirCarryNone {
		t.Fatal("whirlwind lift ignored immunity or failed town evacuation")
	}
	for _, d := range townFootprint {
		if w.Farms[32+d[0]+(32+d[1])*MapSize] != 0 {
			t.Fatal("lifted town retained farmland")
		}
	}
	for i := range w.Heights {
		w.Heights[i] = 0
	}
	w.rebuildCells()
	w.Players[0].Experience[Water] = 64
	worldAirHabitat{w}.CreateWhirlpool(0, 10, 10)
	if !w.Water.Whirlpools[0].Active || w.Water.Whirlpools[0].Life != 364 || w.Cell(10, 10).Code != 152 {
		t.Fatal("whirlwind failed its exact four-water-cell child creation")
	}
	worldAirHabitat{w}.CreateWhirlpool(0, 10, 10)
	if w.Water.Whirlpools[1].Active {
		t.Fatal("animated water admitted a second overlapping whirlpool")
	}
}

func TestWhirlwindCarryingAndLandingMatchOriginalNumericFixtures(t *testing.T) {
	var catalog struct {
		Fixtures []struct {
			Input struct {
				Name, Mode                     string
				Hero, State, Kind, Stage, X, Y int
				Seed                           uint32
				Animation, Ticks               int
			}
			RNG    uint32
			Groups []struct {
				Slot                       int
				Kind, Owner, State, Weapon uint8
				X, Y, Animation            uint16
				Population                 int32
			}
		}
	}
	data, err := os.ReadFile("../populous2/testdata/whirlwind_interactions_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range catalog.Fixtures {
		f := fixture.Input
		if f.Mode != "lifted" && f.Mode != "landing" {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			w := testFlatWorld()
			w.random = randomState(f.Seed)
			id := addFollower(w, f.X, f.Y, 1, 101, Airborne)
			kind := HeroNone
			if f.Hero >= 0 {
				kind = HeroKind(f.Hero + 1)
				w.Followers[id].Hero.Kind = kind
			}
			frames := 12
			base := 1236
			if f.Animation >= 10380 || f.Animation >= 3692 && f.Animation < 3708 {
				for hero, index := range []int{10400, 10920, 3692, 0, 10940, 10380} {
					if index > 0 && f.Animation >= index && f.Animation < index+16 {
						kind = HeroKind(hero + 1)
						w.Followers[id].Hero.Kind = kind
						break
					}
				}
			}
			if kind != HeroNone {
				frames = 4
				base = [6]int{10400, 10920, 3692, 0, 10940, 10380}[kind-1]
			}
			w.Followers[id].Weapons = 4
			w.Air.Carry[id] = AirCarryState{Phase: AirCarryFlying, Effect: 1, Frame: (f.Animation - base) / 4, Frames: frames}
			w.Air.Whirlwinds[0] = WhirlwindEffect{X: (f.X+2)*256 + 211, Y: (f.Y-1)*256 + 17}
			passes := 1
			if f.Mode == "landing" {
				w.Followers[id].Weapons = 1
				w.moveFollowerCell(id, f.X+1, f.Y+1)
				w.Followers[id].positionX, w.Followers[id].positionY = (f.X+1)*256+128, (f.Y+1)*256+128
				w.Air.Carry[id] = AirCarryState{Phase: AirCarryLanding, Effect: 1, Frames: 7}
				w.random.next()
				passes = f.Ticks
				base = 1676
			}
			for i := 0; i < passes; i++ {
				if !w.AdvanceAirCarry(id) {
					break
				}
				if w.Air.Carry[id].Phase == AirCarryNone {
					break
				}
			}
			got := w.Followers[id]
			carry := w.Air.Carry[id]
			animation := base + carry.Frame*4
			state := uint8(20)
			if f.Mode == "landing" {
				state = 26
				if carry.Phase == AirCarryNone {
					state = 2
					animation = 0
				}
			}
			want := fixture.Groups[0]
			if uint16(got.positionX) != want.X || uint16(got.positionY) != want.Y || uint16(animation) != want.Animation || state != want.State || got.Weapons != int(want.Weapon) || int32(got.Population) != want.Population || uint32(w.random) != fixture.RNG {
				t.Fatalf("typed transport differs:xy%d/%d animation%d state%d population%d weapon%d RNG%x; original%+v RNG%x", got.positionX, got.positionY, animation, state, got.Population, got.Weapons, uint32(w.random), want, fixture.RNG)
			}
			checked++
		})
	}
	if checked != 10 {
		t.Fatalf("checked %d transport/landing fixtures, want10", checked)
	}
}

func TestStormWorldUsesDirectFireVictimsWithoutReplacingItsRandomStrike(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 1, 1000, Walking)
	w.Air.Storms[0] = StormEffect{Active: true, Owner: 0, X: 32*256 + 128, Y: 32*256 + 128, Life: 200, Timer: 1}
	w.effects.Slots[0] = EffectReservation{Kind: EffectStorm, Owner: 0}
	rng := w.random
	w.tickAirEffect(0)
	if w.Followers[id].Population != 0 || w.FireDamage.Deaths[id].Mode != FireVictimDying || w.random != rng {
		t.Fatal("storm cooldown damage lost its separate direct-death lifecycle")
	}
}
