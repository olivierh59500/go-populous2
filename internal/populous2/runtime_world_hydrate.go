package populous2

import legacy "go-populous2/internal/legacy"

// hydrateNativeRuntimeGraph copies common raw fields after a native primitive.
// Membership and raw map heads belong to that primitive and are not inferred
// from population, owner, or links. Inactive records retain their coordinates.
func (w *World) hydrateNativeRuntimeGraph() {
	access := w.runtimeMemory().RecordAccess()
	for _, pool := range []struct {
		kind    NativeRecordPool
		records []NativeWorldOccupancyRecord
	}{
		{NativeWallPool, w.Occupancy.Walls[:]},
		{NativeSceneryPool, w.Occupancy.Scenery[:]},
		{NativeFollowerPool, w.Occupancy.Followers[:]},
		{NativeEffectPool, w.Occupancy.Effects[:]},
		{NativeMagnetPool, w.Occupancy.Magnets[:]},
	} {
		for index := range pool.records {
			ref, _ := NativeWorldReference(pool.kind, index)
			record, ok := access.Record(ref)
			if !ok {
				panic("native runtime common record unavailable")
			}
			pool.records[index].Record = record
			if pool.kind == NativeFollowerPool {
				actor := &w.NativeFollowers[index].Actor
				actor.Next, actor.Previous = uint16(record.Next), uint16(record.Previous)
				actor.X, actor.Y = int16(record.X), int16(record.Y)
			}
		}
	}
}

// Allocation and dispatch use the nonzero owner byte, independently of a
// positive population. Native death and ruin records must not be reused early.
func (w *World) nativeRuntimeFollowerReserved(index int) bool {
	if index < 0 || index >= NativeWorldFollowerCapacity {
		return false
	}
	ref := nativeActorReference(NativeFollowerPool, index)
	owner, err := w.RecordImage.Read8(ref, 12)
	return err == nil && owner != 0
}

func nativeRuntimeManagedFollower(actor FollowerEntryActor) bool {
	if actor.Owner == 0 {
		return false
	}
	if actor.Motion.Population <= 0 || actor.Motion.Flags&(2|8) != 0 {
		return true
	}
	if actor.Motion.Kind == 4 {
		return actor.Motion.State != 6
	}
	if actor.Motion.Kind != 2 {
		return true
	}
	if actor.Motion.State == 4 {
		return actor.Motion.ReturnState != 2 && actor.Motion.ReturnState != 18
	}
	return actor.Motion.State != 2 && actor.Motion.State != 18
}

