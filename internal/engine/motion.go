package engine

// Position returns the follower's continuous map coordinates. Geometry uses
// 256 subunits per cell so walking remains smooth independently of cell-entry
// events, occupancy updates, and the presentation frame rate.
func (f Follower) Position() (float64, float64) {
	if f.positionSet {
		return float64(f.positionX) / 256, float64(f.positionY) / 256
	}
	return float64(f.X) + 0.5, float64(f.Y) + 0.5
}

func (f *Follower) initialisePosition() {
	if f.positionSet {
		return
	}
	f.positionX = int(f.X)*256 + 128
	f.positionY = int(f.Y)*256 + 128
	f.positionSet = true
}

func (w *World) beginLeg(id, x, y int) bool {
	f := &w.Followers[id]
	f.initialisePosition()
	if f.MovementSpeed == 0 {
		return false
	}
	speed := int(f.MovementSpeed)
	targetX, targetY := x*256+128, y*256+128
	dx, dy := targetX-f.positionX, targetY-f.positionY
	f.velocityX, f.velocityY = sign(dx)*speed, sign(dy)*speed
	if x == int(f.X) && y == int(f.Y) {
		f.legRemaining = max(abs(dx), abs(dy)) / speed
	} else {
		f.legRemaining = 256 / speed
	}
	for direction, d := range directions {
		if d == [2]int{sign(dx), sign(dy)} {
			f.Direction = uint8(direction)
			break
		}
	}
	f.Target = x + y*MapSize
	f.moving = true
	f.PreviousX, f.PreviousY = f.X, f.Y
	f.MoveProgress = 0
	return true
}

func (w *World) advanceLeg(id int) {
	f := &w.Followers[id]
	if !f.moving {
		return
	}
	f.Frame = (f.Frame + 1) % 4
	f.legRemaining--
	if f.legRemaining < 0 {
		f.moving = false
		f.MoveProgress = 0
		return
	}
	nx, ny := f.positionX+f.velocityX, f.positionY+f.velocityY
	x, y := nx/256, ny/256
	if nx < 0 || ny < 0 || !inside(x, y) || w.Tiles[x+y*MapSize].IsWater() && !f.ImmuneToDrowning() {
		f.moving = false
		f.MoveProgress = 0
		return
	}
	oldX, oldY := int(f.X), int(f.Y)
	if x != oldX || y != oldY {
		if wall := w.WallAt(x, y); wall >= 0 {
			switch w.DecideWallCrossing(id, wall) {
			case WallBlocked:
				f.moving = false
				return
			case WallBreak:
				w.BreakWall(wall)
				f.moving = false
				f.Frame = 0
				return
			}
		}
		other := int(w.Occupants[x+y*MapSize])
		if other == id {
			other = w.Followers[other].NextFollower
		}
		w.moveFollowerCell(id, x, y)
		f.positionX, f.positionY = nx, ny
		if other != 0 {
			if f.Hero.CaptiveOf != 0 {
				return
			}
			w.prepareContact(id, other)
			return
		}

	}
	f.positionX, f.positionY = nx, ny
	// This field remains available for UI consumers; Position is authoritative.
	f.MoveProgress = uint8(min(255, max(abs(nx-(int(f.PreviousX)*256+128)), abs(ny-(int(f.PreviousY)*256+128)))))
}
func sign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}
