package populous2

import (
	"fmt"
	legacy "go-populous2/internal/legacy"
)

type NativeLightningFollower struct {
	Active bool
	Victim LightningVictim
}

// PlaceLightning is native command28: it creates/moves the owner marker
// without a mana debit. The game's power availability still applies.
func (w *World) PlaceLightning(player, x, y int) bool {
	if !w.Available(player, Lightning) || w.Core.War || !inside(x, y) {
		return false
	}
	ok, err := w.LightningRules.Place(&w.LightningState, &w.NativeEffects, player, x, y, w.lightningCallbacks())
	if err != nil {
		panic(err)
	}
	return ok
}

// ActivateLightning is native command30. Its handler debits the native slot18
// price regardless of how many bolts the shared allocation helper can create.
func (w *World) ActivateLightning(player int) bool {
	if !w.Available(player, Lightning) || w.Core.War {
		return false
	}
	cost := w.ManaCost(player, Lightning)
	if w.Core.Magnets[player].Mana < cost {
		return false
	}
	if _, err := w.LightningRules.Activate(&w.LightningState, &w.NativeEffects, player, w.Experience[player][Air], w.lightningCallbacks()); err != nil {
		panic(err)
	}
	w.Core.Magnets[player].Mana -= cost
	w.recordCast(player, Lightning)
	return true
}

func (w *World) DismissLightning(player int) bool {
	if player < 0 || player > 1 || w.LightningState.Markers[player] == 0 {
		return false
	}
	if err := w.LightningRules.Dismiss(&w.LightningState, &w.NativeEffects, player, w.lightningCallbacks()); err != nil {
		panic(err)
	}
	return true
}

func (w *World) lightningCallbacks() LightningCallbacks {
	return LightningCallbacks{
		Insert: func(ref NativeRecordReference) error {
			location, _ := LocateNativeRecord(ref)
			w.linkEffect(location.Index)
			return nil
		},
		Move: func(ref NativeRecordReference, x, y uint16) error {
			location, _ := LocateNativeRecord(ref)
			w.moveActor(NativeEffectPool, location.Index, x, y)
			return nil
		},
		Unlink: func(ref NativeRecordReference) error {
			location, _ := LocateNativeRecord(ref)
			w.unlinkActor(NativeEffectPool, location.Index)
			return nil
		},
		Head:   func(x, y int) NativeRecordReference { return w.Occupancy.Grid.Cells[x+y*64].Head },
		Record: w.lightningRecord, SetRecord: w.setLightningRecord,
		Burn: w.scorchLightningTile, Random: w.random,
	}
}

func (w *World) lightningRecord(ref NativeRecordReference) (LightningVictim, bool) {
	location, ok := LocateNativeRecord(ref)
	if !ok {
		return LightningVictim{}, false
	}
	graph, _ := w.Occupancy.Record(ref)
	switch location.Pool {
	case NativeWallPool:
		return LightningVictim{Kind: 0x1a, Owner: w.Walls.Actors[location.Index].Player + 1, Next: graph.Next}, true
	case NativeSceneryPool:
		return LightningVictim{Kind: uint8(w.Scenery[location.Index].Kind), Next: graph.Next}, true
	case NativeEffectPool:
		a := w.NativeEffects[location.Index]
		return LightningVictim{Kind: a.Kind, Owner: a.Player + 1, State: a.State, Animation: a.Animation, Next: graph.Next}, true
	case NativeFollowerPool:
		index := location.Index - 1
		if index >= len(w.Core.Peeps) {
			return LightningVictim{}, false
		}
		p := w.Core.Peeps[index]
		v := LightningVictim{Kind: 2, Owner: p.Player + 1, State: 2, Population: int32(p.Population), Next: graph.Next}
		if p.Flags&legacy.InTown != 0 {
			v.Kind, v.State = 4, 6
		}
		if w.Heroes[index].Active {
			v.Flags = 2
			v.HeroType = uint16(heroIndex(w.Heroes[index].Spell) * 2)
		}
		if w.Core.Magnets[p.Player].Carried == index+1 && w.Core.Magnets[p.Player].Flags == legacy.MagnetMode {
			v.State = 0x3a
		}
		if owned := w.LightningVictims[index]; owned.Active {
			v = owned.Victim
			v.Next = graph.Next
		}
		return v, true
	}
	return LightningVictim{}, false
}

