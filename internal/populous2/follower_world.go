package populous2

import legacy "go-populous2/internal/legacy"

// NativeFollower is a generation-tagged motion record. Towns, battles, waiting,
// water and heroes retain their separate inherited state handlers for now.
type NativeFollower struct {
	Active     bool
	Generation uint64
	Actor      FollowerMotionActor
}

func (w *World) ordinaryWalker(index int) bool {
	if index < 0 || index >= len(w.Core.Peeps) || index >= legacy.MaxFollowers {
		return false
	}
	p := w.Core.Peeps[index]
	return p.Population > 0 && p.Flags == legacy.OnMove && p.Status != legacy.KnightStatus && !w.Heroes[index].Active && !w.isCaptive(index) && !w.LightningVictims[index].Active && !w.NativeEntries[index].Managed
}

func (w *World) initializeNativeFollower(index int) {
	if index < 0 || index >= len(w.Core.Peeps) || index >= legacy.MaxFollowers {
		return
	}
	p := &w.Core.Peeps[index]
	if p.MovementSpeed == 0 && p.Player < 2 {
		// Older Go saves and inherited births did not retain this byte.
		p.MovementSpeed = w.Level.Players[p.Player].MovementSpeed()
	}
	generation := w.NativeFollowers[index].Generation + 1
	if generation == 0 {
		generation = 1
	}
	w.NativeFollowers[index] = NativeFollower{Active: w.ordinaryWalker(index), Generation: generation, Actor: FollowerMotionActor{Kind: 2, Player: p.Player, X: int16((p.AtPos%64)*256 + 128), Y: int16((p.AtPos/64)*256 + 128), Speed: p.MovementSpeed, State: 2, ReturnState: 2, Population: int32(p.Population), Variant: uint16(index+1) & 14}}
	ref := nativeActorReference(NativeFollowerPool, index)
	if linked, _ := w.Occupancy.Linked(ref); linked {
		record, _ := w.Occupancy.Record(ref)
		if int(record.X>>8)+int(record.Y>>8)*64 == p.AtPos {
			w.NativeFollowers[index].Actor.X, w.NativeFollowers[index].Actor.Y = int16(record.X), int16(record.Y)
			w.NativeFollowers[index].Actor.Next, w.NativeFollowers[index].Actor.Previous = uint16(record.Next), uint16(record.Previous)
		} else {
			w.moveActor(NativeFollowerPool, index, uint16((p.AtPos%64)*256+128), uint16((p.AtPos/64)*256+128))
		}
	} else {
		w.placeActor(NativeFollowerPool, index, uint16((p.AtPos%64)*256+128), uint16((p.AtPos/64)*256+128))
	}
}

func (w *World) bindFollowerMotion() {
	w.Core.OnFollowerAllocated = func(index int) {
		// Same faith and position can belong to a new record generation.
		w.Heroes[index] = Hero{}
		w.captiveIndex[index] = false
		w.LightningVictims[index] = NativeLightningFollower{}
		w.NativeEntries[index] = NativeFollowerEntry{}
		w.unlinkActor(NativeFollowerPool, index)
		w.initializeNativeFollower(index)
	}
	w.Core.NativeFollowerUpdate = w.updateNativeFollower
}

func (w *World) updateNativeFollower(index int) bool {
	if !w.ordinaryWalker(index) {
		if index >= 0 && index < len(w.NativeFollowers) {
			w.NativeFollowers[index].Active = false
		}
		return false
	}
	p := &w.Core.Peeps[index]
	record := &w.NativeFollowers[index]
	if !record.Active || record.Generation == 0 || int(record.Actor.X)>>8+(int(record.Actor.Y)>>8)*64 != p.AtPos {
		w.initializeNativeFollower(index)
		record = &w.NativeFollowers[index]
	}
	actor := &record.Actor
	actor.Player, actor.Population, actor.Speed = p.Player, int32(p.Population), p.MovementSpeed
	w.FollowerMotion.Tick(actor, FollowerMotionCallbacks{
		Prepass: func(actor *FollowerMotionActor) bool {
			if w.Core.BeforeFollower != nil && !w.Core.BeforeFollower(index) {
				record.Active = false
				actor.Population = int32(w.Core.Peeps[index].Population)
				return false
			}
			actor.Player, actor.Population = w.Core.Peeps[index].Player, int32(w.Core.Peeps[index].Population)
			return w.ordinaryWalker(index)
		},
		Decide: func(actor *FollowerMotionActor) bool { return w.planNativeWalker(index, actor) },
		Admit: func(actor *FollowerMotionActor, x, y int16) bool {
			position := (int(x) >> 8) + (int(y)>>8)*64
			if w.GroundRules.Properties[w.nativeTileAt(position%64, position/64)]&8 != 0 {
				return false
			}
			if scenery := w.sceneryAt(position); scenery >= 0 && w.Scenery[scenery].Kind == SceneryBoulder {
				return false
			}
			return w.Core.MovementAllowed == nil || w.Core.MovementAllowed(index, position, true)
		},
		Move: func(actor *FollowerMotionActor, oldX, oldY int16) {
			position := (int(actor.X) >> 8) + (int(actor.Y)>>8)*64
			w.moveActor(NativeFollowerPool, index, uint16(actor.X), uint16(actor.Y))
			w.Core.MapWho[position] = w.legacyFollowerHead(position, index)
			w.Core.CommitWalkerEntry(index, position)
			w.Core.MapSteps[position] = (w.Core.MapSteps[position] + 1) & 31
			actor.Population = int32(w.Core.Peeps[index].Population)
		},
	})
	p = &w.Core.Peeps[index]
	record.Active = w.ordinaryWalker(index)
	if record.Active {
		p.Frame = actor.Animation / 4
		w.moveActor(NativeFollowerPool, index, uint16(actor.X), uint16(actor.Y))
	}
	return true
}

