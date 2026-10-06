package engine

var followerSearchCounts = [19]int{8, 8, 8, 8, 8, 24, 24, 24, 24, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48}
var preferredSettlements = [48][2]int{
	{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1},
	{-2, -2}, {-1, -2}, {0, -2}, {1, -2}, {2, -2}, {-2, -1}, {2, -1}, {-2, 0}, {2, 0}, {-2, 1}, {2, 1}, {-2, 2}, {-1, 2}, {0, 2}, {1, 2}, {2, 2},
	{-3, -3}, {-2, -3}, {-1, -3}, {0, -3}, {1, -3}, {2, -3}, {3, -3}, {-3, -2}, {3, -2}, {-3, -1}, {3, -2}, {-3, 0}, {3, 0}, {-3, 1}, {3, 1}, {-3, 2}, {3, 2}, {-3, 3}, {-2, 3}, {-1, 3}, {0, 3}, {1, 3}, {2, 3}, {3, 3},
}
var decisionNeighbours = [16][2]int{
	{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1},
	{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1},
}

// chooseMove keeps the original ordered settlement search and randomized
// low-pressure fallback. A larger town gives its emigrants a wider search;
// the AI does not substitute a weighted global pathfinder for this rule.
func (w *World) chooseMove(id int) (int, int, bool) {
	f := &w.Followers[id]
	owner := int(f.Owner)
	mode := w.Players[owner].Mode
	if mode == Rally {
		return w.chooseRally(id)
	}
	count := followerSearchCounts[min(18, max(0, f.Search/2))]
	selectedX, selectedY := 0, 0
	selected := false
	for _, d := range preferredSettlements[:count] {
		x, y := int(f.X)+d[0], int(f.Y)+d[1]
		if !inside(x, y) || w.Nature.BlocksWalking(x, y) {
			continue
		}
		other := int(w.Occupants[x+y*MapSize])
		if other != 0 && other != id {
			candidate := w.Followers[other]
			matches := mode == Join && candidate.Owner == f.Owner && candidate.State != Town || mode == Fight && candidate.Owner != f.Owner
			if matches {
				return x, y, true
			}
		}
		if w.Cell(x, y).IsFlat() && w.NatureTownAllowed(owner, x, y) {
			selectedX, selectedY, selected = x, y, true
			if mode == Settle {
				break
			}
		}
	}
	if selected {
		return selectedX, selectedY, true
	}
	bits := w.random.next()
	start := int(bits&14) / 2
	pressure := uint8(255)
	for offset := 0; offset < 8; offset++ {
		d := decisionNeighbours[start+offset]
		x, y := int(f.X)+d[0], int(f.Y)+d[1]
		if !inside(x, y) || w.Cell(x, y).IsWater() || w.Nature.BlocksWalking(x, y) {
			continue
		}
		candidate := w.Pressure[x+y*MapSize]
		if candidate > pressure || candidate == pressure && bits&(1<<uint(7-offset)) == 0 {
			continue
		}
		pressure, selectedX, selectedY, selected = candidate, x, y, true
	}
	return selectedX, selectedY, selected
}

func (w *World) chooseRally(id int) (int, int, bool) {
	f := w.Followers[id]
	p := w.Players[f.Owner]
	tx, ty := p.RallyX, p.RallyY
	if p.Leader != 0 && p.Leader != id && w.Followers[p.Leader].State != Inactive {
		tx, ty = int(w.Followers[p.Leader].X), int(w.Followers[p.Leader].Y)
	}
	dx, dy := sign(tx-int(f.X)), sign(ty-int(f.Y))
	if dx == 0 && dy == 0 {
		return 0, 0, false
	}
	x, y := int(f.X)+dx, int(f.Y)+dy
	if inside(x, y) && !w.Cell(x, y).IsWater() && !w.Nature.BlocksWalking(x, y) {
		return x, y, true
	}
	// Try the source's immediate eight neighbours when direct movement is
	// blocked; footsteps do not alter the preferred homing direction.
	for _, d := range decisionNeighbours[:8] {
		x, y = int(f.X)+d[0], int(f.Y)+d[1]
		if inside(x, y) && !w.Cell(x, y).IsWater() && !w.Nature.BlocksWalking(x, y) {
			return x, y, true
		}
	}
	return 0, 0, false
}
