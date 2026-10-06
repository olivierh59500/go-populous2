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
	Airborne
	Converting
)

type Follower struct {
	Owner                          uint8
	Hero                           HeroState
	Disease                        DiseaseState
	Conversion                     ConversionState
	TerrainDeath                   TerrainDeathState
	Neutral                        NeutralState
	Consecrated                    bool
	X, Y                           uint8
	State                          FollowerState
	Population, Weapons, Search    int
	MovementSpeed                  uint8
	Direction                      uint8
	Frame                          uint16
	Stage                          uint8
	LastDevelopedStage             uint8
	ForceEmigration                bool
	Work                           uint16
	FoundedAt                      uint64
	SettleAfter                    uint64
	PreviousX, PreviousY           uint8
	MoveProgress                   uint8
	Target                         int
	positionX, positionY           int
	velocityX, velocityY           int
	legRemaining                   int
	positionSet, moving            bool
	NextFollower, PreviousFollower int
	ContactWith                    int
	ContactFriendly                bool
	BattleWith                     int
	BattleAggressor                bool
}

type Player struct {
	Statistics                    CampaignStatistics
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
	Editor               bool
	BirthBlocked         bool
	NeutralBirthDeadline uint64
	Scenario             ScenarioState
	Level                Level
	Landscape            Landscape
	Nature               NatureState
	Fire                 FireEffects
	FireDamage           FireDamageState
	Water                WaterEffects
	Air                  AirEffects
	Earth                EarthState
	AI                   [2]AIState
	Wind                 [EffectCapacity]WindEffect
	AirVictims           [FollowerCapacity]LightningVictimState
	Heights              [CornerSize * CornerSize]uint8
	Tiles                [MapSize * MapSize]Cell
	Farms                [MapSize * MapSize]uint8 // Zero, blue, or red cultivation.
	Occupants            [MapSize * MapSize]uint16
	Footsteps            [MapSize * MapSize]uint16
	Pressure             [MapSize * MapSize]uint8
	Followers            [FollowerCapacity]Follower
	Actors               ActorRegistry
	Magnets              [2]MagnetActor
	Players              [2]Player
	Tick                 uint64
	Armageddon           bool
	Result               int // Zero ongoing, one blue victory, two red victory.
	random               randomState
	effects              effectPool
}

