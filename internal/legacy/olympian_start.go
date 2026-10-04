package populous

// InitialFollowers contains the template fields copied by CODE:$10cbe.
type InitialFollowers struct {
	Groups, Population, SearchIndex, Weapons int
	Speed                                    uint8
}

// PlaceOlympianPeople translates the two directional scans at $10b38.
// It keeps the existing terrain generator but replaces the first game's
// starting group counts, strength, mana and scan origins.
func (w *World) PlaceOlympianPeople(sides [2]InitialFollowers) {
	w.Peeps = w.Peeps[:0]
	w.MapWho = [MapWidth * MapHeight]uint16{}
	w.MapSteps = [MapWidth * MapHeight]uint16{}
	for player, side := range sides {
		w.Magnets[player] = Magnet{GoTo: 32 + 32*MapWidth, Flags: SettleMode}
		count := 0
		place := func(pos int) {
			if count >= side.Groups || len(w.Peeps) >= MaxFollowers {
				return
			}
			w.Peeps = append(w.Peeps, Peep{Flags: OnMove, Player: byte(player), Population: side.Population, IQ: side.SearchIndex, Weapons: side.Weapons, MovementSpeed: side.Speed, AtPos: pos})
			w.notifyFollowerAllocated(len(w.Peeps) - 1)
			w.MapWho[pos] = uint16(len(w.Peeps))
			if count == 0 {
				w.Magnets[player].Carried = len(w.Peeps)
				w.Magnets[player].GoTo = pos
			}
			count++
		}
		for pass := 0; pass < 2 && count < side.Groups; pass++ {
			for n := 1; n < MapWidth*MapHeight && count < side.Groups; n++ {
				pos := n
				if player == 1 {
					pos = MapWidth*MapHeight - n
				}
				if pass == 0 && w.MapBlk[pos] != FlatBlock {
					continue
				}
				if pass == 1 && w.MapBlk[pos] == WaterBlock {
					continue
				}
				place(pos)
			}
		}
	}
}
