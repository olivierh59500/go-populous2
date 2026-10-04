package populous

func (w *World) chooseExplorerDirection(index int) int {
	player := int(w.Peeps[index].Player)
	if w.War {
		return w.moveMagnetPeeps(index)
	}
	if isHeadedPeep(w.Peeps[index]) {
		step := w.moveKnightPeep(index)
		if step == noMove && w.legacyTurn.active && w.legacyTurn.advancedPlayer == player {
			if escape, ok := w.advancedKnightEscape(index); ok {
				return escape
			}
		}
		return step
	}
	if player >= 0 && player < len(w.Magnets) && w.Magnets[player].Flags == MagnetMode {
		return w.moveMagnetPeeps(index)
	}
	return w.whereDoIGo(index)
}

// PlanWalkerStep reuses inherited target selection and waiting/settlement
// decisions without committing a position or resolving a contact. The native
// fractional controller owns admission and committed cell entry separately.
func (w *World) PlanWalkerStep(index int) (int, bool) {
	if !w.validPeep(index) || !inMap(w.Peeps[index].AtPos) {
		return 0, false
	}
	delta := w.chooseExplorerDirection(index)
	p := &w.Peeps[index]
	if delta == noMove || delta == 0 && (isHeadedPeep(*p) || w.War) {
		p.Flags |= IAmWaiting
		p.BattlePopulation = 7
		return 0, false
	}
	p.Flags &^= IAmWaiting
	if delta == 0 {
		if w.MapWho[p.AtPos] == 0 {
			w.MapWho[p.AtPos] = uint16(index + 1)
		}
		p.Flags = InTown
		p.BattlePopulation = w.GameTurn
		p.Frame, p.Direction = 0, 0
		w.setFrame(index)
		w.setTown(index, false)
		return 0, false
	}
	return delta, true
}

// CommitWalkerEntry runs only after the native high coordinate bytes change.
// Contact and single-head occupancy are inherited adapters; target decisions
// never call this function, so they cannot duplicate a merger or battle.
func (w *World) CommitWalkerEntry(index, position int) bool {
	if !w.validPeep(index) || !inMap(position) {
		return false
	}
	p := &w.Peeps[index]
	old := p.AtPos
	id := uint16(index + 1)
	if inMap(old) && w.MapWho[old] == id {
		w.MapWho[old] = 0
	}
	p.AtPos, p.Direction = position, position-old
	occupant := int(w.MapWho[position]) - 1
	if occupant >= 0 && occupant != index {
		if !w.resolveContact(index, occupant) {
			return false
		}
	}
	if w.validPeep(index) && w.MapWho[position] == 0 {
		w.MapWho[position] = id
	}
	return w.validPeep(index)
}

func (w *World) notifyFollowerAllocated(index int) {
	if w.OnFollowerAllocated != nil {
		w.OnFollowerAllocated(index)
	}
}