func NewWorld(level Level, land Landscape) (*World, error) {
	for stage := 1; stage < TownStages; stage++ {
		if land.WorkTicks[stage] < 1 || land.EmigrationDivisor[stage] < 1 {
			return nil, fmt.Errorf("invalid economy at town stage %d", stage)
		}
	}
	scenario, err := DecodeScenarioEvents(level.WorldParameters)
	if err != nil {
		return nil, err
	}
	w := &World{Level: level, Landscape: land, Scenario: scenario}
	w.generate(level.Seed)
	for owner := range w.Magnets {
		w.Magnets[owner] = MagnetActor{X: 32*256 + 128, Y: 32*256 + 128, Owner: uint8(owner)}
		w.Actors.Link(ActorRef{Kind: ActorMagnet, Index: uint16(owner)}, w.Magnets[owner].X, w.Magnets[owner].Y)
	}
	for owner, p := range level.Players {
		experience := [6]uint8{}
		if owner == 1 {
			experience = level.OpponentExperience
		}
		w.Players[owner] = Player{Experience: experience, Mana: p.Mana, Mode: Settle, RallyX: 32, RallyY: 32, Computer: owner == 1}
		if p.FixedMagnet && inside(p.MagnetX, p.MagnetY) {
			w.Players[owner].RallyX = p.MagnetX
			w.Players[owner].RallyY = p.MagnetY
			w.moveMagnet(owner, p.MagnetX, p.MagnetY)
		}
		w.compileAIPowers(owner)
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
				w.linkFollower(id)
				if placed == 0 {
					w.Players[owner].Leader = id
					if !p.FixedMagnet {
						w.Players[owner].RallyX = at % MapSize
						w.Players[owner].RallyY = at / MapSize
						w.moveMagnet(owner, at%MapSize, at/MapSize)
					}
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
	w.moveMagnet(owner, x, y)
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
	w.beginAIObservations()
	w.BirthBlocked = false
	for owner := range w.Players {
		if w.Tick&1 == 0 && w.Players[owner].Mana < 32767 {
			w.Players[owner].Mana++
		}
	}
	// Source simulation phases process followers, AI, the shared effect pool,
	// and scenery in that order. A newborn in a later slot runs this pass;
	// a reused lower slot waits for the following pass.
	for id := 1; id < FollowerCapacity; id++ {
		if w.Followers[id].State != Inactive {
			w.stepFollower(id)
		}
	}
	for owner := range w.Players {
		w.thinkAI(owner)
	}
	for id := 0; id < EffectCapacity; id++ {
		switch w.effects.Slots[id].Kind {
		case EffectFungus:
			w.tickNatureEffect(id)
		case EffectFireColumn, EffectFireRain, EffectVolcano, EffectLava:
			w.tickFireEffect(id)
		case EffectHurricane:
			w.tickWind(id)
		case EffectEarthquake:
			w.tickEarthEffect(id)
		case EffectLightning, EffectWhirlwind, EffectStorm:
			w.tickAirEffect(id)
		case EffectBasalt, EffectWhirlpool, EffectTidalWave:
			w.tickWaterEffect(id)
		}
	}
	w.tickWalls()
	w.tickNatureScenery()
	w.tickScenario()
	w.executeAIOrders()
	w.repaintFarms()
	w.summarize()
	w.RecordCampaignMetrics()
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
	if f.Owner > 1 {
		w.tickNeutral(id)
		return
	}
	if w.advanceNeutralVictim(id) {
		return
	}
	if w.tickDisease(id) {
		return
	}
	if w.advanceConversion(id) {
		return
	}
	if w.AdvanceAirCarry(id) {
		return
	}
	if w.AdvanceLightningVictim(id) {
		return
	}
	if w.AdvanceFireDeath(id) {
		return
	}
	if w.EnterNatureHazard(id) {
		return
	}
	if f.State == Inactive || f.Population <= 0 {
		w.remove(id)
		return
	}
	if w.advanceWater(id) {
		return
	}
	if f.State == Fighting {
		w.stepBattle(id)
		return
	}
	if f.ContactWith != 0 {
		w.stepContact(id)
		return
	}
	if f.Hero.CaptiveOf != 0 {
		w.stepCaptive(id)
		return
	}
	if f.IsHero() {
		w.stepHero(id)
		return
	}
	if f.State == Town {
		oldStage := f.Stage
		stage := w.EvaluateTown(id)
		if stage == 0 || w.Players[f.Owner].Mode != Settle {
			f.State = Walking
			f.Stage = 0
			f.Work = 0
			return
		}
		f.Stage = uint8(stage)
		w.observeAITown(id, oldStage)
		f.Frame = uint16(stage)
		f.Work++
		if int(f.Work) < w.Landscape.WorkTicks[stage] {
			return
		}
		f.Work = 0
		if !f.Disease.Infected {
			w.Players[f.Owner].Mana = int(uint32(w.Players[f.Owner].Mana) + uint32(w.Landscape.ManaAdd[stage]))
		}
		grown := int(int32(uint32(f.Population) + uint32(w.Landscape.PopulationAdd[stage])))
		if grown <= w.Landscape.PopulationLimit[stage] && !f.ForceEmigration || w.BirthBlocked {
			f.Population = grown
			return
		}
		f.ForceEmigration = false
		emigrant := int(uint16(grown))
		divisor := w.Landscape.EmigrationDivisor[stage]
		if quotient := uint32(grown) / uint32(divisor); quotient <= 65535 {
			emigrant = int(quotient)
		}
		if emigrant < 0 {
			return
		}
		// The newborn starts at the parent's exact fractional position. Its
		// ordinary search runs later in the ascending follower pass.
		child := Follower{Disease: f.Disease, Owner: f.Owner, X: f.X, Y: f.Y, PreviousX: f.X, PreviousY: f.Y, State: Walking, Population: emigrant, Weapons: stage, Search: stage * 2, MovementSpeed: f.MovementSpeed, positionSet: true, positionX: f.positionX, positionY: f.positionY}

		next := w.allocate(child)
		if next == 0 {
			w.BirthBlocked = true
			return
		}
		f.Population -= emigrant
		if next == 250 && w.Tick >= w.NeutralBirthDeadline {
			w.NeutralBirthDeadline = w.Tick + 50
			w.random.next()
			bits := w.random.next()
			selector := (bits % 12) &^ 1
			kind := NeutralKind(selector/2 + 1)
			_, _ = w.CreateNeutral(kind, int(selector+2), int(bits&63))
		}
		w.linkFollower(next)
		if w.Players[f.Owner].Leader == id {
			w.Players[f.Owner].Leader = next
		}
		return
	}

	f.State = Walking

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
	w.clearHeroLinks(id)
	f := w.Followers[id]
	w.unlinkFollower(id)
	if f.State != Inactive && f.Owner < 2 && w.Players[f.Owner].Leader == id {
		w.Players[f.Owner].Leader = 0
	}
	w.Air.Carry[id] = AirCarryState{}
	w.AirVictims[id] = LightningVictimState{}
	w.FireDamage.Deaths[id] = FireVictimDeath{}
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
		if f.State != Inactive && f.Owner < 2 {
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
		if f.State != Inactive && f.Owner < 2 {
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
	if id <= 0 || id >= FollowerCapacity || w.Followers[id].State != Town || w.Level.Players[w.Followers[id].Owner].Scenario.DisableEmigration {
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
	w.unlinkFollower(id)
	f.PreviousX, f.PreviousY = f.X, f.Y
	f.X, f.Y = uint8(x), uint8(y)
	f.positionSet = false
	f.initialisePosition()
	w.linkFollower(id)
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
	w.moveMagnet(owner, x, y)
	return true
}

func (w *World) moveMagnet(owner, x, y int) {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return
	}
	w.Magnets[owner] = MagnetActor{X: x*256 + 128, Y: y*256 + 128, Owner: uint8(owner)}
	w.Actors.Move(ActorRef{Kind: ActorMagnet, Index: uint16(owner)}, w.Magnets[owner].X, w.Magnets[owner].Y)
}