func (w *World) planNativeWalker(index int, actor *FollowerMotionActor) bool {
	if actor.State != 2 && actor.State != 18 || !w.ordinaryWalker(index) {
		return false
	}
	p := &w.Core.Peeps[index]
	magnet := w.Core.Magnets[p.Player].Flags == legacy.MagnetMode || w.Core.War
	attrition := w.Level.Players[p.Player].FollowerAttrition()
	if w.Core.FollowerAttrition != nil {
		attrition = w.Core.FollowerAttrition(int(p.Player), false)
	}
	if actor.State == 2 && magnet {
		// The initial mode switch subtracts at both $1132e and $11bd0.
		attrition *= 2
	}
	w.Core.DamagePeep(index, attrition)
	actor.Population = int32(w.Core.Peeps[index].Population)
	if w.Core.Peeps[index].Population <= 0 {
		return false
	}
	if !magnet {
		decision, err := w.FollowerDecision.Select(actor, uint8(p.IQ), w.nativeFollowerMode(int(p.Player)), &w.Occupancy.Grid, FollowerDecisionCallbacks{Record: w.followerDecisionRecord, Random: w.random})
		if err != nil {
			panic(err)
		}
		p.MovementSpeed = actor.Speed
		return decision.Fallthrough
	}
	delta, moving := w.Core.PlanWalkerStep(index)
	if !moving {
		return false
	}
	p = &w.Core.Peeps[index]
	position := p.AtPos + delta
	if magnet {
		// $14646 recenters; $140f0 ends before the first fractional step.
		actor.X, actor.Y = int16((p.AtPos%64)*256+128), int16((p.AtPos/64)*256+128)
	}
	if err := w.FollowerMotion.BeginLeg(actor, int16((position%64)*256+128), int16((position/64)*256+128)); err != nil {
		p.Flags |= legacy.IAmWaiting
		return false
	}
	actor.State, actor.ReturnState = 4, 2
	if magnet {
		actor.ReturnState = 18
	}
	return !magnet
}

func (w *World) nativeFollowerMode(player int) NativeFollowerMode {
	switch w.Core.Magnets[player].Flags {
	case legacy.MagnetMode:
		return NativeFollowerMagnet
	case legacy.JoinMode:
		return NativeFollowerJoin
	case legacy.FightMode:
		return NativeFollowerFight
	default:
		return NativeFollowerSettle
	}
}

func (w *World) followerDecisionRecord(ref NativeRecordReference) (FollowerDecisionRecord, bool) {
	v, ok := w.lightningRecord(ref)
	location, located := LocateNativeRecord(ref)
	if located {
		switch location.Pool {
		case NativeFollowerPool:
			index := location.Index - 1
			if index >= len(w.Core.Peeps) || w.Core.Peeps[index].Population <= 0 && !w.flameDeathIndex[index] && !w.LightningVictims[index].Active {
				v.Owner = 0
			}
		case NativeEffectPool:
			if !w.NativeEffects[location.Index].Active {
				v.Owner = 0
			}
		case NativeWallPool:
			if !w.Walls.Actors[location.Index].Active {
				v.Owner = 0
			}
		}
	}
	return FollowerDecisionRecord{Kind: v.Kind, Owner: v.Owner, Next: v.Next}, ok
}

func (w *World) nativeFollowerSnapshot() [legacy.MaxPeeps]NativeFollower {
	records := w.NativeFollowers
	for index := range records {
		if !w.ordinaryWalker(index) || int(records[index].Actor.X)>>8+(int(records[index].Actor.Y)>>8)*64 != w.Core.Peeps[index].AtPos {
			records[index].Active = false
		} else {
			p := w.Core.Peeps[index]
			records[index].Actor.Player = p.Player
			records[index].Actor.Population = int32(p.Population)
			records[index].Actor.Speed = p.MovementSpeed
		}
	}
	return records
}

// FollowerPosition supplies fractions to ordinary walking art. Other state
// handlers retain their current rendering until their native controllers land.
func (w *World) FollowerPosition(index int) (int16, int16, bool) {
	if !w.ordinaryWalker(index) || !w.NativeFollowers[index].Active {
		return 0, 0, false
	}
	a := w.NativeFollowers[index].Actor
	if int(a.X)>>8+(int(a.Y)>>8)*64 != w.Core.Peeps[index].AtPos {
		return 0, 0, false
	}
	return a.X, a.Y, true
}

func (w *World) FollowerFrame(index int) (AnimationFrame, bool) {
	if _, _, ok := w.FollowerPosition(index); !ok {
		return AnimationFrame{}, false
	}
	frame, _, ok := w.FollowerMotion.Frame(w.NativeFollowers[index].Actor)
	return frame, ok
}

func validNativeFollowerLink(reference uint16) bool {
	if _, ok := LocateNativeMagnet(NativeRecordReference(reference)); ok {
		return true
	}
	r := int(int16(reference))
	switch {
	case r < -6000:
		return false
	case r < -2800:
		return (r+6000)%16 == 0
	case r < 0:
		return (r+2800)%14 == 0
	case r < 20800:
		return r%52 == 0
	case r < 28800:
		return (r-20800)%32 == 0
	}
	return false
}
