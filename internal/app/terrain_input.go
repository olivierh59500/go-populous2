package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/network"
)

// applyTerrainClick uses the displayed view's construction rights. Scripted
// effects and AI orders keep their independent admission rules.
func (g *Game) applyTerrainClick(x, y int, lower bool) error {
	target := engine.PowerTarget{X: x, Y: y, Lower: lower}
	view := engine.Viewport{X: g.CameraX, Y: g.CameraY, Size: viewSize}
	if g.Network != nil {
		kind := "terrain-click"
		if lower {
			kind = "terrain-secondary"
		}
		return g.submitNetwork(network.Command{Kind: kind, Target: target, View: view})
	}
	if lower && g.World.Sprog(g.playerSide(), x, y) {
		return nil
	}
	return g.World.CastFromViewport(g.playerSide(), engine.RaiseLower, target, view)
}
