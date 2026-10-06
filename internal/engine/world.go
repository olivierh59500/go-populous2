package engine

import "fmt"

type Mode uint8

const (
	Rally Mode = iota
	Settle
	Join
	Fight
)

type FollowerState uint8

const (
	Inactive FollowerState = iota
	Walking
	Town
	Fighting
	Drowning
	Ruin
)

type Follower struct {
	Owner                       uint8
	X, Y                        uint8
	State                       FollowerState
	Population, Weapons, Search int
	MovementSpeed               uint8
	Direction                   uint8
	Frame                       uint16
	Stage                       uint8
	Work                        uint16
	FoundedAt                   uint64
	SettleAfter                 uint64
	PreviousX, PreviousY        uint8
	MoveProgress                uint8
	Target                      int
	positionX, positionY        int
	velocityX, velocityY        int
	legRemaining                int
	positionSet, moving         bool
	BattleWith                  int
	BattleAggressor             bool
}

type Player struct {
	Experience                    [6]uint8
	Mana                          int
	Mode                          Mode
	RallyX, RallyY                int
	Leader                        int
	Computer                      bool
	Towns, Population, BattlesWon int
}

type Summary struct{ Population, Towns, Groups, BattlesWon, Mana int }

// World owns simulation state. All arrays have a geometric or game meaning;
// none represents CPU memory, a register bank, or a relocated executable.
type World struct {
	Level                Level
	Landscape            Landscape
	Nature               NatureState
	Heights              [CornerSize * CornerSize]uint8
	Tiles                [MapSize * MapSize]Cell
	Farms                [MapSize * MapSize]uint8 // Zero, blue, or red cultivation.
	Occupants            [MapSize * MapSize]uint16
	Footsteps            [MapSize * MapSize]uint16
	Pressure             [MapSize * MapSize]uint8
	Followers            [FollowerCapacity]Follower
	Players              [2]Player
	Tick                 uint64
	Result               int // Zero ongoing, one blue victory, two red victory.
	random               randomState
	effects              effectPool
	terrainTargets       [MapSize * MapSize]uint8
	terrainTargetSet     [MapSize * MapSize]bool
	developmentTarget    [CornerSize * CornerSize]uint8
	developmentTargetSet [CornerSize * CornerSize]bool
}

func NewWorld(level Level, land Landscape) (*World, error) {
	for stage := 1; stage < TownStages; stage++ {
		if land.WorkTicks[stage] < 1 || land.EmigrationDivisor[stage] < 1 {
			return nil, fmt.Errorf("invalid economy at town stage %d", stage)
		}
	}
	w := &World{Level: level, Landscape: land}
	w.generate(level.Seed)
	for owner, p := range level.Players {
		experience := [6]uint8{}
		if owner == 1 {
			experience = level.OpponentExperience
		}
		w.Players[owner] = Player{Experience: experience, Mana: p.Mana, Mode: Settle, RallyX: 32, RallyY: 32, Computer: owner == 1}
		placed := 0
		for pass := 0; pass < 2 && placed < p.Groups; pass++ {
			for n := 1; n < MapSize*MapSize && placed < p.Groups; n++ {
				at := n
				if owner == 1 {
					at = MapSize*MapSize - n
				}
				c := w.Tiles[at]
				if w.Occupants[at] != 0 || pass == 0 && !c.IsFlat() || pass == 1 && c.IsWater() {
					continue
				}
				id := w.allocate(Follower{Owner: uint8(owner), X: uint8(at % MapSize), Y: uint8(at / MapSize), PreviousX: uint8(at % MapSize), PreviousY: uint8(at / MapSize), State: Walking, Population: p.Population, Weapons: p.Weapons, Search: 2, MovementSpeed: p.MovementSpeed})
				if id == 0 {
					break
				}
				w.Occupants[at] = uint16(id)
				if placed == 0 {
					w.Players[owner].Leader = id
					w.Players[owner].RallyX = at % MapSize
					w.Players[owner].RallyY = at / MapSize
				}
				placed++
			}
		}
	}
	w.summarize()
	return w, nil
}

