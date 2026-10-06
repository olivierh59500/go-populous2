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

type ConversionState struct {
	Active      bool
	Frame       uint16
	SourceOwner uint8
	Hero        bool
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
	w.PrepareFollowerDeath(id)
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
		w.Followers[target].Disease.Infected = true
		w.Followers[target].Disease.Frame = from.Frame
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
		if f.Owner > 1 || f.Neutral.Kind != NeutralNone || f.State == Inactive || f.State == Ruin || f.State == Airborne || f.Disease.Dying {
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

// Baptism uses the water experience tier to extend both its sampling divisor
// and attempt count. It replaces only unoccupied fertile or road parcels.
func (w *World) CastBaptism(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid baptism target")
	}
	base := 17 + int(w.Players[owner].Experience[Water]>>5)
	count := int(w.random.next())%base + base/2
	for attempt := 0; attempt <= count; attempt++ {
		nx, ny, ok := w.natureSample(x, y)
		if !ok {
			continue
		}
		at := nx + ny*MapSize
		if w.Occupants[at] != 0 || w.Nature.sceneryAt(nx, ny) >= 0 || w.WallAt(nx, ny) >= 0 {
			continue
		}
		code := w.Cell(nx, ny).Code
		if !fertileNatureCode(code) && !roadCode(code) && !(code >= 220 && code <= 223) {
			continue
		}
		w.paintNature(at, GroundParcel{Mark: GroundBaptism, Owner: uint8(owner)})
	}
	return nil
}

// advanceConversion owns the retained font animation and owner flip. Fonts
// convert either side, independently of the caster that originally placed it.
func (w *World) advanceConversion(id int) bool {
	if id <= 0 || id >= FollowerCapacity {
		return false
	}
	f := &w.Followers[id]
	if f.State == Inactive {
		return false
	}
	if !f.Conversion.Active {
		if w.Nature.Ground[int(f.X)+int(f.Y)*MapSize].Mark != GroundBaptism {
			return false
		}
		if f.State == Ruin || f.State == Airborne || f.Disease.Dying {
			return false
		}
		f.Conversion = ConversionState{Active: true, SourceOwner: f.Owner, Hero: f.IsHero()}
		wasTown := f.State == Town
		f.State = Converting
		f.positionX, f.positionY = int(f.X)*256+128, int(f.Y)*256+128
		f.positionSet = true
		w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, f.positionX, f.positionY)
		f.moving = false
		f.Frame = 0
		if wasTown {
			w.repaintFarms()
		}
	}
	f.Conversion.Frame++
	length := 11
	if !f.Conversion.Hero && f.Conversion.SourceOwner == 0 {
		length = 12
	}
	if int(f.Conversion.Frame) < length {
		return true
	}
	owner := f.Owner
	if w.Players[owner].Leader == id {
		w.Players[owner].Leader = 0
	}
	f.Owner ^= 1
	f.State = Walking
	f.Frame = 0
	f.Conversion = ConversionState{}
	x, y := int(f.X)+sign(f.velocityX), int(f.Y)+sign(f.velocityY)
	if x < 0 || x >= MapSize {
		x = int(f.X)
	}
	if y < 0 || y >= MapSize {
		y = int(f.Y)
	}
	fixedX, fixedY := x*256+(f.positionX&255), y*256+(f.positionY&255)
	w.moveFollowerCell(id, x, y)
	f.positionX, f.positionY = fixedX, fixedY
	w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, fixedX, fixedY)
	f.PreviousX, f.PreviousY = f.X, f.Y
	w.repaintFarms()
	return true
}
