package engine

// Follower links form typed per-parcel chains. More than one group may occupy
// a cell while homing into contact, following a captor, or splitting Adonis.
func (w *World) linkFollower(id int) {
	if id <= 0 || id >= FollowerCapacity || w.Followers[id].State == Inactive {
		return
	}
	f := &w.Followers[id]
	f.initialisePosition()
	w.Actors.Link(ActorRef{Kind: ActorFollower, Index: uint16(id)}, f.positionX, f.positionY)
	at := int(f.X) + int(f.Y)*MapSize
	head := int(w.Occupants[at])
	if head == id {
		return
	}
	f.PreviousFollower = 0
	f.NextFollower = head
	if head > 0 && head < FollowerCapacity {
		w.Followers[head].PreviousFollower = id
	}
	w.Occupants[at] = uint16(id)
}
func (w *World) unlinkFollower(id int) {
	w.Actors.Unlink(ActorRef{Kind: ActorFollower, Index: uint16(id)})
	if id <= 0 || id >= FollowerCapacity {
		return
	}
	f := &w.Followers[id]
	at := int(f.X) + int(f.Y)*MapSize
	previous := 0
	for current, visits := int(w.Occupants[at]), 0; current > 0 && current < FollowerCapacity && visits < FollowerCapacity; visits++ {
		next := w.Followers[current].NextFollower
		if current == id {
			if previous == 0 {
				w.Occupants[at] = uint16(next)
			} else {
				w.Followers[previous].NextFollower = next
			}
			if next > 0 && next < FollowerCapacity {
				w.Followers[next].PreviousFollower = previous
			}
			break
		}
		previous, current = current, next
	}
	f.NextFollower, f.PreviousFollower = 0, 0
}
func (w *World) moveFollowerCell(id, x, y int) {
	f := &w.Followers[id]
	w.unlinkFollower(id)
	f.X, f.Y = uint8(x), uint8(y)
	f.positionX = x*256 + (f.positionX & 255)
	f.positionY = y*256 + (f.positionY & 255)
	w.linkFollower(id)
	w.Footsteps[x+y*MapSize]++
	w.Pressure[x+y*MapSize] += 8
}

// FollowersAt returns chain order without allocating. The returned length is
// bounded by the caller's buffer and the follower pool's capacity.
func (w *World) FollowersAt(x, y int, destination []int) int {
	if !inside(x, y) {
		return 0
	}
	count := 0
	for current, visits := int(w.Occupants[x+y*MapSize]), 0; current > 0 && current < FollowerCapacity && visits < FollowerCapacity && count < len(destination); visits++ {
		destination[count] = current
		count++
		current = w.Followers[current].NextFollower
	}
	return count
}
