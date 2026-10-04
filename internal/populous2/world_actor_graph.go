package populous2

import (
	"fmt"
	legacy "go-populous2/internal/legacy"
)

func nativeActorReference(pool NativeRecordPool, index int) NativeRecordReference {
	ref, ok := NativeWorldReference(pool, index)
	if !ok {
		panic("native actor slot outside its pool")
	}
	return ref
}

func (w *World) initializeActorGraph() {
	w.Occupancy = NativeWorldOccupancy{}
	for pos := range w.Occupancy.Grid.Cells {
		cell := w.TerrainCell(pos%64, pos/64)
		w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: uint8(cell.BaseAltitude) & 7, Tile: cell.Code}
	}
}

func (w *World) bindActorGraphHooks() {
	w.Core.OnFollowerMoved = func(index int) {
		if index < 0 || index >= len(w.Core.Peeps) || index >= NativeWorldFollowerCapacity {
			return
		}
		p := w.Core.Peeps[index]
		ref := nativeActorReference(NativeFollowerPool, index)
		record, _ := w.Occupancy.Record(ref)
		linked, _ := w.Occupancy.Linked(ref)
		if linked && int(record.X>>8)+int(record.Y>>8)*64 == p.AtPos {
			return // A native crossing already committed its exact fractions.
		}
		w.moveActor(NativeFollowerPool, index, uint16((p.AtPos%64)*256+128), uint16((p.AtPos/64)*256+128))
	}
	w.Core.OnFollowerRemoved = func(index int) {
		if index >= 0 && index < NativeWorldFollowerCapacity && !w.flameDeathIndex[index] {
			w.unlinkActor(NativeFollowerPool, index)
		}
	}
}

// reconcileActorGraph bridges remaining inherited handlers and diagnostic
// state edits. It preserves existing chain order/pressure and never rebuilds
// from scalar MapWho. Native allocation/motion paths use direct notifications.
func (w *World) reconcileActorGraph() {
	for index, entry := range w.Occupancy.Followers {
		alive := index < len(w.Core.Peeps) && (w.Core.Peeps[index].Population > 0 || w.flameDeathIndex[index])
		if !alive {
			if entry.Linked {
				w.unlinkActor(NativeFollowerPool, index)
			}
			continue
		}
		p := w.Core.Peeps[index]
		if w.flameDeathIndex[index] && entry.Linked {
			continue // Fire retains native fractions; Fungus centers at entry.
		}
		x, y := uint16((p.AtPos%64)*256+128), uint16((p.AtPos/64)*256+128)
		if fixedX, fixedY, ok := w.FollowerPosition(index); ok {
			x, y = uint16(fixedX), uint16(fixedY)
		}
		if !entry.Linked {
			w.placeActor(NativeFollowerPool, index, x, y)
		} else if entry.Record.X != x || entry.Record.Y != y {
			w.moveActor(NativeFollowerPool, index, x, y)
		}
	}
	for index, entry := range w.Occupancy.Scenery {
		actor := w.Scenery[index]
		if !actor.Active {
			if entry.Linked {
				w.unlinkActor(NativeSceneryPool, index)
			}
		} else if !entry.Linked {
			w.placeActor(NativeSceneryPool, index, uint16(actor.X*256+128), uint16(actor.Y*256+128))
		}
	}
	for index, entry := range w.Occupancy.Walls {
		actor := w.Walls.Actors[index]
		if !actor.Active {
			if entry.Linked {
				w.unlinkActor(NativeWallPool, index)
			}
		} else if !entry.Linked {
			w.placeActor(NativeWallPool, index, uint16(actor.X*256+128), uint16(actor.Y*256+128))
		}
	}
	for index, entry := range w.Occupancy.Effects {
		actor := w.NativeEffects[index]
		mapped := actor.Active && (actor.Kind == 0x20 || actor.Kind == 0x22 || actor.Kind == BasaltActorKind)
		if !mapped {
			if entry.Linked {
				w.unlinkActor(NativeEffectPool, index)
			}
		} else if !entry.Linked {
			w.linkEffect(index)
		}
	}
	w.refreshGraphTiles()
}