func (w *World) setLightningRecord(ref NativeRecordReference, victim LightningVictim) {
	location, ok := LocateNativeRecord(ref)
	if !ok || location.Pool != NativeFollowerPool {
		panic("native lightning wrote a non-follower record")
	}
	index := location.Index - 1
	w.LightningVictims[index] = NativeLightningFollower{Active: victim.State == 0x1c || victim.State == 0x1e || victim.State == 0x20 || victim.State == 0x22, Victim: victim}
	if w.LightningVictims[index].Active {
		w.NativeFollowers[index].Active = false
	}
}

func (w *World) scorchLightningTile(x, y int) {
	cell := w.TerrainCell(x, y)
	if cell.Shape == 15 && w.GroundRules.Properties[cell.Code]&0x400 == 0 {
		w.Marks[x+y*64] = Mark{Spell: Lightning, Life: 1, Persistent: true, NativeTile: 95}
		if err := w.Occupancy.SetTile(x, y, 95); err != nil {
			panic(err)
		}
	}
}

func (w *World) bindLightningVictims() {
	w.Core.NativeEffectFollowerUpdate = w.updateLightningVictim
}

func (w *World) updateLightningVictim(index int) bool {
	if index < 0 || index >= len(w.Core.Peeps) || !w.LightningVictims[index].Active {
		return false
	}
	managed := &w.LightningVictims[index]
	victim := &managed.Victim
	p := &w.Core.Peeps[index]
	if w.GroundRules.Properties[w.nativeTileAt(p.AtPos%64, p.AtPos/64)]&8 != 0 && !(w.Heroes[index].Active && w.Heroes[index].Spell == Helen) {
		// The original common prepass changes water entrants to kind10 /
		// drowning state16 before lightning dispatch. Release ownership to
		// the water handler; it must not take a lightning damage step here.
		managed.Active = false
		if p.Flags&legacy.InTown != 0 {
			w.Core.DetachFollower(index)
		}
		p.Flags = legacy.OnMove | legacy.InWater
		w.Core.Magnets[p.Player].Population += max(0, p.Population)
		return true
	}
	// The native common terrain prepass still precedes the managed dispatch.
	if w.Core.BeforeFollower != nil && !w.Core.BeforeFollower(index) {
		managed.Active = false
		return true
	}
	victim.Population = int32(p.Population)
	step, err := w.LightningRules.TickVictim(victim, LightningVictimCallbacks{
		BoltAlive: func(ref NativeRecordReference) bool {
			location, ok := LocateNativeRecord(ref)
			// Native liveness tests only the signed positive owner byte. A
			// reused effect slot can keep a stale victim reference alive.
			return ok && location.Pool == NativeEffectPool && w.NativeEffects[location.Index].Active && w.NativeEffects[location.Index].Player < 127
		},
		Cleanup: func(retain bool) {
			p.Population = int(victim.Population)
			if retain {
				w.Core.DetachFollower(index)
				w.Core.DamagePeep(index, max(1, p.Population))
				victim.Population = 0
				w.Core.ReserveDeathOccupancy(index)
			} else {
				managed.Active = false
				w.Core.DetachFollower(index)
				w.Core.DamagePeep(index, max(1, p.Population))
				victim.Population = 0
				w.unlinkActor(NativeFollowerPool, index)
			}
		},
		Remove: func() {
			managed.Active = false
			w.unlinkActor(NativeFollowerPool, index)
			w.Core.ReleaseDeathOccupancy(index)
			victim.Owner, victim.Population = 0, 0
		},
		ReformTown: func() {
			managed.Active = false
			p.Population = int(victim.Population)
			w.moveActor(NativeFollowerPool, index, uint16((p.AtPos%64)*256+128), uint16((p.AtPos/64)*256+128))
			p.BattlePopulation = w.Core.GameTurn
			if w.Heroes[index].Active {
				w.Core.DetachFollower(index)
			} else {
				w.Core.ReformOlympianTown(index)
			}
		},
	})
	if err != nil {
		panic(err)
	}
	p.Population = int(victim.Population)
	if step.NeedsDecision {
		managed.Active = false
		p.Flags, p.Frame = legacy.OnMove, 0
		w.initializeNativeFollower(index)
		// Native recovery redispatches its ordinary decision in the same
		// update. Only ordinary routing is currently translated here.
		w.updateNativeFollower(index)
	}
	return true
}

