package game

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/populous2"
)

var scenarioLabels = [10]string{"MODELER PARTOUT", "MODELER AU NIVEAU DE LA MER", "INTERDIRE LE TERRAIN ENNEMI", "INTERDIRE DE LEVER", "INTERDIRE DE BAISSER", "EAU MORTELLE", "CACHER LE CAMP ADVERSE SUR LA CARTE", "DESACTIVER LA SORTIE PAR CLIC DROIT", "CACHER LES CATASTROPHES SUR LA CARTE", "MARAIS PEU PROFONDS"}

func (g *Game) OpenScenarioRules() { g.ScenarioScreen = true; g.scenarioSide = 0 }

func (g *Game) displayedScenarioRule(side int) populous2.ScenarioRules {
	if !g.Playing && !g.World.Custom {
		return g.Bundle.Levels[g.LevelIndex].Players[side].ScenarioRules()
	}
	return g.World.Rules[side]
}

func (g *Game) updateScenarioRules() error {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if hit(x, y, 56, 64, 160, 28) {
		g.scenarioSide = 0
		return nil
	}
	if hit(x, y, 232, 64, 160, 28) {
		g.scenarioSide = 1
		return nil
	}
	if hit(x, y, 420, 414, 160, 28) {
		g.ScenarioScreen = false
		return nil
	}
	if !g.World.Custom {
		return nil
	}
	for bit := 0; bit < 10; bit++ {
		if !hit(x, y, 56, 108+bit*29, 528, 25) {
			continue
		}
		raw := g.World.Rules[g.scenarioSide].Raw ^ (1 << uint(bit))
		g.World.Rules[g.scenarioSide] = populous2.DecodeScenarioRules(raw)
		g.hasCustomScenarioOptions = true
		for side, rule := range g.World.Rules {
			g.customScenarioOptions[side] = rule.Raw
		}
		return nil
	}
	return nil
}

func (g *Game) drawScenarioRules(screen *ebiten.Image) {
	panel(screen, 40, 44, 560, 410)
	button(screen, 56, 64, 160, 28, "CAMP BLEU", true, g.scenarioSide == 0)
	button(screen, 232, 64, 160, 28, "CAMP ROUGE", true, g.scenarioSide == 1)
	rules := g.displayedScenarioRule(g.scenarioSide)
	for bit, name := range scenarioLabels {
		set := rules.Raw&(1<<uint(bit)) != 0
		marker := "[ ]"
		if set {
			marker = "[X]"
		}
		button(screen, 56, 108+bit*29, 528, 25, fmt.Sprintf("%s %s", marker, name), g.World.Custom, set)
	}
	message := "REGLES DE CONQUETE"
	if g.World.Custom {
		message = "PARTIE LIBRE : REGLES MODIFIABLES"
	}
	label(screen, message, 56, 420, muted)
	button(screen, 420, 414, 160, 28, "CONTINUER", true, false)
}
