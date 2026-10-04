package populous

// PlayerPilot proposes ordinary player commands without running a computer
// player in the live world. The frontend must still select tools, move the
// camera, check construction presence and apply each command normally.
type PlayerPilot struct {
	Player      int
	state       ComputerStats
	next        ComputerStats
	initialized bool
	proposedAt  int
	command     Command
	people      [MaxPeeps]Peep
}

type advancedCommandTrace struct {
	enabled bool
	command Command
}

// Plan neither changes the live world nor reads its future random outcomes.
// Speculative spells use a separate fixed RNG; their outcomes are discarded.
func (p *PlayerPilot) Plan(w *World) Command {
	p.command = Command{}
	if w == nil || p.Player < 0 || p.Player > 1 || w.War || w.ComputerControlled[p.Player] || len(w.Peeps) > MaxPeeps {
		return p.command
	}
	if !p.initialized {
		p.state = w.Computer[p.Player]
		p.initialized = true
	}
	trial := *w
	copy(p.people[:], w.Peeps)
	trial.Peeps = p.people[:len(w.Peeps)]
	trial.SoundEvents = nil
	trial.legacyTurn = legacyTurnState{}
	trial.rng = 1
	trial.GameTurn++
	trial.Computer[p.Player] = p.state
	trial.Computer[p.Player].Mode = w.Computer[p.Player].Mode
	trial.Computer[p.Player].Speed = max(1, w.Computer[p.Player].Speed)
	trial.Computer[p.Player].DoneTurn = 0
	trial.ComputerControlled[p.Player] = true
	trial.updateComputerStats()
	trial.advancedTrace = advancedCommandTrace{enabled: true}
	trial.runAdvancedComputerPlayer(p.Player)
	p.next = trial.Computer[p.Player]
	p.proposedAt = trial.GameTurn
	p.command = trial.advancedTrace.command
	if p.command.Kind == CommandInvalid {
		p.state = p.next
	}
	return p.command
}

// Acknowledge keeps only planning memory, never transfers simulated terrain,
// mana, people, RNG, scores or computer privileges into the real game.
func (p *PlayerPilot) Acknowledge(w *World, accepted bool) {
	if !accepted || w == nil || p.command.Kind == CommandInvalid {
		p.command = Command{}
		return
	}
	if p.state.Arrived >= 0 && p.next.Arrived < 0 {
		alt := (-p.next.Arrived) & 15
		deadline := ((w.GameTurn + 1 + 7) / 8) * 8
		p.next.Arrived = -((deadline << 4) | alt)
	}
	if p.state.QuakeCount >= 0 && p.next.QuakeCount < 0 {
		p.next.QuakeCount -= max(0, w.GameTurn+1-p.proposedAt)
	}
	p.state = p.next
	p.command = Command{}
}

func (w *World) advancedOrder(command Command) bool {
	applied, err := w.ApplyCommand(command)
	if applied && err == nil && w.advancedTrace.enabled {
		w.advancedTrace.command = command
	}
	return applied && err == nil
}

func (w *World) advancedMagnet(player, pos int) bool {
	return w.advancedOrder(Command{Kind: CommandSetMagnet, Player: player, X: pos % MapWidth, Y: pos / MapWidth})
}
func (w *World) advancedTendency(player, mode int) bool {
	return w.advancedOrder(Command{Kind: CommandSetTendency, Player: player, Value: mode})
}
func (w *World) advancedKnight(player int) bool {
	return w.advancedOrder(Command{Kind: CommandKnight, Player: player})
}
func (w *World) advancedFlood(player int) bool {
	return w.advancedOrder(Command{Kind: CommandFlood, Player: player})
}
func (w *World) advancedWar(player int) bool {
	return w.advancedOrder(Command{Kind: CommandArmageddon, Player: player})
}
func (w *World) advancedQuake(player, x, y int) bool {
	return w.advancedOrder(Command{Kind: CommandQuake, Player: player, X: x, Y: y})
}
func (w *World) advancedVolcano(player, x, y int) bool {
	return w.advancedOrder(Command{Kind: CommandVolcano, Player: player, X: x, Y: y})
}
func (w *World) advancedSwamp(player, x, y int) bool {
	return w.advancedOrder(Command{Kind: CommandSwamp, Player: player, X: x, Y: y})
}
