package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/network"
)

// applyTerrainClick uses the displayed view's construction rights. Scripted
// effects and AI orders keep their independent admission rules.
func (g *Game) applyTerrainClick(x, y int, lower bool) error {
	target := engine.PowerTarget{X: x, Y: y, Lower: lower}
	displayed := g.displayedGame()
	view := engine.Viewport{X: displayed.CameraX, Y: displayed.CameraY, Size: viewSize}
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
	rights := displayed.World.CursorTerrainRights(g.playerSide(), view)
	return g.World.CastWithCursorTerrainRights(g.playerSide(), target, view, rights)
}
