package app

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
)

func (g *Game) updateBriefing(x, y int, clicked bool) error {
	if clicked && x >= 24 && x < 288 && y >= 54 && y < 66 {
		g.editingWorldCode = true
		g.worldCodeInput = ""
	}
	if g.editingWorldCode {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 'a' && r <= 'z' {
				r -= 32
			}
			if r >= 'A' && r <= 'Z' && len(g.worldCodeInput) < 12 {
				g.worldCodeInput += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.worldCodeInput) > 0 {
			g.worldCodeInput = g.worldCodeInput[:len(g.worldCodeInput)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			index, ok := engine.DecodeLevelCode(g.worldCodeInput)
			if ok {
				g.LevelIndex = index
				g.editingWorldCode = false
			} else {
				g.Message = "UNKNOWN WORLD CODE"
				g.messageUntil = g.Updates + 100
			}
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		g.LevelIndex = max(0, g.LevelIndex-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		g.LevelIndex = min(len(g.Assets.Levels)-1, g.LevelIndex+1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || (clicked && x >= 115 && x < 215 && y >= 164 && y < 189) {
		return g.startConquest()
	}
	return nil
}

func (g *Game) briefingLines() []string {
	level := g.Assets.Levels[g.LevelIndex]
	code := level.Code
	if g.editingWorldCode {
		code = g.worldCodeInput + "_"
	}
	lines := []string{fmt.Sprintf("WORLD %d", g.LevelIndex), code}
	// Campaign author notes are not interface prose and may include editor tags
	// or control bytes. Present the actual opponent's experience in English.
	xp := level.OpponentExperience
	lines = append(lines, fmt.Sprintf("XP PEOPLE %d PLANTS %d", xp[engine.People], xp[engine.Plants]))
	lines = append(lines, fmt.Sprintf("XP EARTH %d AIR %d", xp[engine.Earth], xp[engine.Air]))
	lines = append(lines, fmt.Sprintf("XP FIRE %d WATER %d", xp[engine.Fire], xp[engine.Water]))
	foe := level.Players[1]
	lines = append(lines, fmt.Sprintf("ENEMY GROUPS %d PEOPLE %d", foe.Groups, foe.Population))
	lines = append(lines, fmt.Sprintf("SPEED %d WEAPONS %d", foe.MovementSpeed, foe.Weapons))
	return lines
}
