package populous

// DirectRaiseTerrain exposes the unpriced propagation primitive used by the
// second game's native hero/environment handlers, independently of commands.
func (w *World) DirectRaiseTerrain(x, y int) bool {
	return w.forceRaiseAt(x, y)
}

func (w *World) NextRandom() int { return w.rng.next() }

// DirectLowerTerrain is the unpriced native effect operation. It bypasses
// player admission and Armageddon's ordinary input gate without changing War.
func (w *World) DirectLowerTerrain(x, y int) bool {
	if x < 0 || y < 0 || x > MapWidth || y > MapHeight {
		return false
	}
	bounds := newAltBounds(x, y)
	w.lowerPointTracked(x, y, bounds)
	if bounds.changed == 0 {
		return false
	}
	w.rebuildAltitudeBounds(bounds)
	return true
}

// ReformOlympianTown reconnects the translated support/stage calculation with
// the current farm compositor after a native effect releases a surviving town.
// Original $13352 farm composition remains a separate translation target.
func (w *World) ReformOlympianTown(index int) bool {
	if !w.validPeep(index) || w.OlympianTowns == nil {
		return false
	}
	p := &w.Peeps[index]
	stage := w.OlympianTownStage(int(p.Player), p.AtPos)
	p.TownStage = stage
	p.Flags, p.Frame = InTown, FirstTown+stage*10/18
	if stage == 0 {
		p.Flags, p.Frame = OnMove, 0
		return false
	}
	w.setTown(index, false)
	return true
}

// DetachFollower removes a group from town/combat bookkeeping without changing
// its allegiance or population, for native abduction and actor state changes.
func (w *World) DetachFollower(index int) bool {
	if !w.validPeep(index) {
		return false
	}
	if w.Peeps[index].Flags&InTown != 0 {
		w.setTown(index, true)
	}
	p := &w.Peeps[index]
	p.Flags = OnMove
	p.Frame = 0
	p.Direction = 0
	p.HeadFor = 0
	p.BattlePopulation = 0
	return true
}

func (w *World) MoveFollowerDirect(index, pos int) bool {
	if !w.validPeep(index) || !inMap(pos) {
		return false
	}
	w.clearPeepMapRefs(index)
	w.Peeps[index].AtPos = pos
	if w.OnFollowerMoved != nil {
		w.OnFollowerMoved(index)
	}
	if w.MapWho[pos] == 0 {
		w.MapWho[pos] = uint16(index + 1)
	}
	return true
}

func (w *World) ReserveDeathOccupancy(index int) {
	if index < 0 || index >= len(w.Peeps) || !inMap(w.Peeps[index].AtPos) {
		return
	}
	pos := w.Peeps[index].AtPos
	if w.MapWho[pos] == 0 {
		w.MapWho[pos] = uint16(index + 1)
	}
}

func (w *World) ReleaseDeathOccupancy(index int) { w.clearPeepMapRefs(index) }

// The small hooks in this file are additions for the Populous II prototype.
// The underlying engine remains the supplied Populous 1 conversion.

func (w *World) DamagePeep(index, amount int) bool {
	if !w.validPeep(index) || amount <= 0 {
		return false
	}
	p := &w.Peeps[index]
	p.Population -= amount
	if p.Population <= 0 {
		w.zeroPopulation(index)
	}
	return true
}

func (w *World) ConvertPeep(index, player int) bool {
	if !w.validPeep(index) || player < 0 || player > 1 || int(w.Peeps[index].Player) == player {
		return false
	}
	p := &w.Peeps[index]
	old := int(p.Player)
	if w.Magnets[old].Carried == index+1 {
		w.Magnets[old].Carried = 0
	}
	if p.Flags&InTown != 0 {
		w.setTown(index, true)
		p.Flags = OnMove
		p.Frame = 0
	}
	p.Player = byte(player)
	return true
}

func (w *World) SprogAt(player, pos int) bool {
	for i := range w.Peeps {
		p := &w.Peeps[i]
		if int(p.Player) == player && p.AtPos == pos && p.Population > 10 && p.Flags == InTown {
			if w.OlympianTowns != nil {
				p.ForceEmigration = true
				return true
			}
			before := p.Population
			w.spawnWalkerFromTown(i, min(before-1, w.checkLife(player, pos)))
			return w.Peeps[i].Population < before
		}
	}
	return false
}

// PromoteHero detaches a living carrier without charging legacy knight mana or
// granting legacy scores, sounds, weapons, or automatic attack targets. The
// caller applies the independently decoded Populous II hero creation rules.
func (w *World) PromoteHero(index int) bool {
	if !w.validPeep(index) || w.Peeps[index].Player > 1 || !inMap(w.Peeps[index].AtPos) {
		return false
	}
	if w.Peeps[index].Flags&InTown != 0 {
		w.setTown(index, true)
	}
	p := &w.Peeps[index]
	p.Flags &^= InTown | WaitForMe | IAmWaiting | InBattle | InEffect | InRuin
	p.Flags |= OnMove
	p.Status = KnightStatus
	p.HeadFor = 0
	p.BattlePopulation = 0
	p.Frame = 0
	p.Direction = 0
	if w.Magnets[p.Player].Carried == index+1 {
		w.Magnets[p.Player].Carried = 0
	}
	return true
}

// AllocateHeroClone reserves the first unused native follower record and copies
// a live actor without retaining battle or carrier links. Native record zero is
// reserved, so Go index n represents the native one-based record n+1. The caller
// owns the hero-specific clone rules and its separate hero metadata.
func (w *World) AllocateHeroClone(index int) int {
	if !w.validPeep(index) || w.Peeps[index].Player > 1 || !inMap(w.Peeps[index].AtPos) {
		return -1
	}
	slot := -1
	for i := range w.Peeps {
		if w.Peeps[i].Population <= 0 && (w.FollowerReserved == nil || !w.FollowerReserved(i)) {
			slot = i
			break
		}
	}
	if slot < 0 {
		if len(w.Peeps) >= MaxFollowers {
			return -1
		}
		slot = len(w.Peeps)
	}
	clone := w.Peeps[index]
	clone.Flags = OnMove
	clone.HeadFor = 0
	clone.BattlePopulation = 0
	clone.Frame = 0
	clone.Direction = 0
	clone.InOut = clone.AtPos
	if slot == len(w.Peeps) {
		w.Peeps = append(w.Peeps, clone)
	} else {
		w.clearPeepMapRefs(slot)
		for player := range w.Magnets {
			if w.Magnets[player].Carried == slot+1 {
				w.Magnets[player].Carried = 0
			}
		}
		w.Peeps[slot] = clone
	}
	w.notifyFollowerAllocated(slot)
	if w.MapWho[clone.AtPos] == 0 {
		w.MapWho[clone.AtPos] = uint16(slot + 1)
	}
	return slot
}
