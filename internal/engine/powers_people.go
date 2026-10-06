package engine

import "fmt"

// DiseaseState stores the plague's separate artwork clock. The shipped
// game's disease damage is zero; ordinary positive populations are unchanged.
type DiseaseState struct {
	Infected   bool
	Frame      uint16
	Dying      bool
	DeathFrame uint16
}

// CastPlague affects opposing allocated followers at the target parcel.
// Recasting restarts their overlay; it neither spends RNG nor spreads simply
// because unrelated friendly and enemy groups stand near one another.
func (w *World) CastPlague(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid plague target")
	}
	applied := false
	for id := 1; id < FollowerCapacity; id++ {
		f := &w.Followers[id]
		if f.State == Inactive || f.Owner == uint8(owner) || int(f.X) != x || int(f.Y) != y {
			continue
		}
		if f.State == Ruin || f.State == Airborne {
			continue
		}
		f.Disease.Infected = true
		f.Disease.Frame = 0
		applied = true
	}
	if !applied {
		return fmt.Errorf("plague found no opposing group")
	}
	return nil
}

// tickDisease runs before terrain hazards and returns true for a retained
// plague death. A live group's overlay is independent of its walking clock.
func (w *World) tickDisease(id int) bool {
	if id <= 0 || id >= FollowerCapacity {
		return false
	}
	f := &w.Followers[id]
	if !f.Disease.Infected || f.State == Inactive {
		return false
	}
	f.Disease.Frame = (f.Disease.Frame + 1) % 22
	if f.Disease.Dying {
		f.Disease.DeathFrame++
		if f.Disease.DeathFrame >= 2 {
			w.remove(id)
		}
		return true
	}
	if f.Population > 0 {
		return false
	}
	f.Disease.Dying = true
	f.Disease.DeathFrame = 0
	f.State = Ruin
	if w.Players[f.Owner].Leader == id {
		w.Players[f.Owner].Leader = 0
	}
	w.repaintFarms()
	return true
}

func (w *World) InheritDiseaseMerge(source, target int) {
	if source <= 0 || source >= FollowerCapacity || target <= 0 || target >= FollowerCapacity {
		return
	}
	from := w.Followers[source].Disease
	if from.Infected {
		w.Followers[target].Disease = from
	}
}

// CastArmageddon converts healthy eligible groups to one of the first four
// heroes. It enables heroes to raise terrain directly; no actor is teleported
// and no invented end-of-game countdown is started.
func (w *World) CastArmageddon(owner int) error {
	if owner < 0 || owner > 1 {
		return fmt.Errorf("invalid Armageddon player")
	}
	if w.Armageddon {
		return nil
	}
	for id := 1; id < FollowerCapacity; id++ {
		f := &w.Followers[id]
		if f.State == Inactive || f.State == Ruin || f.State == Airborne || f.Disease.Dying {
			continue
		}
		// Retained terrain/fire deaths and suspended lightning victims belong
		// to their own states and are not ordinary convertible groups.
		if w.Nature.Deaths[id] != NatureAlive || w.FireDamage.Deaths[id].Mode != FireVictimAlive || w.AirVictims[id].Phase != LightningVictimNone {
			continue
		}
		if f.Disease.Infected {
			w.remove(id)
			continue
		}
		kind := HeroKind(1 + w.random.next()%4)
		if err := w.ConvertHero(id, kind); err != nil {
			return err
		}
	}
	w.Armageddon = true
	return nil
}
