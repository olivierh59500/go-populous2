package engine

import "fmt"

type WindEffect struct {
	Active                bool
	Owner                 uint8
	X, Y, Direction, Life int
}

// CastWind reserves a directional wind controller. All parcel actors move
// through the same typed registry, so a crossing can legitimately visit an
// actor again when its destination appears later in the same scan.
func (w *World) CastWind(owner, x, y, direction int) error {
	if owner < 0 || owner > 1 || !inside(x, y) || direction < 0 || direction > 3 {
		return fmt.Errorf("invalid wind target")
	}
	id := w.allocateEffect(EffectHurricane, uint8(owner))
	if id < 0 {
		return fmt.Errorf("wind exhausted the shared effect pool")
	}
	if direction&1 == 0 {
		x = 0
	} else {
		y = 0
	}
	w.Wind[id] = WindEffect{Active: true, Owner: uint8(owner), X: x, Y: y, Direction: direction, Life: 100}
	return nil
}

func (w *World) tickWind(id int) {
	e := &w.Wind[id]
	if !e.Active {
		return
	}
	before := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if before <= 1 {
		e.Active = false
		w.releaseEffect(id)
		return
	}
	d := [4][2]int{{0, -16}, {16, 0}, {0, 16}, {-16, 0}}[e.Direction]
	cell := func(x, y int) { w.Pressure[x+y*MapSize] = 0; w.pushActorParcel(x, y, d[0], d[1]) }
	switch e.Direction {
	case 0:
		for at := e.X + e.Y*MapSize; at >= 0; at-- {
			cell(at%MapSize, at/MapSize)
		}
	case 2:
		for at := e.X + e.Y*MapSize; at < MapSize*MapSize; at++ {
			cell(at%MapSize, at/MapSize)
		}
	case 1:
		for x := e.X; x < MapSize; x++ {
			for y := 0; y < MapSize; y++ {
				cell(x, y)
			}
		}
	case 3:
		for x := e.X; x >= 0; x-- {
			for y := 0; y < MapSize; y++ {
				cell(x, y)
			}
		}
	}
}

func (w *World) pushActorParcel(x, y, dx, dy int) {
	ref := w.Actors.Heads[x+y*MapSize]
	for visits := 0; ref.Kind != ActorNone && visits < 1100; visits++ {
		px, py, active := w.Actors.Position(ref)
		if !active {
			return
		}
		if !w.moveActor(ref, px+dx, py+dy) {
			return
		}
		ref = w.Actors.Next(ref)
	}
}

func (w *World) moveActor(ref ActorRef, x, y int) bool {
	_, inside := actorCell(x, y)
	id := int(ref.Index)
	if !inside {
		switch ref.Kind {
		case ActorFollower:
			w.remove(id)
		case ActorScenery:
			w.Nature.Scenery[id] = SceneryActor{}
			w.Actors.Unlink(ref)
		case ActorWall:
			w.Earth.Walls[id].Active = false
			w.Actors.Unlink(ref)
		case ActorEffect:
			w.deactivateEffectActor(id)
			w.releaseEffect(id)
			w.Actors.Unlink(ref)
		case ActorMagnet:
			return false
		}
		return false
	}
	switch ref.Kind {
	case ActorFollower:
		f := &w.Followers[id]
		if int(f.X) != x>>8 || int(f.Y) != y>>8 {
			w.moveFollowerCell(id, x>>8, y>>8)
		}
		f.positionX, f.positionY, f.positionSet = x, y, true
	case ActorScenery:
		w.Nature.Scenery[id].X, w.Nature.Scenery[id].Y = uint8(x>>8), uint8(y>>8)
	case ActorWall:
		w.Earth.Walls[id].X, w.Earth.Walls[id].Y = x>>8, y>>8
	case ActorMagnet:
		w.Magnets[id].X, w.Magnets[id].Y = x, y
		w.Players[id].RallyX, w.Players[id].RallyY = x>>8, y>>8
	case ActorEffect:
		w.moveEffectActor(id, x, y)
	}
	w.Actors.Move(ref, x, y)
	return true
}

func (w *World) deactivateEffectActor(id int) {
	switch w.effects.Slots[id].Kind {
	case EffectFireColumn:
		w.Fire.Columns[id].Active = false
	case EffectFireRain:
		w.Fire.Rain[id].Active = false
	case EffectVolcano:
		w.Fire.Volcano[id].Active = false
	case EffectLava:
		w.Fire.Lava[id].Active = false
	case EffectWhirlwind:
		w.Air.Whirlwinds[id].Active = false
	case EffectLightning:
		w.Air.Markers[id].Active = false
		w.Air.Bolts[id].Active = false
	case EffectStorm:
		w.Air.Storms[id].Active = false
	case EffectBasalt:
		w.Water.Basalt[id].Active = false
	case EffectWhirlpool:
		w.Water.Whirlpools[id].Active = false
	case EffectTidalWave:
		w.Water.Waves[id].Active = false
	case EffectEarthquake:
		w.Earth.Quakes[id].Active = false
	}
}
func (w *World) moveEffectActor(id, x, y int) {
	switch w.effects.Slots[id].Kind {
	case EffectFireColumn:
		w.Fire.Columns[id].X, w.Fire.Columns[id].Y = x, y
	case EffectFireRain:
		w.Fire.Rain[id].X, w.Fire.Rain[id].Y = x, y
	case EffectVolcano:
		w.Fire.Volcano[id].X, w.Fire.Volcano[id].Y = x, y
	case EffectLava:
		w.Fire.Lava[id].X, w.Fire.Lava[id].Y = x, y
	case EffectWhirlwind:
		w.Air.Whirlwinds[id].X, w.Air.Whirlwinds[id].Y = x, y
	case EffectStorm:
		w.Air.Storms[id].X, w.Air.Storms[id].Y = x, y
	case EffectLightning:
		if w.Air.Markers[id].Active {
			w.Air.Markers[id].X, w.Air.Markers[id].Y = x>>8, y>>8
		} else {
			w.Air.Bolts[id].X, w.Air.Bolts[id].Y = x, y
		}
	case EffectBasalt:
		w.Water.Basalt[id].X, w.Water.Basalt[id].Y = x>>8, y>>8
	case EffectWhirlpool:
		w.Water.Whirlpools[id].X, w.Water.Whirlpools[id].Y = x>>8, y>>8
	case EffectTidalWave:
		w.Water.Waves[id].X, w.Water.Waves[id].Y = x, y
	case EffectEarthquake:
		w.Earth.Quakes[id].X, w.Earth.Quakes[id].Y = x>>8, y>>8
	}
}
