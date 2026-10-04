package populous2

// bindFollowerHazards runs the fungus transition before a live follower's
// ordinary state dispatch, as in $112c0/$12c3c. Other native terrain hazards
// remain separate translations; no second movement pass is introduced.
func (w *World) bindFollowerHazards() {
	w.Core.BeforeFollower = func(index int) bool {
		p := w.Core.Peeps[index]
		properties := w.GroundRules.Properties[w.nativeTileAt(p.AtPos%64, p.AtPos/64)]
		hero := w.Heroes[index]
		decision := w.FungusHazards.Enter(properties, hero.Active, heroIndex(hero.Spell))
		if !decision.Applies {
			return true
		}
		if w.flameDeathIndex[index] {
			return false
		}
		w.moveActor(NativeFollowerPool, index, uint16((p.AtPos%64)*256+int(decision.CenterFraction)), uint16((p.AtPos/64)*256+int(decision.CenterFraction)))
		w.LightningVictims[index].Active = false
		w.FlameDeaths = append(w.FlameDeaths, FlameDeath{Follower: index, X: p.AtPos % 64, Y: p.AtPos / 64, Animation: decision.Animation, End: decision.Animation + w.FungusHazards.SequenceLengths[decision.Animation]*4, Kind: decision.Kind, State: decision.State})
		w.flameDeathIndex[index] = true
		w.Core.DetachFollower(index)
		w.Core.DamagePeep(index, p.Population)
		w.Core.ReserveDeathOccupancy(index)
		w.HazardSerial++
		w.LastHazardCue = decision.SoundCue
		return false
	}
}
