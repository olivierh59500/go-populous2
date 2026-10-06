package app

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"go-populous2/internal/engine"
	"go-populous2/internal/network"
	"go-populous2/internal/visualassets"
)

var hudModes = [4]engine.Mode{engine.Settle, engine.Rally, engine.Join, engine.Fight}

// handleHUDClick uses the original slanted category, power and control grids.
// The controls dispatch the same named game commands as keyboard and network.
func (g *Game) handleHUDClick(x, y int) (bool, error) {
	if g.Assets == nil || g.Assets.HUD == nil || g.World == nil {
		return false, nil
	}
	kind, index := visualassets.HUDHit(x, y)
	switch kind {
	case "category":
		g.Category = engine.Element(index)
		g.PickingPower = false
		return true, nil
	case "power":
		id := engine.PowerID(int(g.Category)*6 + index)
		power, ok := engine.PowerByID(id)
		if !ok || !power.Implemented {
			return true, nil
		}
		if !g.World.Level.Players[g.playerSide()].Powers[id] {
			return true, nil
		}
		if !g.World.Editor && g.World.Players[g.playerSide()].Mana < g.World.PanelPowerCost(g.playerSide(), id) {
			return true, nil
		}
		g.Selected, g.Inspecting = id, false
		g.PickingPower = false
		return true, nil
	case "control":
		action := g.Assets.HUD.Controls[index]
		switch action {
		case "inspect":
			g.Inspecting = true
		case "menu":
			return true, g.openInGameMenu()
		case "settle", "rally", "join", "fight":
			mode := map[string]engine.Mode{"settle": engine.Settle, "rally": engine.Rally, "join": engine.Join, "fight": engine.Fight}[action]
			if g.Network != nil {
				return true, g.submitNetwork(network.Command{Kind: "mode", Mode: mode})
			}
			g.World.SetMode(g.playerSide(), mode)
		case "":
			return false, nil
		default:
			return true, fmt.Errorf("unknown HUD action")
		}
		return true, nil
	}
	return false, nil
}

func (g *Game) hudState(mouseX, mouseY int) visualassets.HUDState {
	w := g.World
	side := g.playerSide()
	s := visualassets.HUDState{Mana: uint32(max(0, w.Players[side].Mana)), Category: int(g.Category), Inspecting: g.Inspecting, Tick: w.Tick, MouseX: mouseX, MouseY: mouseY, Enabled: w.Level.Players[side].Powers}
	for index, mode := range hudModes {
		if mode == w.Players[side].Mode {
			s.Mode = index
		}
	}
	for slot := range s.Costs {
		s.Costs[slot] = 65535
	}
	for _, power := range engine.Powers {
		s.Costs[power.ID] = uint16(w.PanelPowerCost(side, power.ID) / 4)
	}
	for owner := range s.Population {
		s.Population[owner] = uint32(max(0, w.Players[owner].Population))
	}
	return s
}
func (g *Game) drawHUDControls() {
	if g.Assets == nil || g.Assets.HUD == nil || g.World == nil {
		return
	}
	x, y := ebiten.CursorPosition()
	state := g.hudState(x, y)
	art := g.Assets.HUD
	land := g.World.Level.Landscape
	palette := g.Assets.Visual.Palettes[land]
	art.DrawIcons(g.framebuffer, state, palette)
	art.DrawHighlights(g.framebuffer, state, palette)
	art.DrawIndicators(g.framebuffer, state, g.Assets.Visual.Sprites[land], palette)
}
