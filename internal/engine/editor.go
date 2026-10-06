package engine

import "fmt"

// EditorSetHeight changes geometric corners without player mana or campaign
// prohibitions. Recursive slope propagation and overlay invalidation remain
// the same terrain operations used by the simulation's direct effects.
func (w *World) EditorSetHeight(x, y, height int) error {
	if !insideCorner(x, y) || height < 0 || height > 8 {
		return fmt.Errorf("invalid editor corner or height")
	}
	for int(w.Heights[x+y*CornerSize]) < height {
		w.directFireTerrain(x, y, true)
	}
	for int(w.Heights[x+y*CornerSize]) > height {
		w.directFireTerrain(x, y, false)
	}
	return nil
}

func (w *World) EditorPlaceFollower(owner, x, y, population int) error {
	if owner < 0 || owner > 1 || !inside(x, y) || population <= 0 || population > 2147483647 {
		return fmt.Errorf("invalid editor follower")
	}
	speed := w.Level.Players[owner].MovementSpeed
	if speed == 0 {
		speed = 20
	}
	id := w.allocate(Follower{Owner: uint8(owner), X: uint8(x), Y: uint8(y), PreviousX: uint8(x), PreviousY: uint8(y), State: Walking, Population: population, MovementSpeed: speed, Search: 2, Weapons: w.Level.Players[owner].Weapons})
	if id == 0 {
		return fmt.Errorf("follower pool exhausted")
	}
	w.linkFollower(id)
	if w.Players[owner].Leader == 0 {
		w.Players[owner].Leader = id
	}
	w.Result = 0
	w.summarize()
	return nil
}

func (w *World) EditorPlaceScenery(kind SceneryKind, x, y int) error {
	if (kind != SceneryTree && kind != SceneryBoulder) || !inside(x, y) {
		return fmt.Errorf("invalid editor scenery")
	}
	if w.Nature.sceneryAt(x, y) >= 0 {
		return fmt.Errorf("scenery already occupies this parcel")
	}
	for id := range w.Nature.Scenery {
		if w.Nature.Scenery[id].Kind == SceneryNone {
			w.Nature.Scenery[id] = SceneryActor{Kind: kind, X: uint8(x), Y: uint8(y), Age: 24}
			w.Actors.Link(ActorRef{Kind: ActorScenery, Index: uint16(id)}, x*256+128, y*256+128)
			return nil
		}
	}
	return fmt.Errorf("scenery pool exhausted")
}

// EditorClearCell removes objects only on the selected parcel. Geometry is
// retained; actor links, shared reservations, hero claims and farms are cleaned
// through the normal lifecycle instead of resetting an entire map region.
func (w *World) EditorClearCell(x, y int) error {
	if !inside(x, y) {
		return fmt.Errorf("invalid editor parcel")
	}
	for id := 1; id < FollowerCapacity; id++ {
		f := w.Followers[id]
		if f.State != Inactive && int(f.X) == x && int(f.Y) == y {
			w.remove(id)
		}
	}
	for id, a := range w.Nature.Scenery {
		if a.Kind != SceneryNone && int(a.X) == x && int(a.Y) == y {
			w.Nature.Scenery[id] = SceneryActor{}
			w.Actors.Unlink(ActorRef{Kind: ActorScenery, Index: uint16(id)})
		}
	}
	for id, a := range w.Earth.Walls {
		if a.Active && a.X == x && a.Y == y {
			owner := a.Owner
			head := w.Earth.WallHeads[owner]
			previous := uint16(0)
			for visits := 0; head > 0 && visits < WallCapacity; visits++ {
				slot := int(head) - 1
				next := w.Earth.Walls[slot].Next
				if slot == id {
					if previous == 0 {
						w.Earth.WallHeads[owner] = next
					} else {
						w.Earth.Walls[int(previous)-1].Next = next
					}
					break
				}
				previous, head = head, next
			}
			w.Earth.Walls[id].Active = false
			w.Actors.Unlink(ActorRef{Kind: ActorWall, Index: uint16(id)})
		}
	}
	w.syncActorRegistry()
	for id := range w.effects.Slots {
		ref := ActorRef{Kind: ActorEffect, Index: uint16(id)}
		px, py, linked := w.Actors.Position(ref)
		if linked && px>>8 == x && py>>8 == y {
			for owner := range w.Air.MarkerSlots {
				if w.Air.MarkerSlots[owner] == id+1 {
					w.Air.MarkerSlots[owner] = 0
				}
			}
			for owner := range w.Nature.PendingFungus {
				if w.Nature.PendingFungus[owner] == uint16(id+1) {
					w.Nature.PendingFungus[owner] = 0
				}
			}
			w.Nature.Fungi[id].Active = false
			w.deactivateEffectActor(id)
			w.releaseEffect(id)
			w.Actors.Unlink(ref)
		}
	}
	at := x + y*MapSize
	w.Nature.Ground[at] = GroundParcel{}
	w.FireDamage.Painted[at] = false
	w.Water.Painted[at] = false
	w.Earth.Roads[at] = RoadParcel{}
	w.Earth.Cracks[at] = CrackParcel{}
	w.repaintFarms()
	w.summarize()
	return nil
}

// EditorSetTerrain replaces a complete corner-height preset in one operation.
// Heights and all eight neighbouring slopes are validated before any state
// changes. Existing actors keep their normal lifecycle: edits invalidate only
// changed ground, while the next simulation pass handles terrain hazards.
func (w *World) EditorSetTerrain(heights [CornerSize * CornerSize]uint8) error {
	for y := 0; y < CornerSize; y++ {
		for x := 0; x < CornerSize; x++ {
			height := heights[x+y*CornerSize]
			if height > 8 {
				return fmt.Errorf("editor terrain height %d at %d,%d exceeds eight", height, x, y)
			}
			// Four forward neighbours cover every horizontal, vertical and diagonal
			// pair once, including the east and south boundary vertices.
			for _, d := range [4][2]int{{1, 0}, {0, 1}, {1, 1}, {-1, 1}} {
				nx, ny := x+d[0], y+d[1]
				if insideCorner(nx, ny) && abs(int(height)-int(heights[nx+ny*CornerSize])) > 1 {
					return fmt.Errorf("editor terrain slope between %d,%d and %d,%d exceeds one", x, y, nx, ny)
				}
			}
		}
	}
	before := w.Heights
	w.Heights = heights
	for y := 0; y < MapSize; y++ {
		for x := 0; x < MapSize; x++ {
			at := x + y*CornerSize
			if before[at] == heights[at] && before[at+1] == heights[at+1] && before[at+CornerSize] == heights[at+CornerSize] && before[at+CornerSize+1] == heights[at+CornerSize+1] {
				continue
			}
			w.ClearFireTerrain(x, y)
			w.ClearEarthTerrain(x, y)
			w.ClearWaterTerrain(x, y)
			w.Nature.Ground[x+y*MapSize] = GroundParcel{}
			w.Pressure[x+y*MapSize] = 0
		}
	}
	w.rebuildCells()
	return nil
}
