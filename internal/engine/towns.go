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

// TownStage evaluates the three settlement support rings. An adjacent town
// cannot claim the same farmland; a complete outer ring unlocks stage eighteen.
func (w *World) TownStage(owner, x, y, ignoreID int) int {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return 0
	}
	support := func(d [2]int) bool {
		nx, ny := x+d[0], y+d[1]
		if !inside(nx, ny) {
			return false
		}
		at := nx + ny*MapSize
		if !w.NatureTownAllowed(owner, nx, ny) {
			return false
		}
		if !w.Tiles[at].IsFlat() || w.Tiles[at].Corners[0] != w.Tiles[x+y*MapSize].Corners[0] {
			return false
		}
		other := int(w.Occupants[at])
		if other != 0 && other != ignoreID && w.Followers[other].State == Town {
			return false
		}
		if w.Farms[at] != 0 && w.Farms[at] != uint8(owner+1) {
			return false
		}
		return true
	}
	if !support(townFootprint[0]) {
		return 0
	}
	// Separate settlements need clearance, rather than endlessly founding and
	// abandoning huts on land already supporting a neighbouring settlement.
	for _, d := range townFootprint[1:25] {
		nx, ny := x+d[0], y+d[1]
		if !inside(nx, ny) {
			continue
		}
		other := int(w.Occupants[nx+ny*MapSize])
		if other != 0 && other != ignoreID && w.Followers[other].State == Town {
			return 0
		}
	}
	count := 1
	for _, d := range townFootprint[1:9] {
		if support(d) {
			count++
		}
	}
	if count == 9 {
		for _, d := range townFootprint[9:25] {
			if support(d) {
				count++
			}
		}
		if count == 25 {
			clear := true
			for _, d := range townFootprint[25:] {
				if !support(d) {
					clear = false
					break
				}
			}
			if clear {
				count = 26
			}
		}
	}
	return int(supportStages[count])
}

func (w *World) repaintFarms() {
	w.Farms = [MapSize * MapSize]uint8{}
	for _, f := range w.Followers[1:] {
		if f.State == Town {
			limit := 9
			if f.Stage >= 10 {
				limit = 25
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

// computerLand develops existing towns with a stable target elevation. The
// terrain target is recorded once per settlement neighbourhood, preventing
// the wasteful raise/lower oscillation between neighbouring elevations.
func (w *World) computerLand(owner int) {
	if w.Players[owner].Mana < w.PowerCost(owner, RaiseLower) {
		return
	}
	for id := 1; id < len(w.Followers); id++ {
		f := w.Followers[id]
		if f.State != Town || int(f.Owner) != owner {
			continue
		}
		at := int(f.X) + int(f.Y)*MapSize
		if !w.terrainTargetSet[at] {
			w.terrainTargets[at] = w.Tiles[at].Corners[0]
			w.terrainTargetSet[at] = true
		}
		target := w.terrainTargets[at]
		for _, d := range townFootprint[:25] {
			x, y := int(f.X)+d[0], int(f.Y)+d[1]
			if !inside(x, y) {
				continue
			}
			for _, corner := range [4][2]int{{x, y}, {x + 1, y}, {x + 1, y + 1}, {x, y + 1}} {
				vertex := corner[0] + corner[1]*CornerSize
				if w.developmentTargetSet[vertex] && w.developmentTarget[vertex] != target {
					continue
				}
				w.developmentTargetSet[vertex] = true
				w.developmentTarget[vertex] = target
				h := w.Heights[vertex]
				if h == target {
					continue
				}
				// Do not lower the foundation of a different town or propagate through
				// it; the rejected order costs nothing and another site is considered.
				protected := false
				for other := 1; other < len(w.Followers); other++ {
					g := w.Followers[other]
					if other == id || g.State != Town {
						continue
					}
					if abs(corner[0]-int(g.X)) <= 2 && abs(corner[1]-int(g.Y)) <= 2 {
						protected = true
						break
					}
				}
				if protected {
					continue
				}
				if !w.developmentEditAllowed(corner[0], corner[1], h < target) {
					continue
				}
				if h < target {
					if w.RaiseAt(owner, corner[0], corner[1]) {
						return
					}
				} else {
					if w.LowerAt(owner, corner[0], corner[1]) {
						return
					}
				}
			}
		}
	}
}

// developmentEditAllowed checks recursive propagation on a scratch height map.
// A neighbouring settlement's terrain reservation is honoured even when the
// requested corner itself lies outside that settlement's immediate footprint.
func (w *World) developmentEditAllowed(x, y int, raise bool) bool {
	before := w.Heights
	if raise {
		w.raiseCorner(x, y)
	} else {
		w.lowerCorner(x, y)
	}
	allowed := true
	for at, height := range w.Heights {
		if height == before[at] {
			continue
		}
		if w.developmentTargetSet[at] {
			target := int(w.developmentTarget[at])
			if abs(int(height)-target) > abs(int(before[at])-target) {
				allowed = false
				break
			}
		}
	}
	w.Heights = before
	return allowed
}
