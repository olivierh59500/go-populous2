package app

import "go-populous2/internal/engine"

type overviewEffectPoint struct{ X, Y int }

func activeOverviewEffect(w *engine.World, id int) (overviewEffectPoint, bool) {
	if w == nil || id < 0 || id >= engine.EffectCapacity {
		return overviewEffectPoint{}, false
	}
	point := func(active bool, x, y int) (overviewEffectPoint, bool) {
		return overviewEffectPoint{X: x >> 8, Y: y >> 8}, active && x >= 0 && y >= 0 && x < engine.MapSize*256 && y < engine.MapSize*256
	}
	if e := w.Fire.Columns[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Fire.Rain[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Fire.Lava[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Fire.Volcano[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Air.Whirlwinds[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Air.Storms[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Air.Markers[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Air.Bolts[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Water.Waves[id]; e.Active {
		return point(true, e.X, e.Y)
	}
	if e := w.Water.Basalt[id]; e.Active {
		return overviewEffectPoint{e.X, e.Y}, true
	}
	if e := w.Water.Whirlpools[id]; e.Active {
		return overviewEffectPoint{e.X, e.Y}, true
	}
	if e := w.Earth.Quakes[id]; e.Active {
		return overviewEffectPoint{e.X, e.Y}, true
	}
	if e := w.Wind[id]; e.Active {
		return overviewEffectPoint{e.X, e.Y}, true
	}
	if e := w.Nature.Fungi[id]; e.Active {
		// The source omits the point on generation passes, but keeps it
		// during collection and the intervening evolution/aging passes.
		if !e.Collecting && e.Wait == e.Period {
			return overviewEffectPoint{}, false
		}
		return overviewEffectPoint{e.MinX, e.MinY}, true
	}
	return overviewEffectPoint{}, false
}

func visibleOverviewFollower(w *engine.World, observer, x, y int) (engine.Follower, bool) {
	if w == nil || x < 0 || y < 0 || x >= engine.MapSize || y >= engine.MapSize {
		return engine.Follower{}, false
	}
	chosen := 0
	for id, visits := int(w.Occupants[x+y*engine.MapSize]), 0; id > 0 && id < engine.FollowerCapacity && visits < engine.FollowerCapacity; visits++ {
		f := w.Followers[id]
		if f.State != engine.Inactive && w.FollowerVisibleOnMap(observer, int(f.Owner)) && id > chosen {
			chosen = id
		}
		id = f.NextFollower
	}
	if chosen == 0 {
		return engine.Follower{}, false
	}
	return w.Followers[chosen], true
}

// drawOverviewEffects uses the original shared FX palette color and the
// local player's disaster visibility rule. Terrain and people are drawn first.
func (g *Game) drawOverviewEffects(land int) {
	if !g.World.EffectVisibleOnMap(g.playerSide()) {
		return
	}
	for id := 0; id < engine.EffectCapacity; id++ {
		point, visible := activeOverviewEffect(g.World, id)
		if !visible || point.X < 0 || point.Y < 0 || point.X >= engine.MapSize || point.Y >= engine.MapSize {
			continue
		}
		g.framebuffer.SetRGBA(68+point.X-point.Y, 4+(point.X+point.Y)/2, g.Assets.Visual.Palettes[land][5])
	}
}
