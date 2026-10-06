package app

import (
	"fmt"
	"go-populous2/internal/engine"
	"image/color"
)

func (g *Game) drawLightningBeams(land int) {
	for _, bolt := range g.World.Air.Bolts {
		if !bolt.Active || bolt.Marker < 1 || bolt.Marker > engine.EffectCapacity {
			continue
		}
		marker := g.World.Air.Markers[bolt.Marker-1]
		if !marker.Active {
			continue
		}
		fromX, fromY := g.projectCorner(max(0, min(64, bolt.X/256)), max(0, min(64, bolt.Y/256)))
		toX, toY := g.projectCorner(marker.X/256, marker.Y/256)
		toY -= 32
		dx, dy := (toX-fromX)/4, (toY-fromY)/4
		random, direction := bolt.Random, 1
		if g.World.Tick&1 != 0 {
			direction = -1
		}
		x, y := fromX, fromY
		for segment := 0; segment < 3; segment++ {
			nx, ny := x+dx, y+dy
			jitter := int(random&7) + 2
			random >>= 1
			direction = -direction
			if direction < 0 {
				jitter = -jitter
			}
			nx += jitter
			g.line(x, y, nx, ny, g.Assets.Visual.Palettes[land][5])
			x, y = nx, ny
		}
		g.line(x, y, toX, toY, g.Assets.Visual.Palettes[land][5])
	}
}

func (g *Game) line(x0, y0, x1, y1 int, c color.RGBA) {
	dx, dy := absInt(x1-x0), -absInt(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	error := dx + dy
	for {
		if x0 >= 0 && x0 < 320 && y0 >= 0 && y0 < 178 {
			g.framebuffer.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		twice := error * 2
		if twice >= dy {
			error += dy
			x0 += sx
		}
		if twice <= dx {
			error += dx
			y0 += sy
		}
	}
}

func (g *Game) lightningVictim(id, land, x, y int) bool {
	state := g.World.AirVictims[id]
	if state.Phase == engine.LightningVictimNone || state.BoundOnly {
		return false
	}
	name := "lightning/hit"
	switch state.Sequence {
	case engine.LightningRecoverySequence:
		name = "lightning/recovery"
	case engine.LightningDeathSequence:
		name = "death/fire"
	}
	if state.Phase == engine.LightningVictimTownHit {
		name = "lightning/town-hit"
	}
	if g.World.Followers[id].IsHero() {
		name += "/" + heroNames[g.World.Followers[id].Hero.Kind]
	}
	g.animation(name, state.Frame, x, y, land)
	return true
}

func (g *Game) drawAirborne(id, land, x, y int) bool {
	state := g.World.Air.Carry[id]
	if state.Phase == engine.AirCarryNone {
		return false
	}
	name := "airborne/follower"
	if g.World.Followers[id].IsHero() {
		name = fmt.Sprintf("airborne/%s", heroNames[g.World.Followers[id].Hero.Kind])
	}
	if state.Phase == engine.AirCarryLanding {
		name = "airborne/landing"
	}
	g.animation(name, state.Frame, x, y, land)
	return true
}