// hydrateNativeRuntimeRecords runs after a complete native helper, never during
// its intermediate assignments. It reads raw codecs directly and does not call
// readEntryRecord or another inherited projection back into the retained image.
func (w *World) hydrateNativeRuntimeRecords() {
	w.hydrateNativeRuntimeGraph()
	memory := w.runtimeMemory()
	// An alias or native clone may allocate a previously unrepresented slot.
	// Keep reserved slot0 separate and extend only for actual nonzero owners.
	last := len(w.Core.Peeps)
	for index := 0; index < NativeWorldFollowerCapacity; index++ {
		ref := nativeActorReference(NativeFollowerPool, index)
		owner, err := w.RecordImage.Read8(ref, 12)
		if err != nil {
			panic(err)
		}
		if owner != 0 && index >= last {
			last = index + 1
		}
	}
	for len(w.Core.Peeps) < last {
		w.Core.Peeps = append(w.Core.Peeps, legacy.Peep{})
	}
	for index := 0; index < NativeWorldFollowerCapacity; index++ {
		ref := nativeActorReference(NativeFollowerPool, index)
		actor, err := w.RecordImage.ReadFollowerEntry(ref)
		if err != nil {
			panic(err)
		}
		managed := nativeRuntimeManagedFollower(actor)
		w.NativeEntries[index] = NativeFollowerEntry{Initialized: true, Managed: managed, Actor: actor}
		generation := w.NativeFollowers[index].Generation
		if generation == 0 && actor.Owner != 0 {
			generation = 1
		}
		ordinary := actor.Owner != 0 && !managed && actor.Motion.Kind == 2 && (actor.Motion.State == 2 || actor.Motion.State == 4 || actor.Motion.State == 18)
		w.NativeFollowers[index] = NativeFollower{Active: ordinary, Generation: generation, Actor: actor.Motion}
		if index >= len(w.Core.Peeps) {
			continue
		}
		p := &w.Core.Peeps[index]
		p.Population, p.Weapons, p.MovementSpeed = int(actor.Motion.Population), int(actor.Weapon), actor.Motion.Speed
		search, err := w.RecordImage.Read8(ref, 24)
		if err != nil {
			panic(err)
		}
		p.IQ = int(search)
		p.AtPos = int(uint16(actor.Motion.X)>>8) + int(uint16(actor.Motion.Y)>>8)*64
		if actor.Owner != 0 && w.Occupancy.Followers[index].Linked && p.AtPos >= 0 && p.AtPos < 4096 && w.Core.MapWho[p.AtPos] == 0 {
			w.Core.MapWho[p.AtPos] = uint16(index + 1)
		}
		if actor.Owner == 0 {
			w.Core.ReleaseDeathOccupancy(index)
		}
		if actor.Owner >= 1 && actor.Owner <= 2 {
			p.Player = actor.Owner - 1
		}
		w.captiveIndex[index] = actor.Motion.Flags&8 != 0
		if actor.Owner == 0 {
			p.Flags = 0
			w.Heroes[index].Active = false
			continue
		}
		p.Plague = actor.Motion.Flags&16 != 0
		if actor.Motion.Flags&2 != 0 && actor.Hero40&1 == 0 && actor.Hero40 <= 10 {
			hero := &w.Heroes[index]
			hero.Active, hero.Spell, hero.Player, hero.Population, hero.Speed = true, heroIDs[actor.Hero40/2], int(p.Player), p.Population, p.MovementSpeed
			p.Status = legacy.KnightStatus
		} else {
			w.Heroes[index].Active = false
			p.Status = 0
		}
		switch actor.Motion.Kind {
		case 4:
			p.Flags, p.TownStage = legacy.InTown, int(actor.Byte1)
			if actor.Motion.State == 6 {
				p.TownWork = int(uint16(actor.Motion.Timer))
			}
		case 2:
			p.Flags = legacy.OnMove
		default:
			p.Flags = legacy.InEffect
		}
		if ordinary {
			p.Frame = actor.Motion.Animation / 4
		}
	}
	for index := range w.Walls.Actors {
		ref := nativeActorReference(NativeWallPool, index)
		kind, _ := w.RecordImage.Read8(ref, 0)
		owner, _ := w.RecordImage.Read8(ref, 12)
		variant, _ := w.RecordImage.Read8(ref, 1)
		animation, _ := w.RecordImage.Read16(ref, 10)
		next, _ := w.RecordImage.Read16(ref, 14)
		graph := w.Occupancy.Walls[index].Record
		a := &w.Walls.Actors[index]
		a.Active, a.Broken, a.Player, a.Variant, a.Animation, a.X, a.Y = owner != 0, kind == 0x1c, owner-1, variant, int(animation), int(graph.X>>8), int(graph.Y>>8)
		a.Next = 0
		if location, ok := LocateNativeRecord(NativeRecordReference(next)); ok && location.Pool == NativeWallPool {
			a.Next = uint16(location.Index + 1)
		}
	}
	for index := range w.Scenery {
		ref := nativeActorReference(NativeSceneryPool, index)
		kind, _ := w.RecordImage.Read8(ref, 0)
		owner, _ := w.RecordImage.Read8(ref, 12)
		age, _ := w.RecordImage.Read8(ref, 1)
		animation, _ := w.RecordImage.Read16(ref, 10)
		graph := w.Occupancy.Scenery[index].Record
		a := &w.Scenery[index]
		a.Active, a.Age, a.Removing, a.X, a.Y = owner != 0, int8(age), kind == 0x1e, int(graph.X>>8), int(graph.Y>>8)
		a.Kind = SceneryKind(kind)
		if kind == 0x1e {
			a.Kind = SceneryTree
		}
		a.Animation, a.Frame = int(animation), 0
		if w.SceneryBank != nil {
			for start, frames := range w.SceneryBank.Frames {
				if int(animation) >= start && int(animation) < start+len(frames)*4 && (int(animation)-start)%4 == 0 {
					a.Animation, a.Frame = start, (int(animation)-start)/4
					break
				}
			}
		}
	}
	for index := range w.NativeEffects {
		ref := nativeActorReference(NativeEffectPool, index)
		kind, _ := w.RecordImage.Read8(ref, 0)
		owner, _ := w.RecordImage.Read8(ref, 12)
		x, _ := w.RecordImage.Read16(ref, 6)
		y, _ := w.RecordImage.Read16(ref, 8)
		vx, _ := w.RecordImage.Read16(ref, 14)
		vy, _ := w.RecordImage.Read16(ref, 16)
		animation, _ := w.RecordImage.Read16(ref, 10)
		speed, _ := w.RecordImage.Read8(ref, 18)
		timer, _ := w.RecordImage.Read16(ref, 20)
		state, _ := w.RecordImage.Read8(ref, 22)
		life, _ := w.RecordImage.Read16(ref, 24)
		w.NativeEffects[index] = NativeEffectActor{Active: owner != 0, Kind: kind, Player: owner - 1, X: int16(x), Y: int16(y), VX: int16(vx), VY: int16(vy), Animation: int(animation), Speed: speed, Timer: int16(timer), State: state, Life: int16(life)}
		word26, _ := w.RecordImage.Read16(ref, 26)
		word28, _ := w.RecordImage.Read16(ref, 28)
		word30, _ := w.RecordImage.Read16(ref, 30)
		switch kind {
		case BasaltActorKind:
			w.BasaltState.Directions[index] = word26
		case 0x28, 0x2a:
			w.LightningState.Word26[index], w.LightningState.Word28[index], w.LightningState.RandomWords[index] = NativeRecordReference(word26), NativeRecordReference(word28), word30
		case FungusActorKind:
			w.FungusState.Bounds[index] = FungusBounds{DX: uint8(word26 >> 8), DY: uint8(word26), WorkDX: uint8(word28 >> 8), WorkDY: uint8(word28)}
		}
	}
	for player := range w.Core.Magnets {
		owner := uint8(player + 1)
		deity, _ := NativeDeityAddress(owner)
		mana, _ := memory.Read32(deity)
		leader, _ := memory.Read16(deity + 8)
		w.Core.Magnets[player].Mana = int(int32(mana))
		w.Core.Magnets[player].Carried = 0
		if location, ok := LocateNativeRecord(NativeRecordReference(leader)); ok && location.Pool == NativeFollowerPool {
			w.Core.Magnets[player].Carried = location.Index
		}
		marker, _ := memory.Read16(deity + 0x0a)
		if _, ok := LocateNativeMagnet(NativeRecordReference(marker)); ok {
			record, _ := memory.RecordAccess().Record(NativeRecordReference(marker))
			w.Core.Magnets[player].GoTo = int(record.X>>8) + int(record.Y>>8)*64
		}
	}
	w.rebuildSceneryIndex()
}

// Complete-helper hydration is intentionally shared by patch callers: later
// aliases can affect a different family or a newly allocated follower. The
// routine only projects raw data; it never writes the retained images back.
func (w *World) hydrateNativeRuntimePatch(_ NativeRuntimePatch) {
	w.hydrateNativeRuntimeRecords()
}
