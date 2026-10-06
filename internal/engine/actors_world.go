package engine

// syncActorRegistry links newly created semantic actors and moves existing
// references without rebuilding chain order. It does not reproduce a memory
// image: every reference selects a named Go pool and a checked index.
func (w *World) syncActorRegistry() {
	sync := func(ref ActorRef, active bool, x, y int) {
		if !active {
			w.Actors.Unlink(ref)
			return
		}
		w.Actors.Move(ref, x, y)
	}
	for id := 1; id < FollowerCapacity; id++ {
		f := &w.Followers[id]
		x, y := f.Position()
		sync(ActorRef{ActorFollower, uint16(id)}, f.State != Inactive, int(x*256), int(y*256))
	}
	for id, a := range w.Nature.Scenery {
		ref := ActorRef{ActorScenery, uint16(id)}
		x, y, linked := w.Actors.Position(ref)
		if !linked || x>>8 != int(a.X) || y>>8 != int(a.Y) {
			x, y = int(a.X)*256+128, int(a.Y)*256+128
		}
		sync(ref, a.Kind != SceneryNone, x, y)
	}
	for id, a := range w.Earth.Walls {
		ref := ActorRef{ActorWall, uint16(id)}
		x, y, linked := w.Actors.Position(ref)
		if !linked || x>>8 != a.X || y>>8 != a.Y {
			x, y = a.X*256+128, a.Y*256+128
		}
		sync(ref, a.Active, x, y)
	}
	for id := range w.effects.Slots {
		w.syncEffectActor(id)
	}
	for id, a := range w.Magnets {
		sync(ActorRef{ActorMagnet, uint16(id)}, a.Owner < 2, a.X, a.Y)
	}
}

// syncEffectActor updates one actual creator or mover at its lifecycle point.
// It preserves cross-family insertion order and never scans unrelated pools.
func (w *World) syncEffectActor(id int) {
	if id < 0 || id >= EffectCapacity {
		return
	}
	active, x, y := false, 0, 0
	switch w.effects.Slots[id].Kind {
	case EffectFireColumn:
		a := w.Fire.Columns[id]
		active, x, y = a.Active, a.X, a.Y
	case EffectFireRain:
		a := w.Fire.Rain[id]
		active, x, y = a.Active && a.Phase != MeteorWaiting, a.X, a.Y
	case EffectLava:
		a := w.Fire.Lava[id]
		active, x, y = a.Active, a.X, a.Y
	case EffectWhirlwind:
		a := w.Air.Whirlwinds[id]
		active, x, y = a.Active, a.X, a.Y
	case EffectLightning:
		if a := w.Air.Markers[id]; a.Active {
			w.effects.Slots[id].InspectionClass = InspectLightningMarker
			active, x, y = true, a.X, a.Y
		} else {
			a := w.Air.Bolts[id]
			if a.Active {
				w.effects.Slots[id].InspectionClass = InspectLightningBolt
			}
			active, x, y = a.Active, a.X, a.Y
		}
	case EffectStorm:
		a := w.Air.Storms[id]
		active, x, y = a.Active, a.X, a.Y
	case EffectBasalt:
		a := w.Water.Basalt[id]
		active, x, y = a.Active, a.X*256+128, a.Y*256+128
	case EffectTidalWave:
		a := w.Water.Waves[id]
		active, x, y = a.Active, a.X, a.Y
		// Volcano, fungus, whirlpool and quake are unmapped controllers.
	}
	ref := ActorRef{Kind: ActorEffect, Index: uint16(id)}
	if active {
		w.Actors.Move(ref, x, y)
	} else {
		w.Actors.Unlink(ref)
	}
}
func (w *World) syncSceneryActor(id int) {
	if id < 0 || id >= SceneryCapacity {
		return
	}
	a := w.Nature.Scenery[id]
	ref := ActorRef{Kind: ActorScenery, Index: uint16(id)}
	if a.Kind == SceneryNone {
		w.Actors.Unlink(ref)
		return
	}
	w.Actors.Move(ref, int(a.X)*256+128, int(a.Y)*256+128)
}
func (w *World) syncWallActor(id int) {
	if id < 0 || id >= WallCapacity {
		return
	}
	a := w.Earth.Walls[id]
	ref := ActorRef{Kind: ActorWall, Index: uint16(id)}
	if !a.Active {
		w.Actors.Unlink(ref)
		return
	}
	w.Actors.Move(ref, a.X*256+128, a.Y*256+128)
}

func (h worldFireHabitat) SyncEffect(id int) { h.world.syncEffectActor(id) }
