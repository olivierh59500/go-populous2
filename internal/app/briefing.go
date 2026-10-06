package app

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
	"image"
	"image/draw"
	"strconv"
)

func (g *Game) briefingLevel() engine.Level {
	if g.CustomGame && g.CustomLevel != nil {
		return *g.CustomLevel
	}
	return g.Assets.Levels[g.LevelIndex]
}

func (g *Game) briefingValues() (map[string]string, map[string]bool) {
	level := g.briefingLevel()
	code := level.Code
	if g.editingWorldCode {
		code = g.worldCodeInput + "_"
	}
	d := &g.Assets.Conquest.Descriptor
	stage := min(max(g.LevelIndex/32, 0), len(d.Opponents)-1)
	values := map[string]string{"world-code": code, "world-number": strconv.Itoa(g.LevelIndex), "landscape": d.LandscapeNames[level.Landscape], "opponent": d.Opponents[stage].Name}
	s := level.Players[g.playerSide()].Scenario
	flags := map[string]bool{"build-anywhere": s.BuildAnywhere, "sea-level-only": s.SeaLevelOnly, "forbid-enemy-terrain": s.ForbidEnemyTerrain, "forbid-raise": s.ForbidRaise, "forbid-lower": s.ForbidLower, "fatal-water": s.FatalWater, "hide-enemy": s.HideEnemy, "disable-emigration": s.DisableEmigration, "hide-disasters": s.HideDisasters, "shallow-swamps": s.ShallowSwamps}
	return values, flags
}

func (g *Game) updateBriefing(x, y int, clicked bool) error {
	if g.briefingOpponent {
		if clicked || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.briefingOpponent = false
		}
		return nil
	}
	if g.editingWorldCode {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 'a' && r <= 'z' {
				r -= 32
			}
			if r >= 'A' && r <= 'Z' && len(g.worldCodeInput) < 10 {
				g.worldCodeInput += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.worldCodeInput) > 0 {
			g.worldCodeInput = g.worldCodeInput[:len(g.worldCodeInput)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.acceptBriefingCode()
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return g.handleBriefingAction("proceed")
	}
	if clicked && g.Assets.Conquest == nil && x >= 115 && x < 215 && y >= 168 && y < 185 {
		return g.handleBriefingAction("proceed")
	}
	if clicked && g.Assets.Conquest != nil {
		if action := g.Assets.Conquest.Descriptor.Layout.ActionAt(x, y); action != "" {
			return g.handleBriefingAction(action)
		}
		if slot, ok := g.Assets.Conquest.PowerAt(x, y); ok && g.briefingLevel().Players[g.playerSide()].Powers[slot] {
			id := engine.PowerID(slot)
			if _, ok := engine.PowerByID(id); ok {
				return g.openPowerHelp(id)
			}
		}
	}
	return nil
}

func (g *Game) acceptBriefingCode() bool {
	index, ok := engine.DecodeLevelCode(g.worldCodeInput)
	if !ok || index >= len(g.Assets.Levels) {
		g.Message, g.messageUntil = "UNKNOWN WORLD CODE", g.Updates+100
		return false
	}
	g.LevelIndex = index
	g.CustomGame, g.CustomLevel = false, nil
	g.editingWorldCode = false
	return true
}

func (g *Game) handleBriefingAction(action string) error {
	switch action {
	case "world-code":
		g.editingWorldCode = true
		g.worldCodeInput = ""
	case "opponent":
		g.briefingOpponent = true
	case "proceed":
		return g.startConquest()
	case "cancel":
		g.editingWorldCode, g.briefingOpponent = false, false
		g.Screen = MainMenu
	}
	return nil
}

func (g *Game) drawBriefing() {
	art := g.Assets.Conquest
	if art == nil {
		// Installations generated before the requester assets remain usable.
		draw.Draw(g.framebuffer, g.framebuffer.Bounds(), g.Assets.Visual.Startup, image.Point{}, draw.Src)
		g.text("CONQUEST", 128, 15)
		for index, line := range g.briefingLines() {
			g.text(line, 24, 42+index*14)
		}
		g.button("PROCEED", 115, 168, 100)
		return
	}
	palette := art.Descriptor.Layout.Palette
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(palette[0]), image.Point{}, draw.Src)
	if g.briefingOpponent {
		g.drawBriefingOpponent()
		return
	}
	values, flags := g.briefingValues()
	art.Descriptor.Layout.Draw(g.framebuffer, g.Assets.Visual.Font, values, flags)
	art.DrawIcons(g.framebuffer, g.briefingLevel().Players[g.playerSide()].Powers)
	if g.Updates < g.messageUntil {
		g.Assets.Visual.Font.Draw(g.framebuffer, g.Message, 16, 160, palette)
	}
}

func (g *Game) drawBriefingOpponent() {
	art := g.Assets.Conquest
	d := &art.Descriptor
	stage := min(max(g.LevelIndex/32, 0), len(d.Opponents)-1)
	foe := g.briefingLevel().Players[1-g.playerSide()]
	values := art.OpponentValues(g.LevelIndex, foe.ReactionDelay, foe.ArmageddonDeadline)
	d.OpponentLayout.Draw(g.framebuffer, g.Assets.Visual.Font, values, nil)
	art.DrawOpponentFace(g.framebuffer, stage, g.Assets.Visual.PortraitParts)
}

func (g *Game) briefingLines() []string {
	level := g.briefingLevel()
	code := level.Code
	if g.editingWorldCode {
		code = g.worldCodeInput + "_"
	}
	xp := level.OpponentExperience
	foe := level.Players[1]
	return []string{fmt.Sprintf("WORLD %d", g.LevelIndex), code, fmt.Sprintf("XP PEOPLE %d PLANTS %d", xp[engine.People], xp[engine.Plants]), fmt.Sprintf("XP EARTH %d AIR %d", xp[engine.Earth], xp[engine.Air]), fmt.Sprintf("XP FIRE %d WATER %d", xp[engine.Fire], xp[engine.Water]), fmt.Sprintf("ENEMY GROUPS %d PEOPLE %d", foe.Groups, foe.Population), fmt.Sprintf("SPEED %d WEAPONS %d", foe.MovementSpeed, foe.Weapons)}
}
