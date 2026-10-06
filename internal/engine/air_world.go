package engine

import "fmt"

type worldAirHabitat struct{ world *World }

func (h worldAirHabitat) Reserve(kind EffectKind, owner uint8) int {
	id := h.world.allocateEffect(kind, owner)
	if id >= 0 && kind == EffectWhirlwind {
		previous := h.world.effects.Slots[id]
		h.world.Air.Whirlwinds[id].VX, h.world.Air.Whirlwinds[id].VY = previous.LastVelocityX, previous.LastVelocityY
	}
	return id
}
func (h worldAirHabitat) Release(id int) {
	if id >= 0 && id < EffectCapacity && h.world.effects.Slots[id].Kind == EffectWhirlwind {
		h.world.effects.Slots[id].LastVelocityX, h.world.effects.Slots[id].LastVelocityY = h.world.Air.Whirlwinds[id].VX, h.world.Air.Whirlwinds[id].VY
	}
	h.world.releaseEffect(id)
}
func (h worldAirHabitat) Random() uint16 { return h.world.random.next() }
func (h worldAirHabitat) AirExperience(owner uint8) uint8 {
	if owner > 1 {
		return 0
	}
	return h.world.Players[owner].Experience[Air]
}
func (h worldAirHabitat) Parcel(x, y int) FireParcel { return worldFireHabitat{h.world}.Parcel(x, y) }
func (h worldAirHabitat) Scorch(x, y int)            { h.world.scorchFireParcel(x, y) }
func (h worldAirHabitat) DamageStorm(x, y int) int   { return h.world.damageFireParcel(x, y, false) }
func (h worldAirHabitat) StrikeLightning(bolt, x, y int) bool {
	if !inside(x, y) {
		return true
	}
	if h.world.WallBlocksLightning(x, y) {
		return false
	}
	var followers [FollowerCapacity]int
	count := h.world.FollowersAt(x, y, followers[:])
	for _, id := range followers[:count] {
		f := &h.world.Followers[id]
		if f.State == Inactive || f.State == Ruin || int(f.X) != x || int(f.Y) != y {
			continue
		}
		victim := &h.world.AirVictims[id]
		victim.Bind(bolt, f.State == Town, f.Consecrated || f.Conversion.Active, f.Hero.Kind)
		if victim.Phase == LightningVictimWalkingHit || victim.Phase == LightningVictimTownHit {
			f.Frame = uint16(victim.Frame)
			f.moving = false
		}
	}
	return true
}

func (h worldAirHabitat) CreateWhirlpool(owner uint8, x, y int) {
	w := h.world
	for _, d := range whirlpoolFootprint {
		if !inside(x+d[0], y+d[1]) || w.Cell(x+d[0], y+d[1]).Code != 0 {
			return
		}
	}
	id := w.allocateEffect(EffectWhirlpool, owner)
	if id < 0 {
		return
	}
	life := 300
	if owner < 2 {
		life += int(w.Players[owner].Experience[Water])
	}
	w.Water.Whirlpools[id] = WhirlpoolEffect{Active: true, Owner: owner, X: x, Y: y, Life: life, Delay: 16}
	w.paintWhirlpool(&w.Water.Whirlpools[id])
}
func (h worldAirHabitat) LiftFollowers(effect, x, y int) { h.world.liftAirFollowers(effect, x, y) }
func (h worldAirHabitat) ReleaseFollowers(effect, x, y int) {
	h.world.releaseAirFollowers(effect, x, y)
}

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
	w.RecordPowerUse(owner, Lightning)
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
	defer w.syncEffectActor(id)
	if w.effects.Slots[id].Kind == EffectLightning {
		w.Air.TickLightning(id, worldAirHabitat{w})
	} else if w.effects.Slots[id].Kind == EffectWhirlwind {
		w.Air.TickWhirlwind(id, worldAirHabitat{w})
	} else if w.effects.Slots[id].Kind == EffectStorm {
		w.Air.TickStorm(id, worldAirHabitat{w})
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
			f.Stage = 0
			f.LastDevelopedStage = 0
			f.FoundedAt = w.Tick
			f.positionX = int(f.X)*256 + 128
			f.positionY = int(f.Y)*256 + 128
			f.positionSet = true
			w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, f.positionX, f.positionY)
			if stage := w.EvaluateTown(id); stage > 0 {
				f.State = Town
				f.Stage = uint8(stage)
			} else {
				f.State = Walking
				f.Stage = 0
			}
		}
	case LightningVictimRetainDeath:
		w.PrepareFollowerDeath(id)
		f.State = Ruin
		f.moving = false
	case LightningVictimResume:
		*v = LightningVictimState{}
		f.State, f.Frame = Walking, 0
	}
	return true
}

