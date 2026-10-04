package populous2

import legacy "go-populous2/internal/legacy"

// castPlague translates the actor selection at CODE:$1730e. The disease lives
// on the follower, rather than on an expiring circle of neighboring ground.
func (w *World) castPlague(player, pos int) bool {
	applied := false
	for i := range w.Core.Peeps {
		p := &w.Core.Peeps[i]
		if p.Population > 0 && int(p.Player) != player && p.AtPos == pos && p.Flags&legacy.InRuin == 0 {
			p.Plague = true
			applied = true
		}
	}
	return applied
}

func (w *World) spreadPlague() {
	var infected [4096]bool
	for _, p := range w.Core.Peeps {
		if p.Population > 0 && p.Plague && p.AtPos >= 0 && p.AtPos < len(infected) {
			infected[p.AtPos] = true
		}
	}
	for i := range w.Core.Peeps {
		p := &w.Core.Peeps[i]
		if p.Population > 0 && p.AtPos >= 0 && p.AtPos < len(infected) && infected[p.AtPos] {
			p.Plague = true
		}
	}
}

func (w *World) removePlagueVictims() {
	for i, p := range w.Core.Peeps {
		if p.Plague && p.Population > 0 {
			w.Core.DamagePeep(i, p.Population)
		}
	}
}