func (w *World) actorGraphSnapshot() NativeWorldOccupancy {
	copyWorld := *w
	copyWorld.reconcileActorGraph()
	return copyWorld.Occupancy
}

func (w *World) placeActor(pool NativeRecordPool, index int, x, y uint16) {
	ref := nativeActorReference(pool, index)
	linked, _ := w.Occupancy.Linked(ref)
	before, _ := w.Occupancy.Record(ref)
	if linked {
		if err := w.Occupancy.Remove(ref); err != nil {
			panic(err)
		}
	}
	if err := w.Occupancy.Place(ref, x, y); err != nil {
		panic(err)
	}
	after, _ := w.Occupancy.Record(ref)
	w.syncFollowerGraphReferences(ref, before.Next, before.Previous, after.Next, after.Previous)
}

func (w *World) unlinkActor(pool NativeRecordPool, index int) {
	ref := nativeActorReference(pool, index)
	linked, _ := w.Occupancy.Linked(ref)
	before, _ := w.Occupancy.Record(ref)
	if linked {
		if err := w.Occupancy.Remove(ref); err != nil {
			panic(err)
		}
	}
	w.syncFollowerGraphReferences(ref, before.Next, before.Previous)
}

func (w *World) moveActor(pool NativeRecordPool, index int, x, y uint16) {
	ref := nativeActorReference(pool, index)
	linked, _ := w.Occupancy.Linked(ref)
	if !linked {
		w.placeActor(pool, index, x, y)
		return
	}
	before, _ := w.Occupancy.Record(ref)
	if _, err := w.Occupancy.Move(ref, x, y); err != nil {
		panic(err)
	}
	after, _ := w.Occupancy.Record(ref)
	w.syncFollowerGraphReferences(ref, before.Next, before.Previous, after.Next, after.Previous)
}

func (w *World) syncFollowerGraphReferences(references ...NativeRecordReference) {
	for _, reference := range references {
		location, ok := LocateNativeRecord(reference)
		if !ok || location.Pool != NativeFollowerPool {
			continue
		}
		index := location.Index - 1
		record := w.Occupancy.Followers[index].Record
		w.NativeFollowers[index].Actor.Next, w.NativeFollowers[index].Actor.Previous = uint16(record.Next), uint16(record.Previous)
	}
}

func (w *World) linkEffect(index int) {
	actor := w.NativeEffects[index]
	w.placeActor(NativeEffectPool, index, uint16(actor.X), uint16(actor.Y))
}

func (w *World) castWhirlwind(player, x, y int) bool {
	for index := range w.NativeEffects {
		if !w.NativeEffects[index].Active {
			if !w.Whirlwinds.Create(&w.NativeEffects, player, x, y, w.Experience[player][Air]) {
				return false
			}
			w.linkEffect(index)
			return true
		}
	}
	return false
}

func (w *World) refreshChangedTerrain(before [legacy.EndWidth * legacy.EndWidth]int) {
	for y := range 64 {
		for x := range 64 {
			at := x + y*65
			if before[at] == w.Core.Alt[at] && before[at+1] == w.Core.Alt[at+1] && before[at+65] == w.Core.Alt[at+65] && before[at+66] == w.Core.Alt[at+66] {
				continue
			}
			cell := w.TerrainCell(x, y)
			if err := w.Occupancy.SetTerrain(x, y, uint8(cell.BaseAltitude), cell.Code, true); err != nil {
				panic(err)
			}
			w.setBasaltLegacyLand(x, y)
		}
	}
}

func (w *World) setBasaltLegacyLand(x, y int) {
	mark := w.Marks[x+y*64]
	if mark.Spell == Basalt && mark.NativeTile&0xf0 == 0xe0 {
		// The inherited route/water handlers use their own block IDs. Basalt
		// is traversable nonwater even when its vertex heights are all zero.
		w.Core.MapBlk[x+y*64] = legacy.FlatBlock
	}
}