func (w *World) CastWhirlwind(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid whirlwind target")
	}
	if !w.Air.CreateWhirlwind(uint8(owner), x, y, worldAirHabitat{w}) {
		return fmt.Errorf("whirlwind exhausted effect reservations")
	}
	return nil
}

func (w *World) liftAirFollowers(effect, x, y int) {
	var followers [FollowerCapacity]int
	count := w.FollowersAt(x, y, followers[:])
	for _, id := range followers[:count] {
		f := &w.Followers[id]
		if f.State == Inactive || f.State == Ruin || f.Consecrated || f.Conversion.Active || f.Hero.Kind == HeroOdysseus || w.Air.Carry[id].Phase == AirCarryFlying {
			continue
		}
		if f.State == Town {
			w.clearLiftedTownFarms(id)
		}
		frames := 12
		if f.IsHero() {
			frames = 4
		}
		f.State, f.Frame, f.Weapons, f.moving = Airborne, 0, 1, false
		w.Air.Carry[id] = AirCarryState{Phase: AirCarryFlying, Effect: effect + 1, Frames: frames}
	}
}

func (w *World) clearLiftedTownFarms(id int) {
	f := w.Followers[id]
	limit := 9
	if f.Stage >= 10 {
		limit = 25
	}
	if f.Stage >= 18 {
		limit = 49
	}
	for _, d := range townFootprint[:limit] {
		x, y := int(f.X)+d[0], int(f.Y)+d[1]
		if inside(x, y) && w.Farms[x+y*MapSize] == f.Owner+1 {
			w.Farms[x+y*MapSize] = 0
		}
	}
}

// releaseAirFollowers uses semantic adjacent destinations. Undefined record-
// layout aliasing in the original release routine does not become a game rule.
func (w *World) releaseAirFollowers(effect, x, y int) {
	var followers [FollowerCapacity]int
	count := w.FollowersAt(x, y, followers[:])
	for _, id := range followers[:count] {
		carry := &w.Air.Carry[id]
		if carry.Phase != AirCarryFlying {
			continue
		}
		d := fireNeighbors[int(w.random.next()&14)/2]
		nx, ny, valid := offsetFireParcel(x, y, d[0], d[1])
		if !valid {
			w.remove(id)
			continue
		}
		w.moveFollowerCell(id, nx, ny)
		f := &w.Followers[id]
		f.positionX, f.positionY, f.positionSet = nx*256+128, ny*256+128, true
		w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, f.positionX, f.positionY)
		f.Frame = 0
		carry.Phase, carry.Frame, carry.Frames = AirCarryLanding, 0, 7
	}
}

// AdvanceAirCarry follows the carrier's fractional position without advancing
// its lifetime. Landing retains identity for seven ordinary passes before the
// follower may make its next movement decision.
func (w *World) AdvanceAirCarry(id int) bool {
	if id <= 0 || id >= FollowerCapacity {
		return false
	}
	carry := &w.Air.Carry[id]
	if carry.Phase == AirCarryNone {
		return false
	}
	f := &w.Followers[id]
	if f.State == Inactive {
		*carry = AirCarryState{}
		return false
	}
	if carry.Phase == AirCarryLanding {
		if carry.Frame+1 >= carry.Frames {
			carry.Phase = AirCarryNone
			f.State, f.Frame = Walking, 0
		} else {
			carry.Frame++
			f.Frame = uint16(carry.Frame)
		}
		return true
	}
	carry.Frame = (carry.Frame + 1) % carry.Frames
	f.Frame = uint16(carry.Frame)
	source := w.Air.Whirlwinds[carry.Effect-1]
	x, y := source.X, source.Y
	if !inside(x>>8, y>>8) {
		w.remove(id)
		return true
	}
	if int(f.X) != x>>8 || int(f.Y) != y>>8 {
		w.moveFollowerCell(id, x>>8, y>>8)
	}
	f.positionX, f.positionY, f.positionSet = x, y, true
	w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, x, y)
	return true
}

func (w *World) CastStorm(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid storm target")
	}
	if !w.Air.CreateStorm(uint8(owner), x, y, worldAirHabitat{w}) {
		return fmt.Errorf("storm exhausted effect reservations")
	}
	return nil
}

func (h worldAirHabitat) SyncEffect(id int) { h.world.syncEffectActor(id) }