func (w *World) allocate(f Follower) int {
	for id := 1; id < len(w.Followers); id++ {
		if w.Followers[id].State == Inactive {
			f.initialisePosition()
			w.Followers[id] = f
			return id
		}
	}
	return 0
}
func (w *World) SetMode(owner int, mode Mode) bool {
	if owner < 0 || owner > 1 || mode > Fight {
		return false
	}
	w.Players[owner].Mode = mode
	return true
}
func (w *World) SetRally(owner, x, y int) bool {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return false
	}
	w.Players[owner].RallyX = x
	w.Players[owner].RallyY = y
	w.Players[owner].Mode = Rally
	return true
}

// Step advances one simulation pass. The presentation independently delivers
// audio and input at its display cadence; it must not call Step once per pixel.
func (w *World) Step() {
	if w.Result != 0 {
		return
	}
	w.Tick++
	w.tickNature()
	for owner := range w.Players {
		if w.Tick&1 == 0 && w.Players[owner].Mana < 32767 {
			w.Players[owner].Mana++
		}
		if w.Players[owner].Computer && w.Tick%4 == 0 {
			w.computerLand(owner)
		}
	}
	// New emigrants are processed on the next pass, avoiding a pool-index bias.
	var active [FollowerCapacity]bool
	for id := 1; id < len(w.Followers); id++ {
		active[id] = w.Followers[id].State != Inactive
	}
	for id := 1; id < len(w.Followers); id++ {
		if active[id] {
			w.stepFollower(id)
		}
	}
	w.repaintFarms()
	w.summarize()
	if w.Tick > 25 {
		if w.Players[0].Population == 0 && w.Players[1].Population > 0 {
			w.Result = 2
		}
		if w.Players[1].Population == 0 && w.Players[0].Population > 0 {
			w.Result = 1
		}
	}
}

func (w *World) stepFollower(id int) {
	f := &w.Followers[id]
	if w.EnterNatureHazard(id) {
		return
	}
	if f.State == Inactive || f.Population <= 0 {
		w.remove(id)
		return
	}
	tile := int(f.X) + int(f.Y)*MapSize
	if f.State == Fighting {
		w.stepBattle(id)
		return
	}
	if f.State == Town {
		stage := w.TownStage(int(f.Owner), int(f.X), int(f.Y), id)
		if stage == 0 || w.Players[f.Owner].Mode != Settle {
			f.State = Walking
			f.Stage = 0
			f.Work = 0
			return
		}
		f.Stage = uint8(stage)
		f.Frame = uint16(stage)
		f.Work++
		if int(f.Work) < w.Landscape.WorkTicks[stage] {
			return
		}
		f.Work = 0
		w.Players[f.Owner].Mana += w.Landscape.ManaAdd[stage]
		grown := f.Population + w.Landscape.PopulationAdd[stage]
		if grown <= w.Landscape.PopulationLimit[stage] {
			f.Population = grown
			return
		}
		emigrant := grown / w.Landscape.EmigrationDivisor[stage]
		if emigrant <= 0 || emigrant >= f.Population {
			return
		}
		// An emigrant must have a real adjacent destination. The parent is left
		// unchanged if the follower pool or the surrounding land is full.
		x, y, ok := w.emptyNeighbour(int(f.X), int(f.Y))
		if !ok {
			return
		}
		child := Follower{Owner: f.Owner, X: uint8(x), Y: uint8(y), PreviousX: uint8(f.X), PreviousY: uint8(f.Y), State: Walking, Population: emigrant, Weapons: stage, Search: stage * 2, MovementSpeed: f.MovementSpeed}
		next := w.allocate(child)
		if next == 0 {
			return
		}
		f.Population -= emigrant
		w.Occupants[x+y*MapSize] = uint16(next)
		if w.Players[f.Owner].Leader == id {
			w.Players[f.Owner].Leader = next
		}
		return
	}
	if w.Tiles[tile].IsWater() {
		f.State = Drowning
		f.Frame = (f.Frame + 1) % 4
		f.Population -= max(1, w.Level.Players[f.Owner].Attrition)
		if f.Population <= 0 {
			w.remove(id)
		}
		return
	}
	f.State = Walking
	if w.Players[f.Owner].Mode == Settle && w.Tick >= f.SettleAfter {
		if stage := w.TownStage(int(f.Owner), int(f.X), int(f.Y), id); stage > 0 {
			f.State = Town
			f.Stage = uint8(stage)
			f.Work = 0
			f.FoundedAt = w.Tick
			f.Frame = uint16(stage)
			return
		}
	}
	if !f.moving {
		f.Population -= w.Level.Players[f.Owner].Attrition
		if f.Population <= 0 {
			w.remove(id)
			return
		}
		x, y, ok := w.chooseMove(id)
		if !ok || !w.beginLeg(id, x, y) {
			f.Frame = (f.Frame + 1) % 4
			return
		}
	}
	w.advanceLeg(id)

}

