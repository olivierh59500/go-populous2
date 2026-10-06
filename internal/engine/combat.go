package engine

const combatPopulationDivisor = 100

func (w *World) beginBattle(attacker, defender int) {
	if !w.Followers[attacker].IsHero() && w.Followers[defender].IsHero() {
		attacker, defender = defender, attacker
	}
	w.clearHeroLinks(attacker)
	a, d := &w.Followers[attacker], &w.Followers[defender]
	a.State, d.State = Fighting, Fighting
	a.BattleWith, d.BattleWith = defender, attacker
	a.BattleAggressor, d.BattleAggressor = true, false
	a.moving, d.moving = false, false
	a.Frame, d.Frame = 0, 0
}

// stepBattle preserves the original asymmetric damage calculation: both
// sides use the aggressor's population quotient, with the opposing weapon
// strength and a ten-person minimum. Only the aggressor performs the update.
func (w *World) stepBattle(id int) {
	a := &w.Followers[id]
	if !a.BattleAggressor {
		return
	}
	if a.BattleWith <= 0 || a.BattleWith >= FollowerCapacity {
		a.State = Walking
		a.BattleWith = 0
		return
	}
	enemy := a.BattleWith
	d := &w.Followers[enemy]
	if d.State == Inactive || d.Owner == a.Owner || d.BattleWith != id {
		a.State = Walking
		a.BattleWith = 0
		return
	}
	w.random.next()
	quotient := combatQuotient(a.Population)
	d.Population -= a.Weapons*quotient + 10
	a.Population -= d.Weapons*quotient + 10
	a.Frame = (a.Frame + 1) % 4
	d.Frame = a.Frame
	if a.Population <= 0 && d.Population <= 0 {
		w.remove(id)
		w.remove(enemy)
		return
	}
	if a.Population <= 0 {
		w.finishBattle(enemy, id)
		return
	}
	if d.Population <= 0 {
		w.finishBattle(id, enemy)
	}
}
func (w *World) finishBattle(winner, loser int) {
	owner := w.Followers[winner].Owner
	w.Players[owner].BattlesWon++
	w.remove(loser)
	f := &w.Followers[winner]
	f.State = Walking
	f.BattleWith = 0
	f.BattleAggressor = false
	f.Frame = 0
	if f.IsHero() {
		f.Hero.Phase = HeroFindTarget
	}
}

// combatQuotient preserves the original word-sized quotient used by battle
// strength. Oversized populations retain their low word instead of wrapping
// the quotient; ordinary campaign populations stay well below that boundary.
func combatQuotient(population int) int {
	quotient := uint32(population) / combatPopulationDivisor
	if quotient > 65535 {
		return int(uint16(population))
	}
	return int(quotient)
}
