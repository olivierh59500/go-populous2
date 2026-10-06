package app

import (
	"fmt"
	"image"
	"image/draw"
)

func (g *Game) openInGameMenu() error {
	if g.World == nil {
		return fmt.Errorf("no game is active")
	}
	if g.Assets.InGameLayout == nil {
		return fmt.Errorf("original in-game menu assets are missing")
	}
	g.menuReturn, g.Screen = Playing, InGameMenuScreen
	return nil
}
func (g *Game) applyInGameAction(action string) error {
	side := g.playerSide()
	switch action {
	case "resume":
		g.Screen = g.menuReturn
	case "load":
		return g.openSaveBrowser(false)
	case "save":
		return g.openSaveBrowser(true)
	case "options":
		return g.openOptions()
	case "about":
		g.Screen = AboutScreen
	case "quit-map":
		if g.Network != nil {
			g.Network.Close()
			g.Network = nil
		}
		g.Screen = MainMenu
	case "restart":
		if g.Network != nil {
			return fmt.Errorf("leave the network game before restarting")
		}
		return g.startConquest()
	case "editor":
		if !g.CustomGame {
			return nil
		}
		return g.openEditor()
	case "network":
		if !g.CustomGame {
			return nil
		}
		g.openNetworkSetup()
	case "profile":
		if !g.CustomGame || g.Network != nil {
			return nil
		}
		g.LocalSide = 1 - side
	case "assist":
		if g.Network != nil {
			return fmt.Errorf("player control cannot change during a network game")
		}
		g.World.Players[side].Computer = !g.World.Players[side].Computer
	case "opponent-control":
		if g.Network != nil {
			return fmt.Errorf("player control cannot change during a network game")
		}
		g.World.Players[1-side].Computer = !g.World.Players[1-side].Computer
	}
	return nil
}
func (g *Game) drawInGameMenu() {
	layout := g.Assets.InGameLayout
	if layout == nil {
		return
	}
	g.drawWorld()
	side := "BLUE"
	if g.playerSide() == 1 {
		side = "RED"
	}
	assist := "OFF"
	if g.World.Players[g.playerSide()].Computer {
		assist = "ON"
	}
	opponent := "HUMAN"
	if g.World.Players[1-g.playerSide()].Computer {
		opponent = "COMPUTER"
	}
	layout.Draw(g.framebuffer, g.Assets.Visual.Font, map[string]string{"side": side, "assist": assist, "opponent": opponent}, nil)
	if g.Updates < g.messageUntil {
		g.drawMessage(176)
	}
}
func (g *Game) drawAbout() {
	// This text is replaced by the original decoded About requester when its
	// layout is available; the game information remains in English.
	if layout := g.Assets.InGameLayout; layout != nil {
		draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(layout.Palette[0]), image.Point{}, draw.Src)
	}
	g.text("POPULOUS II", 112, 56)
	g.text("BULLFROG PRODUCTIONS", 80, 80)
	g.text("GO / EBITENGINE RECREATION", 56, 104)
	g.text("PRESS ENTER TO RETURN", 72, 144)
}
