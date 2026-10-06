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
	for id, slot := range w.effects.Slots {
		active, x, y := false, 0, 0
		switch slot.Kind {
		case EffectFireColumn:
			a := w.Fire.Columns[id]
			active, x, y = a.Active, a.X, a.Y
		case EffectFireRain:
			a := w.Fire.Rain[id]
			active, x, y = a.Active && a.Phase != MeteorWaiting, a.X, a.Y
		case EffectVolcano:
			a := w.Fire.Volcano[id]
			active, x, y = a.Active, a.X, a.Y
		case EffectLava:
			a := w.Fire.Lava[id]
			active, x, y = a.Active, a.X, a.Y
		case EffectWhirlwind:
			a := w.Air.Whirlwinds[id]
			active, x, y = a.Active, a.X, a.Y
		case EffectLightning:
			if a := w.Air.Markers[id]; a.Active {
				active, x, y = true, a.X*256+128, a.Y*256+128
			} else {
				a := w.Air.Bolts[id]
				active, x, y = a.Active, a.X, a.Y
			}
		case EffectStorm:
			a := w.Air.Storms[id]
			active, x, y = a.Active, a.X, a.Y
		case EffectBasalt:
			a := w.Water.Basalt[id]
			active, x, y = a.Active, a.X*256+128, a.Y*256+128
		case EffectWhirlpool:
			a := w.Water.Whirlpools[id]
			active, x, y = a.Active, a.X*256+128, a.Y*256+128
		case EffectTidalWave:
			a := w.Water.Waves[id]
			active, x, y = a.Active, a.X, a.Y
		case EffectEarthquake:
			a := w.Earth.Quakes[id]
			active, x, y = a.Active, a.X*256+128, a.Y*256+128
		}
		sync(ActorRef{ActorEffect, uint16(id)}, active, x, y)
	}
	for id, a := range w.Magnets {
		sync(ActorRef{ActorMagnet, uint16(id)}, a.Owner < 2, a.X, a.Y)
	}
}
