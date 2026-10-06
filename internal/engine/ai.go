package engine

// AIOrder is an ordinary deferred game command. Thinking never changes the
// landscape in the AI phase; execution follows effects and scenery.
type AIOrderKind uint8

const (
	AINoOrder AIOrderKind = iota
	AIRaise
	AILower
	AIReleaseTown
	AICastPower
	AISetMode
)

type AIOrder struct {
	Kind           AIOrderKind
	X, Y, Follower int
	Power          PowerID
	Mode           Mode
	Target         PowerTarget
}
type AIState struct {
	TerrainRequestFollower, TerrainRequestX, TerrainRequestY    int
	WaterRequestFollower                                        int
	Choices                                                     [33]AIPowerChoice
	ChoiceCount, LeaderChoiceCount, ChoiceIndex, MagnetCooldown int
	Prepared                                                    bool
	PreparedPower                                               PowerID
	PreparedTarget                                              PowerTarget
	Reaction, ExpansionCooldown, ReleaseCooldown                int
	ExpansionTown, BestTown, BestPopulation                     int
	Order                                                       AIOrder
}

var expansionParcels = [60][2]int{
	{0, -1}, {1, -1}, {2, 0}, {2, 1}, {1, 2}, {0, 2}, {-1, 1}, {-1, 0}, {-1, -1}, {2, -1}, {2, 2}, {-1, 2},
	{-3, 0}, {-3, -1}, {-3, -2}, {-3, -3}, {-2, -3}, {-1, -3}, {0, -3}, {1, -3}, {2, -3}, {3, -3}, {4, -3}, {4, -2}, {4, -1}, {4, 0}, {4, 1}, {4, 2}, {4, 3}, {4, 4}, {3, 4}, {2, 4}, {1, 4}, {0, 4}, {-1, 4}, {-2, 4}, {-3, 4}, {-3, 3}, {-3, 2}, {-3, 1},
	{-2, 0}, {-2, -1}, {-2, -2}, {-1, -2}, {0, -2}, {1, -2}, {2, -2}, {3, -2}, {3, -1}, {3, 0}, {3, 1}, {3, 2}, {3, 3}, {2, 3}, {1, 3}, {0, 3}, {-1, 3}, {-2, 3}, {-2, 2}, {-1, 2},
}

func (w *World) observeAITown(id int, previousStage uint8) {
	f := w.Followers[id]
	a := &w.AI[f.Owner]
	if (previousStage == 0 || f.Stage < 17) && f.Stage != f.LastDevelopedStage {
		a.ExpansionTown = id
	}
	population := int(int16(uint16(f.Population)))
	if population > a.BestPopulation {
		a.BestPopulation = population
		a.BestTown = id
	}
}
func (w *World) beginAIObservations() {
	for owner := range w.AI {
		w.AI[owner].ExpansionTown = 0
		w.AI[owner].BestTown = 0
		w.AI[owner].BestPopulation = 0
		w.AI[owner].TerrainRequestFollower = 0
		w.AI[owner].WaterRequestFollower = 0
	}
}

func (w *World) thinkAI(owner int) {
	if !w.Players[owner].Computer {
		return
	}
	a := &w.AI[owner]
	a.Reaction--
	w.chooseAIUrgent(owner)
	if a.Reaction > 0 {
		return
	}
	a.Reaction = w.Level.Players[owner].ReactionDelay
	if a.Order.Kind != AINoOrder {
		return
	}
	if w.chooseAIExpansion(owner) || w.chooseAIRelease(owner) {
		return
	}
	if !w.chooseAIOffensive(owner) {
		w.chooseAIMagnet(owner)
	}
}

func (w *World) chooseAIExpansion(owner int) bool {
	a := &w.AI[owner]
	id := a.ExpansionTown
	if id <= 0 || w.Followers[id].State != Town {
		return false
	}
	before := a.ExpansionCooldown
	a.ExpansionCooldown--
	if int16(uint16(before)) > 1 {
		return false
	}
	a.ExpansionCooldown = 2
	if w.Players[owner].Mana < 22 {
		return false
	}
	f := &w.Followers[id]
	origin := w.Heights[int(f.X)+int(f.Y)*CornerSize]
	for _, d := range expansionParcels {
		x, y := int(f.X)+d[0], int(f.Y)+d[1]
		if !inside(x, y) {
			continue
		}
		skip := false
		var actors [FollowerCapacity]int
		for _, other := range actors[:w.FollowersAt(x, y, actors[:])] {
			g := w.Followers[other]
			if g.State != Town && int(g.Owner) == owner {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		height := w.Heights[x+y*CornerSize]
		code := w.Cell(x, y).Code
		forceRaise := code == 95
		if !forceRaise && (code == 143 || code == 144 || code >= 145 && code <= 150 || code >= 168 && code <= 171) {
			forceRaise = w.random.next()&15 <= 3
		}
		if !forceRaise && height == origin {
			continue
		}
		kind := AIRaise
		if !forceRaise && height > origin {
			kind = AILower
		}
		a.Order = AIOrder{Kind: kind, X: x, Y: y}
		return true
	}
	f.LastDevelopedStage = f.Stage
	return false
}

func (w *World) chooseAIRelease(owner int) bool {
	a := &w.AI[owner]
	if a.BestTown == 0 {
		return false
	}
	before := a.ReleaseCooldown
	a.ReleaseCooldown--
	if int16(uint16(before)) > 1 {
		return false
	}
	a.ReleaseCooldown = 4
	if w.Players[owner].Mana < 20 {
		return false
	}
	threshold := w.Landscape.PopulationLimit[1]
	if w.Players[owner].Mode == Rally {
		threshold = w.Landscape.PopulationLimit[4]
	}
	if w.Followers[a.BestTown].Population <= threshold {
		return false
	}
	f := w.Followers[a.BestTown]
	a.Order = AIOrder{Kind: AIReleaseTown, Follower: a.BestTown, X: int(f.X), Y: int(f.Y)}
	return true
}

func (w *World) executeAIOrders() {
	for owner := range w.AI {
		order := w.AI[owner].Order
		w.AI[owner].Order = AIOrder{}
		switch order.Kind {
		case AIRaise:
			w.changeHeight(owner, order.X, order.Y, true, false)
		case AILower:
			w.changeHeight(owner, order.X, order.Y, false, false)
		case AIReleaseTown:
			w.Sprog(owner, order.X, order.Y)
		case AICastPower:
			_ = w.Cast(owner, order.Power, order.Target)
		case AISetMode:
			w.SetMode(owner, order.Mode)
		}
	}
}
