package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

// Original secondary HUD gestures inspect existing heroes and the rally
// destination. They neither cast a power nor change the tactical mode.
func (g *Game) handleHUDSecondary(x, y int, newPress bool) (bool, error) {
	if g.Assets == nil || g.Assets.HUD == nil || g.World == nil {
		return false, nil
	}
	kind, index := visualassets.HUDHit(x, y)
	side := g.playerSide()
	if kind == "control" && g.Assets.HUD.Controls[index] == "rally" {
		if id := g.World.Players[side].Leader; selectedFollowerExists(g.World, id) {
			g.selectFollowerTemporarily(id)
			g.centerOnFollower(id)
		} else {
			p := g.World.Players[side]
			g.CameraX, g.CameraY = max(0, min(56, p.RallyX-4)), max(0, min(56, p.RallyY-4))
		}
		return true, nil
	}
	if kind != "power" {
		return false, nil
	}
	displayed := g.displayedGame()
	power := engine.PowerID(int(displayed.Category)*6 + index)
	if _, valid := engine.PowerByID(power); !valid {
		return true, nil
	}
	if !g.World.Level.Players[side].Powers[power] || !newPress && !g.World.Editor && g.World.Players[side].Mana < g.World.PanelPowerCost(side, power) {
		return true, nil
	}
	if _, hero := engine.HeroKindForPower(power); !hero {
		return g.inspectEffectIcon(power, newPress), nil
	}
	start := g.heroScanCursor
	if start < 0 || start >= engine.FollowerCapacity {
		start = 0
	}
	id, advance := start, newPress
	for scanned := 0; scanned < engine.FollowerCapacity; scanned++ {
		if advance {
			id = (id + 1) % engine.FollowerCapacity
			// The original stops before re-testing its stored cursor. This
			// keeps a new click from reselecting a sole existing hero.
			if id == start {
				break
			}
		}
		advance = true
		f := g.World.Followers[id]
		if id > 0 && f.State != engine.Inactive && int(f.Owner) == side && f.Hero.Kind != engine.HeroNone {
			g.heroScanCursor = id
			g.selectFollowerTemporarily(id)
			g.centerOnFollower(id)
			return true, nil
		}
	}
	return true, nil
}

// handleHUDHelp follows the dedicated Help key over an enabled power icon.
// Right-click inspection is a different gesture, and help has no mana gate.
func (g *Game) handleHUDHelp(x, y int) (bool, error) {
	if g.Assets == nil || g.Assets.HUD == nil || g.World == nil {
		return false, nil
	}
	kind, index := visualassets.HUDHit(x, y)
	if kind != "power" {
		return false, nil
	}
	view := g.displayedGame()
	id := engine.PowerID(int(view.Category)*6 + index)
	if _, ok := engine.PowerByID(id); !ok || !g.World.Level.Players[g.playerSide()].Powers[id] {
		return true, nil
	}
	return true, g.openPowerHelp(id)
}

func (g *Game) centerOnFollower(id int) {
	f := g.World.Followers[id]
	g.CameraX, g.CameraY = max(0, min(56, int(f.X)-4)), max(0, min(56, int(f.Y)-4))
}
