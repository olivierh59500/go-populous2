package engine

// Repeated entries are intentional: the original policy weights several
// powers by placing them more than once in its random choice catalogue.
type AITargetKind uint8

const (
	AITargetTown AITargetKind = iota
	AITargetPrepared
	AITargetTwoWater
	AITargetSixWater
	AITargetLeader
	AITargetFinalBattle
)

type AIPowerChoice struct {
	Power  PowerID
	Target AITargetKind
}

var offensiveChoices = [24]AIPowerChoice{
	{Swamp, AITargetTown}, {Swamp, AITargetTown}, {Swamp, AITargetTown}, {Swamp, AITargetTown},
	{Whirlwind, AITargetTown}, {Whirlwind, AITargetTown}, {Whirlwind, AITargetTown},
	{FireRain, AITargetTown}, {FireRain, AITargetTown}, {Volcano, AITargetTown},
	{Earthquake, AITargetTown}, {Earthquake, AITargetTown}, {FireColumn, AITargetTown}, {FireColumn, AITargetTown},
	{Baptism, AITargetTown}, {Batholith, AITargetPrepared}, {Whirlpool, AITargetTwoWater}, {Tsunami, AITargetSixWater},
	{Storm, AITargetTown}, {Storm, AITargetTown}, {Storm, AITargetTown}, {Plague, AITargetTown}, {Wind, AITargetTown}, {Armageddon, AITargetFinalBattle},
}
var leaderChoices = [8]AIPowerChoice{{Perseus, AITargetLeader}, {Adonis, AITargetLeader}, {Adonis, AITargetLeader}, {Heracles, AITargetLeader}, {Odysseus, AITargetLeader}, {Achilles, AITargetLeader}, {Heracles, AITargetLeader}, {Helen, AITargetLeader}}

func (w *World) compileAIPowers(owner int) {
	a := &w.AI[owner]
	a.ChoiceCount = 1
	a.LeaderChoiceCount = 0
	a.ChoiceIndex = 0
	for _, choice := range offensiveChoices {
		if w.Level.Players[owner].Powers[choice.Power] {
			a.Choices[a.ChoiceCount] = choice
			a.ChoiceCount++
		}
	}
	for _, choice := range leaderChoices {
		if w.Level.Players[owner].Powers[choice.Power] {
			a.Choices[a.ChoiceCount+a.LeaderChoiceCount] = choice
			a.LeaderChoiceCount++
		}
	}
}

func (w *World) chooseAIOffensive(owner int) bool {
	if w.Tick < 250 {
		return false
	}
	a := &w.AI[owner]
	enemy := owner ^ 1
	if w.AI[enemy].BestTown == 0 {
		return false
	}
	choiceIndex := a.ChoiceIndex
	if choiceIndex == 0 {
		choiceIndex = int(w.random.next()) % max(1, a.ChoiceCount)
		if choiceIndex == 0 {
			return false
		}
	}
	if choiceIndex < 0 || choiceIndex >= a.ChoiceCount+a.LeaderChoiceCount {
		a.ChoiceIndex = 0
		return false
	}
	choice := a.Choices[choiceIndex]
	if choice.Target == AITargetFinalBattle {
		if uint64(w.Level.Players[owner].ArmageddonDeadline)<<6 < w.Tick || w.Players[owner].Population <= w.Players[enemy].Population {
			a.ChoiceIndex = 0
			return false
		}
	}
	power, known := PowerByID(choice.Power)
	if !known || power.Cost*4 >= w.Players[owner].Mana {
		a.ChoiceIndex = choiceIndex
		return false
	}
	a.ChoiceIndex = 0
	target := w.Followers[w.AI[enemy].BestTown]
	aim := PowerTarget{X: int(target.X), Y: int(target.Y)}
	switch choice.Target {
	case AITargetPrepared:
		a.PreparedPower = choice.Power
		a.PreparedTarget = aim
		a.Prepared = true
		return false
	case AITargetLeader:
		leader := w.Players[owner].Leader
		if leader <= 0 || w.Followers[leader].Population < 4096 {
			return false
		}
	case AITargetTwoWater, AITargetSixWater:
		required := 2
		if choice.Target == AITargetSixWater {
			required = 6
		}
		x, y, ok := w.aiWaterStrip(int(target.X), int(target.Y), required)
		if !ok {
			return false
		}
		aim.X, aim.Y = x, y
	}
	a.Order = AIOrder{Kind: AICastPower, Power: choice.Power, Target: aim}
	return true
}

