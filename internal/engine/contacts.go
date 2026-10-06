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
}

func (w *World) stepContact(id int) {
	f := &w.Followers[id]
	target := f.ContactWith
	if target <= 0 || target >= FollowerCapacity || w.Followers[target].State == Inactive {
		f.ContactWith = 0
		f.moving = false
		return
	}
	if f.moving {
		w.advanceLeg(id)
		return
	}
	f.ContactWith = 0
	other := &w.Followers[target]
	if f.Owner == other.Owner {
		w.InheritDiseaseMerge(id, target)
		other.Population += f.Population
		w.remove(id)
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
