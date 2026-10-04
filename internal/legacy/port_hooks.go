package populous

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
		if w.Peeps[i].Population <= 0 {
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
	if w.MapWho[clone.AtPos] == 0 {
		w.MapWho[clone.AtPos] = uint16(slot + 1)
	}
	return slot
}
