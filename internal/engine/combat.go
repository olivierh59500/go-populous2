package engine

const combatPopulationDivisor = 100

func (w *World) beginBattle(attacker, defender int) {
	if !w.Followers[attacker].IsHero() && w.Followers[defender].IsHero() {
		attacker, defender = defender, attacker
	}
	w.clearHeroClaim(attacker)
	a, d := &w.Followers[attacker], &w.Followers[defender]
	a.BattleWasTown = a.State == Town
	d.BattleWasTown = d.State == Town
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
	f, l := &w.Followers[winner], &w.Followers[loser]
	owner, enemy := int(f.Owner), int(l.Owner)
	reward := w.battleReward(loser)
	if owner < 2 {
		w.Players[owner].BattlesWon++
		w.Players[owner].Mana = int(uint32(w.Players[owner].Mana) + uint32(reward))
	}
	if enemy < 2 {
		remaining := uint32(0)
		if int32(uint32(w.Players[enemy].Mana)) > int32(reward) {
			remaining = uint32(w.Players[enemy].Mana) - uint32(reward)
		}
		w.Players[enemy].Mana = int(remaining)
		if w.Players[enemy].Leader == loser {
			w.Players[enemy].Statistics.LeaderLosses++
			w.Players[enemy].Leader = 0
		}
	}
	w.clearHeroLinks(loser)
	l.Population = 0
	l.State = Ruin
	l.moving = false
	l.BattleWith = 0
	l.BattleAggressor = false
	l.Frame = 0
	frames := uint16(12)
	kind := CombatDefeated
	if l.IsHero() {
		frames = 6
		kind = CombatHeroDefeated
	}
	l.CombatAftermath = CombatAftermathState{Kind: kind, Frames: frames}
	f.BattleWith = 0
	f.BattleAggressor = false
	f.Frame = 0
	if f.IsHero() {
		if l.BattleWasTown {
			w.clearTownFarms(loser)
		}
		f.State = Walking
		f.Hero.Phase = HeroFindTarget
		if f.Hero.Kind == HeroAdonis && !w.BirthBlocked {
			w.SplitAdonis(winner)
		}
	} else if l.BattleWasTown || f.BattleWasTown {
		if l.BattleWasTown {
			w.clearTownFarms(loser)
		}
		if f.BattleWasTown {
			w.clearTownFarms(winner)
		}
		f.State = Town
		f.Stage = 0
		f.LastDevelopedStage = 0
		f.FoundedAt = w.Tick
		f.positionX = int(f.X)*256 + 128
		f.positionY = int(f.Y)*256 + 128
		f.positionSet = true
		w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(winner)}, f.positionX, f.positionY)
		if stage := w.EvaluateTown(winner); stage > 0 {
			f.Stage = uint8(stage)
			f.Frame = uint16(stage)
		} else {
			f.State = Walking
			f.Stage = 0
		}
	} else {
		f.State = Walking
		f.CombatAftermath = CombatAftermathState{Kind: CombatVictorious, Frames: 14}
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

type CombatAftermathKind uint8

const (
	CombatAftermathNone CombatAftermathKind = iota
	CombatDefeated
	CombatHeroDefeated
	CombatVictorious
)

type CombatAftermathState struct {
	Kind          CombatAftermathKind
	Frame, Frames uint16
}

func (w *World) battleReward(loser int) uint16 {
	f := w.Followers[loser]
	reward := uint16(w.Landscape.Parameters[0])
	next := 1
	if f.Owner < 2 && w.Players[f.Owner].Leader == loser {
		reward += uint16(w.Landscape.Parameters[next])
		next++
	}
	if f.IsHero() {
		reward += uint16(w.Landscape.Parameters[next])
	}
	if f.BattleWasTown {
		reward += uint16(w.Landscape.Weapons[min(TownStages-1, int(f.Stage))])
	}
	return reward
}

func (w *World) advanceCombatAftermath(id int) bool {
	f := &w.Followers[id]
	a := &f.CombatAftermath
	if a.Kind == CombatAftermathNone {
		return false
	}
	a.Frame++
	f.Frame = a.Frame
	if a.Frame < a.Frames {
		return true
	}
	kind := a.Kind
	f.CombatAftermath = CombatAftermathState{}
	if kind == CombatVictorious {
		f.State = Walking
		f.Frame = 0
		f.moving = false
	} else {
		w.remove(id)
	}
	return true
}
