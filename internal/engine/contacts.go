package engine

// prepareContact keeps both groups on the same parcel until the arriving
// group reaches its target's fractional position. Contact is not an immediate
// merge simply because two actors share the same integer map coordinates.
func (w *World) prepareContact(source, target int) {
	if source <= 0 || target <= 0 || source == target {
		return
	}
	a, b := &w.Followers[source], &w.Followers[target]
	if a.State == Inactive || b.State == Inactive {
		return
	}
	a.ContactWith = target
	a.ContactFriendly = a.Owner == b.Owner
	a.moving = false
	a.initialisePosition()
	b.initialisePosition()
	dx, dy := b.positionX-a.positionX, b.positionY-a.positionY
	speed := int(a.MovementSpeed)
	if speed == 0 {
		return
	}
	a.velocityX, a.velocityY = sign(dx)*speed, sign(dy)*speed
	a.legRemaining = max(abs(dx), abs(dy)) / speed
	a.moving = true
	if b.moving {
		b.ContactWait = a.legRemaining + 1
		b.ContactWaiting = true
		b.moving = false
		b.Frame = 0
	}
}

func (w *World) stepContact(id int) {
	f := &w.Followers[id]
	if f.moving {
		w.advanceLeg(id)
		if f.moving {
			return
		}
	}
	f.ContactWith = 0
	friend, enemy := 0, 0
	var occupants [FollowerCapacity]int
	for _, other := range occupants[:w.FollowersAt(int(f.X), int(f.Y), occupants[:])] {
		if other == id {
			continue
		}
		g := w.Followers[other]
		if g.Owner == f.Owner {
			friend = other
		} else {
			enemy = other
		}
	}
	target := friend
	if target == 0 {
		target = enemy
	}
	if target == 0 {
		f.moving = false
		f.Frame = 0
		w.stepFollower(id)
		return
	}
	other := &w.Followers[target]
	if f.Owner == other.Owner {
		w.mergeFollowers(id, target)
		return
	}
	if f.Hero.Kind == HeroHelen {
		w.CaptureByHelen(id, target)
		return
	}
	if other.Hero.Kind == HeroHelen {
		w.CaptureByHelen(target, id)
		return
	}
	w.beginBattle(id, target)
}

func (w *World) stepContactWait(id int) {
	f := &w.Followers[id]
	f.Frame = (f.Frame + 1) % 2
	before := f.ContactWait
	f.ContactWait--
	if before <= 1 {
		f.ContactWaiting = false
		f.ContactWait = 0
		f.Frame = 0
		f.moving = false
		w.stepFollower(id)
	}
}

// mergeFollowers transfers identity attributes without counting an ordinary
// join as a death. The original leader and hero flags move to the survivor.
func (w *World) mergeFollowers(source, target int) {
	if source <= 0 || source >= FollowerCapacity || target <= 0 || target >= FollowerCapacity || source == target {
		return
	}
	from, to := &w.Followers[source], &w.Followers[target]
	owner := from.Owner
	if from.IsHero() {
		to.Hero = from.Hero
	}
	w.InheritDiseaseMerge(source, target)
	if int8(uint8(from.Weapons)) > int8(uint8(to.Weapons)) {
		to.Weapons = from.Weapons
	}
	to.Population = int(int32(uint32(to.Population) + uint32(from.Population)))
	if owner < 2 && w.Players[owner].Leader == source {
		w.Players[owner].Leader = target
	}
	if to.ContactWaiting {
		to.ContactWaiting = false
		to.ContactWait = 0
		to.moving = false
		to.Frame = 0
	}
	w.clearHeroLinks(source)
	w.SelectionTransfers.add(source, target)
	w.unlinkFollower(source)
	w.Followers[source] = Follower{}
}
