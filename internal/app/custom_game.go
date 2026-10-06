package app

import "go-populous2/internal/engine"

// defaultCustomLevel is the original custom game's named starting setup.
// Campaign restrictions and opposition profiles do not supply these values.
func defaultCustomLevel(number, landscape int) engine.Level {
	level := engine.Level{Number: number, Code: engine.CodeForLevel(number), Landscape: landscape, Seed: 0x058028af, OpponentExperience: [6]uint8{255, 255, 255, 255, 255, 255}}
	for side := range level.Players {
		p := &level.Players[side]
		*p = engine.PlayerOptions{Groups: 1, Population: 50, MovementSpeed: 20, Weapons: 1, Mana: 800, Attrition: 2, ReactionDelay: 1, ArmageddonDeadline: 50, FixedMagnet: false, Extra: [5]uint16{0, 1, 50, 20, 65535}}
		if side == 1 {
			p.Mana = 200
		}
		p.Powers[engine.RaiseLower], p.Powers[engine.PapalMagnet], p.Powers[engine.Armageddon] = true, true, true
	}
	return level
}

func (g *Game) startCustomGame() error {
	previousLevel, previousComputer, previousCustom := g.CustomLevel, g.CustomComputer, g.CustomGame
	if g.CustomLevel == nil {
		landscape := 0
		if g.World != nil {
			landscape = g.World.Level.Landscape
		}
		level := defaultCustomLevel(g.LevelIndex, landscape)
		g.CustomLevel = &level
		g.CustomComputer = [2]bool{false, true}
	}
	g.CustomGame = true
	if err := g.startConquest(); err != nil {
		g.CustomLevel, g.CustomComputer, g.CustomGame = previousLevel, previousComputer, previousCustom
		return err
	}
	return nil
}
