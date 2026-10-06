package app

import (
	"fmt"
	"go-populous2/internal/engine"
)

// drawRegisteredActor presents one member of the mixed parcel chain. Family
// banks share effect slots, but their drawing order is the registry's order.
func (g *Game) drawRegisteredActor(ref engine.ActorRef, land int) {
	switch ref.Kind {
	case engine.ActorFollower:
		if ref.Index > 0 && int(ref.Index) < len(g.World.Followers) && g.World.Followers[ref.Index].State != engine.Inactive {
			g.drawFollowerActor(int(ref.Index), land)
		}
	case engine.ActorScenery:
		g.drawSceneryActor(int(ref.Index), land)
	case engine.ActorWall:
		g.drawWallActor(int(ref.Index), land)
	case engine.ActorMagnet:
		g.drawMagnetActor(int(ref.Index), land)
	case engine.ActorEffect:
		g.drawEffectActor(int(ref.Index), land)
	}
}

func (g *Game) drawSceneryActor(id, land int) {
	if id < 0 || id >= len(g.World.Nature.Scenery) {
		return
	}
	scenery := g.World.Nature.Scenery[id]
	if scenery.Kind == engine.SceneryNone {
		return
	}
	name := "tree"
	if scenery.Kind == engine.SceneryBoulder {
		name = "boulder"
	}
	key := fmt.Sprintf("scenery/%s/%d", name, scenery.Variant)
	if scenery.Kind == engine.SceneryBurningTree {
		key = "scenery/burning-tree"
	}
	ax, ay := g.projectActor(int(scenery.X)*256+128, int(scenery.Y)*256+128)
	g.animationCropped(key, int(scenery.Frame), ax, ay, land, int(scenery.Age))
}

func (g *Game) drawWallActor(id, land int) {
	if id < 0 || id >= len(g.World.Earth.Walls) {
		return
	}
	wall := g.World.Earth.Walls[id]
	if !wall.Active {
		return
	}
	key := fmt.Sprintf("wall/connection/%d", wall.Connections)
	if wall.Gate {
		key = "wall/gate-horizontal"
		if wall.GateVertical {
			key = "wall/gate-vertical"
		}
	}
	if wall.Broken {
		key = fmt.Sprintf("wall/broken/%d", wall.Variant)
	}
	ax, ay := g.projectActor(int(wall.X)*256+128, int(wall.Y)*256+128)
	g.animation(key, int(wall.Frame), ax, ay+8, land)
}

func (g *Game) drawMagnetActor(id, land int) {
	if id < 0 || id >= len(g.World.Magnets) {
		return
	}
	magnet := g.World.Magnets[id]
	name := fmt.Sprintf("magnet/%d", id)
	animation, ok := g.Assets.Visual.Animations[name]
	if !ok || len(animation.Frames) == 0 {
		return
	}
	ax, ay := g.projectActor(magnet.X, magnet.Y)
	g.animation(name, int(g.World.Tick)%len(animation.Frames), ax, ay, land)
}

func (g *Game) drawEffectActor(id, land int) {
	if id < 0 || id >= engine.EffectCapacity {
		return
	}
	w := g.World
	if e := w.Fire.Columns[id]; e.Active {
		ax, ay := g.projectActor(e.X, e.Y)
		key := "fire-column/active"
		if e.Phase == engine.FireEmerging {
			key = "fire-column/emerging"
		}
		if e.Phase == engine.FireEnding {
			key = "fire-column/ending"
		}
		g.animation(key, e.Frame, ax, ay, land)
		return
	}
	if e := w.Fire.Rain[id]; e.Active {
		if e.Phase == engine.MeteorWaiting {
			return
		}
		ax, ay := g.projectActor(e.X, e.Y)
		key := "meteor/falling"
		if e.Phase != engine.MeteorFalling {
			key = "fire-impact/land"
			if e.WaterImpact {
				key = "fire-impact/water"
			}
		}
		g.animation(key, e.Frame, ax, ay, land)
		return
	}
	if e := w.Fire.Lava[id]; e.Active {
		key := lavaAnimation(w.Cell(e.X/256, e.Y/256).Shape, e.Direction)
		if key != "" {
			ax, ay := g.projectActor(e.X, e.Y)
			g.animation(key, e.Frame, ax, ay+8, land)
		}
		return
	}
	if e := w.Air.Storms[id]; e.Active {
		ax, ay := g.projectActor(e.X, e.Y)
		for _, d := range stormDraws(e, ax, ay) {
			if d.Strike {
				g.drawStormStrike(d, ay, land)
			} else {
				g.animation(d.Animation, d.Frame, d.X, d.Y, land)
			}
		}
		return
	}
	if e := w.Air.Markers[id]; e.Active {
		ax, ay := g.projectActor(e.X, e.Y)
		key := "lightning/active"
		if e.Phase == engine.LightningAppearing {
			key = "lightning/appearing"
		}
		if e.Phase == engine.LightningDisappearing {
			key = "lightning/ending"
		}
		g.animation(key, e.Frame, ax, ay+8, land)
		return
	}
	if e := w.Air.Whirlwinds[id]; e.Active {
		ax, ay := g.projectActor(e.X, e.Y)
		key := "whirlwind/active"
		if e.Phase == engine.WhirlwindAppearing {
			key = "whirlwind/appearing"
		}
		if e.Phase == engine.WhirlwindDisappearing {
			key = "whirlwind/ending"
		}
		g.animation(key, e.Frame, ax, ay, land)
		return
	}
	if e := w.Water.Waves[id]; e.Active {
		ax, ay := g.projectActor(e.X, e.Y)
		key := [4]string{"north", "east", "south", "west"}[e.Direction&3]
		g.animation("tidal/"+key, e.Frame, ax, ay, land)
		return
	}
	if e := w.Water.Basalt[id]; e.Active {
		ax, ay := g.projectActor(e.X*256+128, e.Y*256+128)
		g.animation("fire-impact/water", e.Frame, ax, ay, land)
	}
	// Volcano, earthquake, fungus, hurricane and whirlpool controllers change
	// terrain or other actors; they do not own an additional viewport sprite.
}
