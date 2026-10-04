package populous2

import legacy "go-populous2/internal/legacy"

type FlameDeath struct{ Follower, X, Y, Animation, End int }

func (w *World) bindFlameDeaths() {
	w.rebuildFlameDeathIndex()
	w.Core.FollowerReserved = func(index int) bool {
		return index >= 0 && index < len(w.flameDeathIndex) && w.flameDeathIndex[index]
	}
}

func (w *World) rebuildFlameDeathIndex() {
	w.flameDeathIndex = [legacy.MaxPeeps]bool{}
	for _, death := range w.FlameDeaths {
		if death.Follower >= 0 && death.Follower < len(w.flameDeathIndex) {
			w.flameDeathIndex[death.Follower] = true
		}
	}
}

func (w *World) tickFlameDeaths() {
	live := w.FlameDeaths[:0]
	for _, death := range w.FlameDeaths {
		death.Animation += 4
		if death.Animation < death.End {
			live = append(live, death)
		} else {
			w.Core.ReleaseDeathOccupancy(death.Follower)
		}
	}
	w.FlameDeaths = live
	w.rebuildFlameDeathIndex()
}

// burnFireCell follows $1735a/$16542: scorch only suitable flat ground and
// damage actors on the current cell, regardless of their allegiance.
func (w *World) burnFireCell(x, y int) {
	w.burnActorsAt(x, y, false, true)
}

func (w *World) spreadTreeFire(x, y int) {
	for _, d := range [4][2]int{{0, -1}, {0, 1}, {1, 0}, {-1, 0}} {
		xx, yy := x+d[0], y+d[1]
		if inside(xx, yy) {
			w.burnActorsAt(xx, yy, true, false)
		}
	}
}

func (w *World) burnActorsAt(x, y int, treeSpread, scorch bool) {
	pos := x + y*64
	cell := w.TerrainCell(x, y)
	if scorch && cell.Shape == 15 && w.GroundRules.Properties[cell.Code]&0x400 == 0 {
		w.Marks[pos] = Mark{Spell: FireColumn, Life: 1, Persistent: true, NativeTile: 95}
	}
	for i := range w.Scenery {
		a := &w.Scenery[i]
		if a.Active && a.Kind == SceneryTree && !a.Removing && a.X == x && a.Y == y {
			a.Removing = true
			a.Animation = 0xf10
			a.Frame = 0
		}
	}
	for i, p := range w.Core.Peeps {
		if p.Population <= 0 || p.AtPos != pos {
			continue
		}
		if p.Flags&legacy.InTown == 0 {
			if treeSpread && w.Heroes[i].Active {
				continue
			}
			if w.Core.Magnets[p.Player].Carried == i+1 && w.Core.Magnets[p.Player].Flags == legacy.MagnetMode {
				continue
			}
			if w.Heroes[i].Active && w.FireColumns.HeroDeath[heroIndex(w.Heroes[i].Spell)] == 0 {
				continue
			}
		}
		animation := 0x178
		if p.Flags&legacy.InTown != 0 {
			stage := clamp(p.TownStage, 0, TownStages-1)
			animation = w.FireColumns.TownDeath[stage]
			var burnt []int
			if w.Core.OlympianTowns != nil {
				limit := int(w.FireColumns.TownClearCount[stage])
				for _, d := range w.Core.OlympianTowns.Footprint[:limit] {
					xx, yy := x+d[0], y+d[1]
					if inside(xx, yy) && w.nativeTileAt(xx, yy) == uint8(47+16*int(p.Player)) {
						burnt = append(burnt, xx+yy*64)
					}
				}
			}
			w.Core.DetachFollower(i)
			for _, tile := range burnt {
				w.Marks[tile] = Mark{Spell: FireColumn, Life: 1, Persistent: true, NativeTile: 95}
			}
		} else if w.Heroes[i].Active {
			animation = w.FireColumns.HeroDeath[heroIndex(w.Heroes[i].Spell)]
		}
		if animation != 0 && len(w.FlameDeaths) < legacy.MaxFollowers {
			w.FlameDeaths = append(w.FlameDeaths, FlameDeath{Follower: i, X: x, Y: y, Animation: animation, End: animation + w.FireColumns.SequenceLengths[animation]*4})
			w.flameDeathIndex[i] = true
		}
		w.Core.DamagePeep(i, p.Population)
		w.Core.ReserveDeathOccupancy(i)
	}
}
