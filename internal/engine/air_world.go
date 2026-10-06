package engine

import "fmt"

type worldAirHabitat struct{ world *World }

func (h worldAirHabitat) Reserve(kind EffectKind, owner uint8) int {
	return h.world.allocateEffect(kind, owner)
}
func (h worldAirHabitat) Release(id int) { h.world.releaseEffect(id) }
func (h worldAirHabitat) Random() uint16 { return h.world.random.next() }
func (h worldAirHabitat) AirExperience(owner uint8) uint8 {
	if owner > 1 {
		return 0
	}
	return h.world.Players[owner].Experience[Air]
}
func (h worldAirHabitat) Parcel(x, y int) FireParcel { return worldFireHabitat{h.world}.Parcel(x, y) }
func (h worldAirHabitat) Scorch(x, y int)            { h.world.scorchFireParcel(x, y) }
func (h worldAirHabitat) StrikeLightning(bolt, x, y int) bool {
	if !inside(x, y) {
		return true
	}
	var followers [FollowerCapacity]int
	count := h.world.FollowersAt(x, y, followers[:])
	for _, id := range followers[:count] {
		f := &h.world.Followers[id]
		if f.State == Inactive || f.State == Ruin || int(f.X) != x || int(f.Y) != y {
			continue
		}
		victim := &h.world.AirVictims[id]
		victim.Bind(bolt, f.State == Town, f.Consecrated, f.Hero.Kind)
		if victim.Phase == LightningVictimWalkingHit || victim.Phase == LightningVictimTownHit {
			f.Frame = uint16(victim.Frame)
			f.moving = false
		}
	}
	return true
}

// Whirlwind interactions are separate from the lightning slice. Its controller
// remains unavailable to public Cast until carrying and water birth are bound.
func (h worldAirHabitat) CreateWhirlpool(uint8, int, int) {
	panic("whirlwind water birth is not bound")
}
func (h worldAirHabitat) LiftFollowers(int, int, int)    { panic("whirlwind carrying is not bound") }
func (h worldAirHabitat) ReleaseFollowers(int, int, int) { panic("whirlwind release is not bound") }

// PlaceLightning creates or moves the targeting marker without charging mana.
func (w *World) PlaceLightning(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid lightning target")
	}
	if !w.Level.Players[owner].Powers[Lightning] {
		return fmt.Errorf("lightning is disabled in this world")
	}
	if !w.Air.PlaceLightning(uint8(owner), x, y, worldAirHabitat{w}) {
		return fmt.Errorf("lightning marker exhausted the shared effect pool")
	}
	return nil
}

// ActivateLightning is the separate strike command. An admitted command spends
// its normal power cost even when no marker or free bolt slot is available.
func (w *World) ActivateLightning(owner int) error {
	if owner < 0 || owner > 1 {
		return fmt.Errorf("invalid lightning owner")
	}
	if !w.Level.Players[owner].Powers[Lightning] {
		return fmt.Errorf("lightning is disabled in this world")
	}
	cost := w.PowerCost(owner, Lightning)
	if w.Players[owner].Mana < cost {
		return fmt.Errorf("not enough mana")
	}
	w.Air.ActivateLightning(uint8(owner), worldAirHabitat{w})
	w.Players[owner].Mana -= cost
	return nil
}
func (w *World) DismissLightning(owner int) bool {
	if owner < 0 || owner > 1 || w.Air.MarkerSlots[owner] == 0 {
		return false
	}
	w.Air.DismissLightning(uint8(owner), worldAirHabitat{w})
	return true
}
func (w *World) tickAirEffect(id int) {
	if w.effects.Slots[id].Kind == EffectLightning {
		w.Air.TickLightning(id, worldAirHabitat{w})
	}
}

// AdvanceLightningVictim owns the recovery or removal of an affected follower.
// It runs before ordinary movement, town work and zero-population cleanup.
func (w *World) AdvanceLightningVictim(id int) bool {
	if id <= 0 || id >= FollowerCapacity {
		return false
	}
	v := &w.AirVictims[id]
	if v.Phase == LightningVictimNone {
		return false
	}
	f := &w.Followers[id]
	if f.State == Inactive {
		*v = LightningVictimState{}
		return false
	}
	alive := v.Bolt > 0 && w.Air.Bolts[v.Bolt-1].Active
	transition := TickLightningVictim(v, &f.Population, alive)
	f.Frame = uint16(v.Frame)
	switch transition {
	case LightningVictimRemove:
		if f.State == Town {
			w.clearBurntTownFarms(id)
		}
		w.remove(id)
		*v = LightningVictimState{}
	case LightningVictimReformTown:
		*v = LightningVictimState{}
		if f.IsHero() {
			f.State = Walking
		} else {
			if stage := w.TownStage(int(f.Owner), int(f.X), int(f.Y), id); stage > 0 {
				f.State = Town
				f.Stage = uint8(stage)
			} else {
				f.State = Walking
				f.Stage = 0
			}
		}
	case LightningVictimRetainDeath:
		f.State = Ruin
		f.moving = false
	case LightningVictimResume:
		*v = LightningVictimState{}
		f.State, f.Frame = Walking, 0
	}
	return true
}
