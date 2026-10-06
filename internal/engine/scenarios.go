package engine

// TerrainEditAllowed is the player's height admission. It is independent of
// population proximity and mana affordability. Direct effect terrain changes
// use their own creator policies rather than this player cursor rule.
func (s ScenarioOptions) TerrainEditAllowed(height int, raise bool) bool {
	if height < 0 || height > 8 || raise && s.ForbidRaise || !raise && s.ForbidLower {
		return false
	}
	if s.BuildAnywhere {
		return true
	}
	if !s.SeaLevelOnly {
		return false
	}
	if raise {
		return height == 0
	}
	return height <= 1
}

func (w *World) terrainPlanAllowed(owner, x, y int, raise bool) bool {
	return w.terrainPlanAllowedCommand(owner, x, y, raise, true)
}
func (w *World) terrainPlanAllowedCommand(owner, x, y int, raise, cursorAdmission bool) bool {
	if owner < 0 || owner > 1 || !insideCorner(x, y) {
		return false
	}
	options := w.Level.Players[owner].Scenario
	if cursorAdmission && !options.TerrainEditAllowed(int(w.Heights[x+y*CornerSize]), raise) || raise && options.ForbidRaise || !raise && options.ForbidLower {
		return false
	}
	before := w.Heights
	if raise {
		w.raiseCorner(x, y)
	} else {
		w.lowerCorner(x, y)
	}
	allowed := true
	for cy := 0; cy < MapSize && allowed; cy++ {
		for cx := 0; cx < MapSize; cx++ {
			at := cx + cy*CornerSize
			changed := before[at] != w.Heights[at] || before[at+1] != w.Heights[at+1] || before[at+CornerSize] != w.Heights[at+CornerSize] || before[at+CornerSize+1] != w.Heights[at+CornerSize+1]
			if !changed {
				continue
			}
			if wall := w.WallAt(cx, cy); wall >= 0 && !w.Earth.Walls[wall].Broken {
				allowed = false
				break
			}
			if options.ForbidEnemyTerrain && w.Farms[cx+cy*MapSize] == uint8((owner^1)+1) {
				allowed = false
				break
			}
		}
	}
	w.Heights = before
	return allowed
}

func (w *World) FollowerVisibleOnMap(observer, owner int) bool {
	if observer < 0 || observer > 1 || owner < 0 || owner > 1 {
		return false
	}
	return observer == owner || !w.Level.Players[observer].Scenario.HideEnemy
}
func (w *World) EffectVisibleOnMap(observer int) bool {
	return observer >= 0 && observer < 2 && !w.Level.Players[observer].Scenario.HideDisasters
}

type TerrainDeathState struct {
	Active        bool
	Frame, Frames uint16
}

// advanceWater handles stranded swimmers and the fatal-water campaign option.
// Fatal or exhausted swimmers remain mapped until their death sequence ends.
func (w *World) advanceWater(id int) bool {
	f := &w.Followers[id]
	if f.TerrainDeath.Active {
		f.TerrainDeath.Frame++
		f.Frame = f.TerrainDeath.Frame
		if f.TerrainDeath.Frame >= f.TerrainDeath.Frames {
			w.remove(id)
		}
		return true
	}
	if f.ImmuneToDrowning() {
		return false
	}
	water := w.Cell(int(f.X), int(f.Y)).IsWater()
	if !water && f.State != Drowning {
		return false
	}
	f.State = Drowning
	f.Frame = (f.Frame + 1) % 2
	options := w.Level.Players[f.Owner].Scenario
	alive := !options.FatalWater
	if alive {
		before := f.Population
		amount := w.Level.Players[f.Owner].Attrition
		f.Population = int(int32(uint32(before) - uint32(amount)))
		alive = int64(int32(before))-int64(int32(uint32(amount))) > 0
	}
	if !alive {
		frames := f.WaterDeathFrames()
		w.PrepareFollowerDeath(id)
		f.TerrainDeath = TerrainDeathState{Active: true, Frame: 1, Frames: uint16(frames)}
		f.State = Ruin
		f.Population = 0
		f.Frame = 1
		w.clearHeroLinks(id)
		return true
	}
	if water && f.Owner < 2 {
		w.AI[f.Owner].WaterRequestFollower = id
	}
	if !water {
		f.State = Walking
		f.Frame = 0
		return false
	}
	return true
}

// Sprog requests a town's next emigration cycle. It precedes the lower-ground
// prohibition in the original right-click command and does not spend mana.
func (w *World) Sprog(owner, x, y int) bool {
	if owner < 0 || owner > 1 || !inside(x, y) || w.Level.Players[owner].Scenario.DisableEmigration {
		return false
	}
	var occupants [FollowerCapacity]int
	for _, id := range occupants[:w.FollowersAt(x, y, occupants[:])] {
		f := &w.Followers[id]
		if int(f.Owner) == owner && f.State == Town {
			f.ForceEmigration = true
			return true
		}
	}
	return false
}

// DetectResult checks the first faction before the second, as the original
// campaign pass does. Editor worlds suppress automatic results.
func (w *World) DetectResult() int {
	if w.Editor {
		return 0
	}
	if w.Players[0].Population == 0 {
		return 2
	}
	if w.Players[1].Population == 0 {
		return 1
	}
	return 0
}

// RefreshAIChoices recompiles the named power catalogue after live campaign
// rules change. It does not reset reaction/cooldown timers or world state.
func (w *World) RefreshAIChoices() {
	for owner := range w.Players {
		w.compileAIPowers(owner)
	}
}