func (w *World) refreshGraphTiles() {
	for pos := range w.Occupancy.Grid.Cells {
		cell := w.TerrainCell(pos%64, pos/64)
		w.Occupancy.Grid.Cells[pos].Tile = cell.Code
		w.Occupancy.Grid.Cells[pos].Header = w.Occupancy.Grid.Cells[pos].Header&0xf8 | uint8(cell.BaseAltitude)&7
	}
}

// The inherited handlers still use one follower reference per cell. Their
// contact adapter selects a live follower from the mixed native chain without
// replacing or rebuilding that chain. Exact native contacts are separate work.
func (w *World) legacyFollowerHead(pos, excluding int) uint16 {
	if pos < 0 || pos >= 4096 {
		return 0
	}
	for ref := w.Occupancy.Grid.Cells[pos].Head; ref != 0; {
		location, ok := LocateNativeRecord(ref)
		if !ok {
			panic("invalid native actor reference in contact adapter")
		}
		if location.Pool == NativeFollowerPool {
			index := location.Index - 1
			if index != excluding && index < len(w.Core.Peeps) && w.Core.Peeps[index].Population > 0 {
				return uint16(index + 1)
			}
		}
		record, _ := w.Occupancy.Record(ref)
		ref = record.Next
	}
	return 0
}

func validateSavedActorGraph(snapshot Snapshot) error {
	var deaths [NativeWorldFollowerCapacity]bool
	for _, death := range snapshot.FlameDeaths {
		if death.Follower >= 0 && death.Follower < len(deaths) {
			deaths[death.Follower] = true
		}
	}
	for index, entry := range snapshot.Occupancy.Followers {
		live := index < len(snapshot.Core.Peeps) && (snapshot.Core.Peeps[index].Population > 0 || deaths[index])
		if entry.Linked != live {
			return fmt.Errorf("saved follower graph membership differs at slot %d", index)
		}
		if live && int(entry.Record.X>>8)+int(entry.Record.Y>>8)*64 != snapshot.Core.Peeps[index].AtPos {
			return fmt.Errorf("saved follower graph position differs at slot %d", index)
		}
		if motion := snapshot.NativeFollowers[index]; motion.Active && (entry.Record.X != uint16(motion.Actor.X) || entry.Record.Y != uint16(motion.Actor.Y) || uint16(entry.Record.Next) != motion.Actor.Next || uint16(entry.Record.Previous) != motion.Actor.Previous) {
			return fmt.Errorf("saved follower motion and graph differ at slot %d", index)
		}
	}
	for index, entry := range snapshot.Occupancy.Scenery {
		actor := snapshot.Scenery[index]
		if entry.Linked != actor.Active || actor.Active && (entry.Record.X != uint16(actor.X*256+128) || entry.Record.Y != uint16(actor.Y*256+128)) {
			return fmt.Errorf("saved scenery graph differs at slot %d", index)
		}
	}
	for index, entry := range snapshot.Occupancy.Walls {
		actor := snapshot.Walls.Actors[index]
		if entry.Linked != actor.Active || actor.Active && (entry.Record.X != uint16(actor.X*256+128) || entry.Record.Y != uint16(actor.Y*256+128)) {
			return fmt.Errorf("saved wall graph differs at slot %d", index)
		}
	}
	for index, entry := range snapshot.Occupancy.Effects {
		actor := snapshot.NativeEffects[index]
		mapped := actor.Active && (actor.Kind == 0x20 || actor.Kind == 0x22 || actor.Kind == BasaltActorKind)
		if entry.Linked != mapped || mapped && (entry.Record.X != uint16(actor.X) || entry.Record.Y != uint16(actor.Y)) {
			return fmt.Errorf("saved effect graph differs at slot %d", index)
		}
	}
	return nil
}
