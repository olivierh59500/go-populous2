package engine

var supportStages = [27]uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 18}
var townFootprint = [49][2]int{
	{0, 0}, {-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1},
	{-2, -2}, {0, -2}, {2, -2}, {-2, 0}, {2, 0}, {-2, 2}, {0, 2}, {2, 2},
	{-1, -2}, {1, -2}, {-2, -1}, {2, -1}, {-2, 1}, {2, 1}, {-1, 2}, {1, 2},
	{-3, -3}, {-2, -3}, {-1, -3}, {0, -3}, {1, -3}, {2, -3}, {3, -3},
	{-3, -2}, {3, -2}, {-3, -1}, {3, -1}, {-3, 0}, {3, 0}, {-3, 1}, {3, 1}, {-3, 2}, {3, 2},
	{-3, 3}, {-2, 3}, {-1, 3}, {0, 3}, {1, 3}, {2, 3}, {3, 3},
}

// settlementLand describes the six ground codes which support cultivation.
// The geometry may be at different elevations; support is parcel based.
func settlementLand(code uint8) bool {
	switch code {
	case 15, 31, 47, 63, 151, 245:
		return true
	}
	return false
}

// TownStage is the read-only support preview. Actual settlement evaluation
// also clears competing towns and claims farms in EvaluateTown.
func (w *World) TownStage(owner, x, y, ignoreID int) int {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return 0
	}
	return w.townSupport(owner, x, y, ignoreID, false)
}

func (w *World) townSupport(owner, x, y, ignoreID int, mutate bool) int {
	pending := [49]int{}
	pendingCount := 0
	visit := func(d [2]int, claim bool) bool {
		nx, ny := x+d[0], y+d[1]
		if !inside(nx, ny) || !settlementLand(w.Cell(nx, ny).Code) {
			return false
		}
		// Only a boulder blocks the occupancy scan. Trees and ordinary walkers
		// coexist with farms and are handled independently by scenery simulation.
		if w.Nature.BlocksWalking(nx, ny) {
			return false
		}
		at := nx + ny*MapSize
		if mutate && claim {
			var occupants [FollowerCapacity]int
			count := w.FollowersAt(nx, ny, occupants[:])
			for _, other := range occupants[:count] {
				if other != ignoreID && w.Followers[other].State == Town {
					w.clearTownFarms(other)
					g := &w.Followers[other]
					g.State = Walking
					g.Stage = 0
					g.Frame = 0
				}
			}
		}
		if claim && mutate {
			pending[pendingCount] = at
			pendingCount++
		}
		return true
	}
	count := 0
	if visit(townFootprint[0], true) {
		count++
	}
	if count == 0 {
		return 0
	}
	for _, d := range townFootprint[1:9] {
		if visit(d, true) {
			count++
		}
	}
	if count == 9 {
		for _, d := range townFootprint[9:25] {
			if visit(d, true) {
				count++
			}
		}
		if count == 25 {
			clear := true
			for _, d := range townFootprint[25:] {
				if !visit(d, false) {
					clear = false
					break
				}
			}
			if clear {
				for _, d := range townFootprint[25:] {
					visit(d, true)
				}
				count = 26
			}
		}
	}
	stage := int(supportStages[count])
	if mutate {
		// Farm writes run in reverse visit order, preserving the evaluator's
		// conflict precedence without storing addresses or register state.
		for i := pendingCount - 1; i >= 0; i-- {
			w.Farms[pending[i]] = uint8(owner + 1)
		}
		w.rebuildCells()
	}
	return stage
}

func (w *World) EvaluateTown(id int) int {
	if id <= 0 || id >= FollowerCapacity || w.Followers[id].State == Inactive || w.Followers[id].Owner > 1 {
		return 0
	}
	f := w.Followers[id]
	if f.Stage != 0 && w.Tick&3 != uint64(id)&3 && settlementLand(w.Cell(int(f.X), int(f.Y)).Code) {
		return int(f.Stage)
	}
	return w.townSupport(int(f.Owner), int(f.X), int(f.Y), id, true)
}

func (w *World) clearTownFarms(id int) {
	f := w.Followers[id]
	limit := 9
	if f.Stage >= 10 {
		limit = 25
	}
	if f.Stage == 18 {
		limit = 49
	}
	for _, d := range townFootprint[:limit] {
		x, y := int(f.X)+d[0], int(f.Y)+d[1]
		if inside(x, y) && w.Farms[x+y*MapSize] == f.Owner+1 {
			w.Farms[x+y*MapSize] = 0
			w.Tiles[x+y*MapSize].Code = w.Tiles[x+y*MapSize].Shape
		}
	}
}

func (w *World) repaintFarms() {
	w.Farms = [MapSize * MapSize]uint8{}
	for _, f := range w.Followers[1:] {
		if f.State == Town {
			limit := 9
			if f.Stage >= 10 {
				limit = 25
			}
			if f.Stage == 18 {
				limit = 49
			}
			for _, d := range townFootprint[:limit] {
				x, y := int(f.X)+d[0], int(f.Y)+d[1]
				if inside(x, y) && w.Tiles[x+y*MapSize].IsFlat() {
					w.Farms[x+y*MapSize] = f.Owner + 1
				}
			}
		}
	}
	w.rebuildCells()
}