func (w *World) aiWaterStrip(x, y, required int) (int, int, bool) {
	// The original water-strip scan retains its probe between directions and
	// compares signed packed-coordinate distances, including byte carry.
	current := uint16(y<<8 | x)
	probe := current
	best := int16(32767)
	candidate := uint16(0)
	remaining := required
	for _, offset := range [4]int16{-256, 1, 256, -1} {
		for steps := 0; steps < 65536; steps++ {
			probe += uint16(offset)
			px, py := int(uint8(probe)), int(uint8(probe>>8))
			if !inside(px, py) {
				break
			}
			if !w.Cell(px, py).IsWater() {
				remaining = required
				continue
			}
			remaining--
			if remaining != 0 {
				continue
			}
			current -= probe
			if int16(current) < 0 {
				current = uint16(-int16(current))
			}
			if best >= int16(current) {
				best, candidate = int16(current), probe
			}
			break
		}
	}
	if candidate == 0 {
		return 0, 0, false
	}
	return int(uint8(candidate)), int(uint8(candidate >> 8)), true
}

func (w *World) chooseAIMagnet(owner int) bool {
	a := &w.AI[owner]
	p := &w.Players[owner]
	switchMode := func() bool {
		a.MagnetCooldown = 10
		mode := Settle
		switch p.Population & 3 {
		case 2:
			mode = Join
		case 3:
			mode = Fight
		}
		a.Order = AIOrder{Kind: AISetMode, Mode: mode}
		return true
	}
	if p.Towns <= 30 {
		if p.Mode == Rally {
			return switchMode()
		}
		return false
	}
	leader := p.Leader
	if leader <= 0 || w.Followers[leader].State == Inactive {
		if a.ChoiceIndex >= a.ChoiceCount {
			a.ChoiceIndex = 0
		}
		if p.Mode != Rally {
			a.MagnetCooldown = 100
			a.Order = AIOrder{Kind: AISetMode, Mode: Rally}
			return true
		}
		return false
	}
	oldest := 0
	oldestTick := ^uint64(0)
	for id := 1; id < FollowerCapacity; id++ {
		f := w.Followers[id]
		if f.State == Town && int(f.Owner) == owner && f.FoundedAt <= oldestTick {
			oldest, oldestTick = id, f.FoundedAt
		}
	}
	population := w.Followers[leader].Population
	useEnemy := false
	if population >= 4096 && a.LeaderChoiceCount > 0 {
		next := int(w.random.next())%a.LeaderChoiceCount + a.ChoiceCount
		if a.ChoiceIndex >= a.ChoiceCount {
			useEnemy = true
		} else {
			a.ChoiceIndex = next
		}
	}
	if population >= 8192 {
		useEnemy = true
	}
	if useEnemy {
		oldest = 0
		oldestTick = ^uint64(0)
		for id := 1; id < FollowerCapacity; id++ {
			f := w.Followers[id]
			if f.State == Town && int(f.Owner) != (owner) && f.Owner < 2 && f.FoundedAt <= oldestTick {
				oldest, oldestTick = id, f.FoundedAt
			}
		}
	}
	if oldest == 0 {
		return false
	}
	target := w.Followers[oldest]
	if p.RallyX != int(target.X) || p.RallyY != int(target.Y) {
		a.Order = AIOrder{Kind: AICastPower, Power: PapalMagnet, Target: PowerTarget{X: int(target.X), Y: int(target.Y)}}
		return true
	}
	before := a.MagnetCooldown
	a.MagnetCooldown = int(uint16(a.MagnetCooldown - 1))
	if p.Mode == Rally {
		if int16(uint16(before)) <= 1 && !w.Cell(int(w.Followers[leader].X), int(w.Followers[leader].Y)).IsFlat() {
			return switchMode()
		}
	} else if int16(uint16(before)) <= 1 {
		a.MagnetCooldown = 100
		a.Order = AIOrder{Kind: AISetMode, Mode: Rally}
		return true
	}
	return false
}

func (w *World) chooseAIUrgent(owner int) bool {
	a := &w.AI[owner]
	if a.Prepared {
		power, ok := PowerByID(a.PreparedPower)
		if ok && power.Cost*4 < w.Players[owner].Mana {
			a.Order = AIOrder{Kind: AICastPower, Power: a.PreparedPower, Target: a.PreparedTarget}
			return true
		}
		a.Prepared = false
	}
	// A threatened walker reports its current parcel; emergency raising is
	// queued instead of altering terrain during follower simulation.
	for id := 1; id < FollowerCapacity; id++ {
		f := w.Followers[id]
		if f.State == Inactive || int(f.Owner) != owner {
			continue
		}
		if f.State == Drowning && a.Reaction <= 0 {
			a.Order = AIOrder{Kind: AIRaise, X: int(f.X), Y: int(f.Y)}
			return true
		}
	}
	return false
}
