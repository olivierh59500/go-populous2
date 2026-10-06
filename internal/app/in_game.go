package app

import (
	"fmt"
	"image"
	"image/draw"
)

func (g *Game) restoreControlMode() {
	g.controlMode = 2
	if g.World != nil && g.World.Players[g.playerSide()].Assisted {
		g.controlMode = 18
	} else if g.World != nil && g.World.Players[g.playerSide()].Computer {
		g.controlMode = 4
	}
}

func (g *Game) applyControlMode() {
	if g.World == nil {
		return
	}
	side := g.playerSide()
	g.World.Players[side].Computer = g.controlMode == 4
	g.World.Players[side].Assisted = g.controlMode == 18
}

func (g *Game) inGameControlLabels() (side, assist, matchup string) {
	side = "GOOD"
	if g.playerSide() == 1 {
		side = "BAD "
	}
	assist, matchup = "OFF", "HUMAN V HUMAN      "
	if g.World == nil {
		return
	}
	if g.World.Players[g.playerSide()].Assisted {
		assist = "ON "
	}
	computer0, computer1 := g.World.Players[0].Computer, g.World.Players[1].Computer
	if computer0 && computer1 {
		matchup = "COMPUTER V COMPUTER"
	} else if computer0 || computer1 {
		matchup = "COMPUTER V HUMAN   "
	}
	return
}

func (g *Game) openInGameMenu() error {
	if g.World == nil {
		return fmt.Errorf("no game is active")
	}
	if g.Assets.InGameLayout == nil {
		return fmt.Errorf("original in-game menu assets are missing")
	}
	g.restoreControlMode()
	g.menuReturn, g.Screen = g.Screen, InGameMenuScreen
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
		if g.Editor != nil {
			if err := g.applyEditor(); err != nil {
				return err
			}
			g.menuReturn = Playing
			g.Screen = InGameMenuScreen
			return nil
		}
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
		// The profile handoff exchanges both deity control words. Preserve
		// imported human and assistance modes instead of manufacturing an AI.
		old, new := &g.World.Players[side], &g.World.Players[1-side]
		old.Computer, new.Computer = new.Computer, old.Computer
		old.Assisted, new.Assisted = new.Assisted, old.Assisted
		old.Experience, new.Experience = new.Experience, old.Experience
		g.LocalSide = 1 - side
		g.restoreControlMode()
	case "assist":
		if g.Network != nil {
			return fmt.Errorf("player control cannot change during a network game")
		}
		g.restoreControlMode()
		if g.controlMode == 18 {
			g.controlMode = 2
		} else {
			g.controlMode = 18
		}
		g.applyControlMode()
	case "opponent-control":
		if g.Network != nil {
			return fmt.Errorf("player control cannot change during a network game")
		}
		g.restoreControlMode()
		if g.controlMode == 4 {
			g.controlMode = 2
		} else {
			g.controlMode = 4
		}
		g.applyControlMode()
	}
	return nil
}
func (g *Game) drawInGameMenu() {
	layout := g.Assets.InGameLayout
	if layout == nil {
		return
	}
	if g.Editor != nil {
		preview := *g
		preview.World = g.Editor.Draft
		preview.drawWorld()
	} else {
		g.drawWorld()
	}
	side, assist, opponent := g.inGameControlLabels()
	layout.Draw(g.framebuffer, g.Assets.Visual.Font, map[string]string{"side": side, "assist": assist, "opponent": opponent}, map[string]bool{"conquest": !g.CustomGame && g.Network == nil, "custom": g.CustomGame && g.Network == nil, "network": g.Network != nil, "paint": g.Editor != nil})
	if g.Updates < g.messageUntil {
		g.drawMessage(176)
	}
}
func (g *Game) drawAbout() {
	if layout := g.Assets.AboutLayout; layout != nil {
		g.drawWorld()
		layout.Draw(g.framebuffer, g.Assets.Visual.Font, nil, nil)
		return
	}
	if layout := g.Assets.InGameLayout; layout != nil {
		draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(layout.Palette[0]), image.Point{}, draw.Src)
	}
	g.text("POPULOUS II", 112, 56)
	g.text("BULLFROG PRODUCTIONS", 80, 80)
	g.text("GO / EBITENGINE RECREATION", 56, 104)
	g.text("PRESS ENTER TO RETURN", 72, 144)
}
