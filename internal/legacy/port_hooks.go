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
