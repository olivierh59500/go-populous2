package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/network"
)

var hudModes = [4]engine.Mode{engine.Settle, engine.Rally, engine.Join, engine.Fight}

// handleHUDClick consumes controls before terrain picking. The same named
// commands serve the mouse and keyboard, including the joining player's camp.
func (g *Game) handleHUDClick(x, y int) (bool, error) {
	if x >= 4 && x < 104 && y >= 68 && y < 76 {
		g.Inspecting = !g.Inspecting
		return true, nil
	}
	if x < 4 || x >= 104 || y < 77 || y >= 174 {
		return false, nil
	}
	if y < 96 {
		g.PickingPower = true
		return true, nil
	}
	if y < 138 {
		index := (x-4)/50 + 2*((y-98)/20)
		if y < 98 || index < 0 || index > 3 {
			return true, nil
		}
		mode := hudModes[index]
		if g.Network != nil {
			return true, g.submitNetwork(network.Command{Kind: "mode", Mode: mode})
		}
		g.World.SetMode(g.playerSide(), mode)
		return true, nil
	}
	if y < 156 {
		if x < 54 {
			g.Selected = engine.PapalMagnet
		} else {
			g.helpReturn, g.Screen = Playing, HelpScreen
		}
		return true, nil
	}
	if x < 54 {
		return true, g.openOptions()
	}
	return true, g.openSaveBrowser(true)
}

func (g *Game) drawHUDControls() {
	label := "INSPECT [I]"
	if g.Inspecting {
		label = "INSPECT ON"
	}
	g.text(label, 8, 68)
	g.button("POWERS", 4, 77, 100)
	for i, name := range [4]string{"BUILD", "RALLY", "JOIN", "FIGHT"} {
		g.button(name, 4+(i%2)*50, 98+(i/2)*20, 48)
	}
	g.button("FLAG", 4, 138, 48)
	g.button("HELP", 54, 138, 48)
	g.button("OPT", 4, 156, 48)
	g.button("SAVE", 54, 156, 48)
}
