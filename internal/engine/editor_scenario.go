package engine

import "fmt"

// EditorSetScenarioEvent validates a detached map's event and updates both its
// typed schedule and the campaign-compatible data record. It does not cast a
// power, consume mana, or reinterpret original program instructions.
func (w *World) EditorSetScenarioEvent(index int, event ScenarioEvent) error {
	if w == nil || !w.Editor || index < 0 || index >= len(w.Scenario.Events) || event.X >= MapSize || event.Y >= MapSize || event.Direction > 3 {
		return fmt.Errorf("invalid editor scenario event")
	}
	record, err := EncodeScenarioEvent(event)
	if err != nil {
		return err
	}

	w.Scenario.Events[index] = event
	if index < len(w.Level.WorldParameters)/6 {
		at := index * 6
		copy(w.Level.WorldParameters[at:at+6], record[:])

	}

	return nil
}

func (w *World) EditorRegenerate() (*World, error) {
	if w == nil || !w.Editor {
		return nil, fmt.Errorf("no detached editor map")
	}
	level := w.Level
	level.Seed = uint32(w.random)
	fresh, err := NewWorld(level, w.Landscape)
	if err != nil {
		return nil, err
	}
	fresh.Editor = true
	for owner := range fresh.Players {
		fresh.Players[owner].Computer = w.Players[owner].Computer
		fresh.Players[owner].Assisted = w.Players[owner].Assisted
		fresh.Players[owner].Experience = w.Players[owner].Experience
	}
	return fresh, nil
}

// EditorCycleScenery retains the original paint brush's repeated-click art
// cycle. Pool position and age stay unchanged while the four variants rotate.
func (w *World) EditorCycleScenery(kind SceneryKind, x, y int) error {
	if w == nil || !w.Editor {
		return fmt.Errorf("no detached editor map")
	}
	return w.cycleSceneryBrush(kind, x, y)
}
func (w *World) cycleSceneryBrush(kind SceneryKind, x, y int) error {
	if w == nil || !inside(x, y) || (kind != SceneryTree && kind != SceneryBoulder) {
		return fmt.Errorf("invalid editor scenery brush")
	}
	for ref, visits := w.Actors.Heads[x+y*MapSize], 0; ref.Kind != ActorNone && visits < FollowerCapacity+EffectCapacity+SceneryCapacity+WallCapacity+2; visits++ {
		if ref.Kind == ActorScenery && int(ref.Index) < len(w.Nature.Scenery) {
			scenery := &w.Nature.Scenery[ref.Index]
			if scenery.Kind == kind {
				scenery.Variant = (scenery.Variant + 1) & 3
				scenery.Frame = 0
				return nil
			}
		}
		ref = w.Actors.Next(ref)
	}
	return w.EditorPlaceScenery(kind, x, y)
}

// EditorRemoveFirst follows the original paint right-click: skip permanent
// magnets and remove one eligible actor from the head of the mixed chain.
// Terrain, other actors and the remainder of the parcel stay unchanged.
func (w *World) EditorRemoveFirst(x, y int) error {
	if w == nil || !w.Editor {
		return fmt.Errorf("no detached editor map")
	}
	return w.removeFirstPaintActor(x, y)
}
func (w *World) removeFirstPaintActor(x, y int) error {
	if w == nil || !inside(x, y) {
		return fmt.Errorf("invalid editor removal parcel")
	}
	for ref, visits := w.Actors.Heads[x+y*MapSize], 0; ref.Kind != ActorNone && visits < FollowerCapacity+EffectCapacity+SceneryCapacity+WallCapacity+2; visits++ {
		id := int(ref.Index)
		switch ref.Kind {
		case ActorFollower:
			w.remove(id)
			w.repaintFarms()
			w.summarize()
			return nil
		case ActorScenery:
			w.Nature.Scenery[id] = SceneryActor{}
			w.Actors.Unlink(ref)
			return nil
		case ActorWall:
			owner := w.Earth.Walls[id].Owner
			current, previous := w.Earth.WallHeads[owner], uint16(0)
			for steps := 0; current != 0 && steps < WallCapacity; steps++ {
				slot := int(current) - 1
				if slot == id {
					next := w.Earth.Walls[slot].Next
					if previous == 0 {
						w.Earth.WallHeads[owner] = next
					} else {
						w.Earth.Walls[previous-1].Next = next
					}
					break
				}
				previous, current = current, w.Earth.Walls[slot].Next
			}
			w.Earth.Walls[id].Active = false
			w.Actors.Unlink(ref)
			return nil
		case ActorEffect:
			for owner, slot := range w.Air.MarkerSlots {
				if slot == id+1 {
					w.Air.MarkerSlots[owner] = 0
				}
			}
			w.deactivateEffectActor(id)
			w.releaseEffect(id)
			return nil
		}
		ref = w.Actors.Next(ref)
	}
	return nil
}