func (w *World) LightningFollowerFrame(index int) (AnimationFrame, bool) {
	if index < 0 || index >= len(w.LightningVictims) || !w.LightningVictims[index].Active {
		return AnimationFrame{}, false
	}
	frame, ok := w.LightningRules.Frames[w.LightningVictims[index].Victim.Animation]
	return frame, ok
}

// LightningBeams returns original procedural beam segments in native screen
// coordinates. Endpoint projection remains the caller's terrain/view mapping.
func (w *World) LightningBeams(project func(int16, int16) (LightningPoint, bool)) [][4]LightningSegment {
	var beams [][4]LightningSegment
	for index, actor := range w.NativeEffects {
		if !actor.Active || actor.Kind != 0x2a {
			continue
		}
		marker, ok := lightningSlot(w.LightningState.Word28[index])
		if !ok || !w.NativeEffects[marker].Active {
			continue
		}
		from, visibleFrom := project(actor.X, actor.Y)
		m := w.NativeEffects[marker]
		_, visibleTo := project(m.X, m.Y)
		if !visibleFrom && !visibleTo {
			continue
		}
		header := w.Occupancy.Grid.Cells[(int(actor.X)>>8)+(int(actor.Y)>>8)*64].Header
		to := LightningMarkerEndpoint(int(m.X)>>8, int(m.Y)>>8, w.effectViewX, w.effectViewY, header)
		beams = append(beams, LightningBeamSegments(from, to, w.LightningState.RandomWords[index], uint16(w.Core.GameTurn)))
	}
	return beams
}

func validateLightningSave(bundle *Bundle, snapshot Snapshot) error {
	for player, ref := range snapshot.LightningState.Markers {
		if ref == 0 {
			continue
		}
		index, ok := lightningSlot(ref)
		if !ok || !snapshot.NativeEffects[index].Active || snapshot.NativeEffects[index].Kind != 0x28 || snapshot.NativeEffects[index].Player != uint8(player) || snapshot.NativeEffects[index].State != 22 {
			return fmt.Errorf("invalid saved lightning marker reference")
		}
	}
	for index, actor := range snapshot.NativeEffects {
		if !actor.Active || actor.Kind != 0x28 && actor.Kind != 0x2a {
			continue
		}
		if actor.Kind == 0x2a {
			marker, ok := lightningSlot(snapshot.LightningState.Word28[index])
			if !ok || !snapshot.NativeEffects[marker].Active || snapshot.NativeEffects[marker].Kind != 0x28 {
				return fmt.Errorf("invalid saved lightning bolt marker")
			}
		}
		if actor.Kind == 0x28 && actor.State == 26 {
			// Dismissal retains the old bolt-head word while its entries can
			// already be inactive or recycled. The outro does not traverse it.
			continue
		}
		seen := make(map[NativeRecordReference]bool)
		for ref := snapshot.LightningState.Word26[index]; ref != 0; {
			if seen[ref] {
				return fmt.Errorf("cyclic saved lightning bolt chain")
			}
			seen[ref] = true
			slot, ok := lightningSlot(ref)
			if !ok {
				return fmt.Errorf("invalid saved lightning chain reference")
			}
			bolt := snapshot.NativeEffects[slot]
			if !bolt.Active || bolt.Kind != 0x2a || bolt.Player != actor.Player {
				return fmt.Errorf("saved active lightning chain contains another actor")
			}
			ref = snapshot.LightningState.Word26[slot]
		}
	}
	for index, record := range snapshot.LightningVictims {
		if !record.Active {
			continue
		}
		v := record.Victim
		if snapshot.Version < 14 || index >= len(snapshot.Core.Peeps) || v.Owner < 1 || v.Owner > 2 || v.State != 0x1c && v.State != 0x1e && v.State != 0x20 && v.State != 0x22 {
			return fmt.Errorf("invalid saved lightning victim state")
		}
		p := snapshot.Core.Peeps[index]
		if v.Owner != p.Player+1 || v.Population != int32(p.Population) || v.Kind != 2 && v.Kind != 4 {
			return fmt.Errorf("saved lightning victim identity mismatch")
		}
		if _, ok := bundle.LightningRules.Frames[v.Animation]; !ok {
			return fmt.Errorf("invalid saved lightning victim frame")
		}
		if v.EffectReference != 0 {
			if _, ok := lightningSlot(v.EffectReference); !ok {
				return fmt.Errorf("invalid saved lightning victim effect reference")
			}
		}
	}
	return nil
}
