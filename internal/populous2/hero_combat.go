package populous2

import legacy "go-populous2/internal/legacy"

func (w *World) bindHeroCombat() {
	w.rebuildCaptiveIndex()
	w.Core.CanHeroCrossWater = func(index int) bool {
		return index >= 0 && index < len(w.Heroes) && w.Heroes[index].Active && w.Heroes[index].Spell == Helen
	}
	w.Core.HeroTargetAllowed = func(hero, target int) bool {
		if hero < 0 || hero >= len(w.Heroes) || w.Heroes[hero].Spell != Helen {
			return true
		}
		return !w.isCaptive(target)
	}
	w.Core.BeforeBattle = func(attacker, defender int) bool { return w.captureByHelen(attacker, defender) }
	w.Core.SkipFollower = func(index int) bool {
		return w.isCaptive(index)
	}
	w.Core.OnBattleWon = func(winner, loser int) {
		if winner < 0 || winner >= len(w.Core.Peeps) || !w.Heroes[winner].Active {
			return
		}
		// The inherited victory path decrements its generic knight status.
		// Native heroes retain their kind after winning a battle.
		w.Core.Peeps[winner].Status = legacy.KnightStatus
		if w.Heroes[winner].Spell == Adonis {
			w.splitAdonis(winner)
		}
	}
}

func (w *World) isCaptive(index int) bool {
	return index >= 0 && index < len(w.captiveIndex) && w.captiveIndex[index]
}

func (w *World) rebuildCaptiveIndex() {
	w.captiveIndex = [legacy.MaxPeeps]bool{}
	for _, hero := range w.Heroes {
		if !hero.Active || hero.Spell != Helen {
			continue
		}
		for _, captive := range hero.Captives {
			if captive >= 0 && captive < len(w.captiveIndex) {
				w.captiveIndex[captive] = true
			}
		}
	}
}

// captureByHelen translates contact at CODE:$12ade. A captive keeps its faith;
// the ordinary battle is replaced by abduction and a follower-chain link.
func (w *World) captureByHelen(attacker, defender int) bool {
	if attacker < 0 || defender < 0 || attacker >= len(w.Core.Peeps) || defender >= len(w.Core.Peeps) {
		return false
	}
	for _, pair := range [][2]int{{attacker, defender}, {defender, attacker}} {
		hero, victim := pair[0], pair[1]
		if !w.Heroes[hero].Active || w.Heroes[hero].Spell != Helen || w.Core.Peeps[hero].Player == w.Core.Peeps[victim].Player {
			continue
		}
		for _, h := range w.Heroes {
			for _, index := range h.Captives {
				if index == victim {
					return true
				}
			}
		}
		if !w.Core.DetachFollower(victim) {
			return false
		}
		w.Core.Peeps[hero].HeadFor = 0
		w.Heroes[hero].Captives = append(w.Heroes[hero].Captives, victim)
		w.captiveIndex[victim] = true
		return true
	}
	return false
}

func (w *World) followHelenCaptives() {
	for i := range w.Heroes {
		hero := &w.Heroes[i]
		if len(hero.Captives) == 0 {
			continue
		}
		if !hero.Active || i >= len(w.Core.Peeps) || w.Core.Peeps[i].Population <= 0 {
			hero.Captives = nil
			continue
		}
		previous := w.Core.Peeps[i].AtPos
		live := hero.Captives[:0]
		for _, index := range hero.Captives {
			if index < 0 || index >= len(w.Core.Peeps) || w.Core.Peeps[index].Population <= 0 {
				continue
			}
			p := &w.Core.Peeps[index]
			x, y := p.AtPos%64, p.AtPos/64
			if x < previous%64 {
				x++
			} else if x > previous%64 {
				x--
			}
			if y < previous/64 {
				y++
			} else if y > previous/64 {
				y--
			}
			w.Core.MoveFollowerDirect(index, x+y*64)
			previous = p.AtPos
			if w.isWaterAt(p.AtPos) {
				w.Core.DamagePeep(index, p.Population)
				continue
			}
			live = append(live, index)
		}
		hero.Captives = live
	}
	w.rebuildCaptiveIndex()
}

// splitAdonis translates CODE:$146d8 after the native battle-victory test at
// $129fa. A living hero above twenty people halves into two heroes. Recruited
// population alone must not trigger this operation.
func (w *World) splitAdonis(index int) bool {
	if index < 0 || index >= len(w.Core.Peeps) || !w.Heroes[index].Active || w.Heroes[index].Spell != Adonis || w.Core.Peeps[index].Population <= 20 {
		return false
	}
	clone := w.Core.AllocateHeroClone(index)
	if clone < 0 {
		return false
	}
	population := w.Core.Peeps[index].Population >> 1
	w.Core.Peeps[index].Population = population
	w.Core.Peeps[clone].Population = population
	w.Core.Peeps[clone].Status = legacy.KnightStatus
	w.Heroes[clone] = w.Heroes[index]
	w.Heroes[index].Population = population
	w.Heroes[clone].Population = population
	return true
}