func (w *World) emptyNeighbour(x, y int) (int, int, bool) {
	start := int(w.random.next()) % 8
	for n := 0; n < 8; n++ {
		d := directions[(start+n)%8]
		nx, ny := x+d[0], y+d[1]
		if inside(nx, ny) && !w.Tiles[nx+ny*MapSize].IsWater() && w.Occupants[nx+ny*MapSize] == 0 {
			return nx, ny, true
		}
	}
	return 0, 0, false
}

func (w *World) remove(id int) {
	f := w.Followers[id]
	at := int(f.X) + int(f.Y)*MapSize
	if w.Occupants[at] == uint16(id) {
		w.Occupants[at] = 0
	}
	if f.State != Inactive && w.Players[f.Owner].Leader == id {
		w.Players[f.Owner].Leader = 0
	}
	w.Nature.Deaths[id] = NatureAlive
	w.Nature.DeathFrames[id] = 0
	w.Followers[id] = Follower{}
}

func (w *World) summarize() {
	for owner := range w.Players {
		w.Players[owner].Population = 0
		w.Players[owner].Towns = 0
	}
	for _, f := range w.Followers[1:] {
		if f.State != Inactive {
			w.Players[f.Owner].Population += max(0, f.Population)
			if f.State == Town {
				w.Players[f.Owner].Towns++
			}
		}
	}
}
func (w *World) Summaries() [2]Summary {
	var result [2]Summary
	for owner, p := range w.Players {
		result[owner] = Summary{Population: p.Population, Towns: p.Towns, BattlesWon: p.BattlesWon, Mana: p.Mana}
	}
	for _, f := range w.Followers[1:] {
		if f.State != Inactive {
			result[f.Owner].Groups++
		}
	}
	return result
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Evacuate releases a town population as a walking group without fabricating
// an extra follower or changing its owner. It is the ordinary town right-click
// action and is separate from lowering the terrain under the settlement.
func (w *World) Evacuate(id int) bool {
	if id <= 0 || id >= FollowerCapacity || w.Followers[id].State != Town {
		return false
	}
	f := &w.Followers[id]
	f.State = Walking
	f.Stage = 0
	f.Work = 0
	f.Frame = 0
	// The group gets one movement interval to leave before it may settle again.
	f.moving = false
	f.SettleAfter = w.Tick + 24
	x, y, ok := w.emptyNeighbour(int(f.X), int(f.Y))
	if !ok {
		return true
	}
	at := int(f.X) + int(f.Y)*MapSize
	w.Occupants[at] = 0
	f.PreviousX, f.PreviousY = f.X, f.Y
	f.X, f.Y = uint8(x), uint8(y)
	f.positionSet = false
	f.initialisePosition()
	w.Occupants[x+y*MapSize] = uint16(id)
	w.repaintFarms()
	w.summarize()
	return true
}

// PlaceMagnet relocates the papal magnet without selecting the rally tactic.
// Mode selection is a separate HUD action, as in the original game.
func (w *World) PlaceMagnet(owner, x, y int) bool {
	if owner < 0 || owner > 1 || !inside(x, y) || w.Players[owner].Leader == 0 {
		return false
	}
	w.Players[owner].RallyX = x
	w.Players[owner].RallyY = y
	return true
}
