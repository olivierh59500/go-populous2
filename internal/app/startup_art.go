package app

import (
	"fmt"
	"image"
	"image/draw"

	"go-populous2/internal/visualassets"
)

func (g *Game) drawOriginalStartup() bool {
	if g.Assets.StartupMenu == nil {
		return false
	}
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), g.Assets.Visual.Startup, image.Point{}, draw.Src)
	g.Assets.StartupMenu.Draw(g.framebuffer, g.Assets.Visual.Font)
	if g.Updates < g.messageUntil {
		g.drawMessage(0)
	}
	return true
}

// handleOriginalStartup maps source layout slots to independent Go screens.
// The caller handles the returned quit request with its normal app shutdown.
func (g *Game) handleOriginalStartup(x, y int) (handled, quit bool, err error) {
	menu := g.Assets.StartupMenu
	if menu == nil {
		return false, false, nil
	}
	switch menu.ActionAt(x, y) {
	case visualassets.StartupProfile:
		g.openDeityProfile(MainMenu)
	case visualassets.StartupConquest:
		g.CustomGame = false
		g.Screen = ConquestBriefing
	case visualassets.StartupCustom:
		err = g.startCustomGame()
	case visualassets.StartupLoad:
		err = g.openSaveBrowser(false)
	case visualassets.StartupQuit:
		return true, true, nil
	case "":
		return true, false, nil
	default:
		return true, false, fmt.Errorf("unknown startup action")
	}
	return true, false, err
}
