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
	if nx >= 0 && ny >= 0 && inside(x, y) && (x != int(f.X) || y != int(f.Y)) && f.Owner < 2 {
		cell := w.Cell(x, y)
		if cell.BaseAltitude == 0 && cell.Shape&1 == 0 {
			a := &w.AI[f.Owner]
			a.TerrainRequestFollower = id
			a.TerrainRequestX = x
			a.TerrainRequestY = y
		}
	}
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
		w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, nx, ny)
		if other != 0 {
			if f.Hero.CaptiveOf != 0 {
				return
			}
			if w.Followers[other].Owner == f.Owner && w.Followers[other].State == Town && !f.IsHero() {
				w.mergeFollowers(id, other)
				return
			}
			w.prepareContact(id, other)
			return
		}

		if !f.IsHero() && f.Hero.CaptiveOf == 0 && w.Players[f.Owner].Mode != Rally && w.Tick >= f.SettleAfter && settlementLand(w.Cell(x, y).Code) {
			w.RecordFoundingAttempt(int(f.Owner))
			if stage := w.EvaluateTown(id); stage > 0 {
				f.State = Town
				f.Stage = uint8(stage)
				f.Work = uint16(f.legRemaining)
				f.FoundedAt = w.Tick
				f.Frame = uint16(stage)
				f.moving = false
				f.positionX = x*256 + 128
				f.positionY = y*256 + 128
				f.positionSet = true
				w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, f.positionX, f.positionY)
				w.stepFollower(id)
				return
			}
		}

	}
	f.positionX, f.positionY = nx, ny
	w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, nx, ny)
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

// beginSearchLeg uses the original ordinary-search target fractions (zero).
// Velocity signs compare cell coordinates for another-cell targets; only a
// same-cell contact uses its fractional distance. Hero homing centers its own
// position separately and does not route through this search constructor.
func (w *World) beginSearchLeg(id, x, y int) bool {
	f := &w.Followers[id]
	f.initialisePosition()
	if f.MovementSpeed == 0 || !inside(x, y) {
		return false
	}
	speed := int(f.MovementSpeed)
	dx, dy := x-int(f.X), y-int(f.Y)
	f.velocityX, f.velocityY = sign(dx)*speed, sign(dy)*speed
	if dx == 0 && dy == 0 {
		f.velocityX, f.velocityY = -sign(f.positionX&255)*speed, -sign(f.positionY&255)*speed
		f.legRemaining = max(f.positionX&255, f.positionY&255) / speed
	} else {
		f.legRemaining = 256 / speed
	}
	if f.velocityX == 0 && f.velocityY == 0 {
		return false
	}
	for direction, d := range directions {
		if d == [2]int{sign(f.velocityX), sign(f.velocityY)} {
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
