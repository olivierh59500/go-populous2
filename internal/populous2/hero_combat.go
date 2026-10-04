package populous2

import legacy "go-populous2/internal/legacy"

func (w *World) bindHeroCombat() {
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
