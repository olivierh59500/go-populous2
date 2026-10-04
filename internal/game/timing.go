package game

import "go-populous2/internal/fixedstep"

func (g *Game) SetSimulationRate(rate int) { g.scheduler = fixedstep.New(rate, 60) }
