package populous2

import (
	"fmt"

	legacy "go-populous2/internal/legacy"
)

// World is the playable porting baseline. Terrain propagation, walkers, towns,
// fights and land AI are reused from Populous 1. The exact Populous II formats
// are independent of this baseline. Its new effect rules are provisional until
// checked against the original routines; see docs/PORTAGE.md.
type World struct {
	Level          Level
	Core           *legacy.World
	Landscape      Landscape
	Spells         []Spell
	ManaRules      ManaRules
	GroundRules    GroundEffectRules
	RoadRules      RoadRules
	SceneryBank    *SceneryBank
	BatholithRange int
	WallRules      WallRules
	Walls          WallState
	Scenery        [SceneryCapacity]SceneryActor
	sceneryIndex   [4096]uint16
	Experience     [2][6]uint8
	Deity          Deity
	Custom         bool
	Demo           bool
	Effects        []Effect
	Marks          [legacy.MapWidth * legacy.MapHeight]Mark
	Heroes         [legacy.MaxPeeps]Hero
	Random         uint16
	LastSpell      SpellID
	LastPlayer     int
	SpellSerial    int
	captiveIndex   [legacy.MaxPeeps]bool
}

type Effect struct {
	Spell  SpellID
	Player int
	X      int
	Y      int
	DX     int
	DY     int
	Age    int
	Life   int
}

type Mark struct {
	Spell      SpellID
	Player     int
	Life       int
	Persistent bool
	NativeTile uint8
}

type Hero struct {
	Spell      SpellID
	Active     bool
	Player     int
	Population int
	Speed      uint8
	Captives   []int
}

type Target struct{ X, Y, X2, Y2, Direction int }

func NewWorld(bundle *Bundle, levelIndex int, custom bool) (*World, error) {
	if bundle == nil || levelIndex < 0 || levelIndex >= len(bundle.Levels) {
		return nil, fmt.Errorf("world index %d outside campaign", levelIndex)
	}
	level := bundle.Levels[levelIndex]
	land := bundle.Landscapes[level.Terrain]
	rules := legacy.DefaultTerrainRules()
	for i := range rules.PopulationAdd {
		stage := i * (TownStages - 1) / (len(rules.PopulationAdd) - 1)
		rules.PopulationAdd[i] = max(0, land.PopulationAdd[stage])
		rules.ManaAdd[i] = max(0, land.ManaAdd[stage])
	}
	copy(rules.MapColor[:], land.MapColor[:16])
	// Spell AI is handled through Cast below, so the reused land AI cannot
	// silently invoke the first game's different spell set and mana prices.
	baseline := legacy.Level{Number: 0, Code: level.Code, Terrain: byte(level.Terrain), SeedOffset: level.Seed,
		PlayerPopulation: byte(level.Players[0].InitialGroups()), EnemyPopulation: byte(level.Players[1].InitialGroups()),
		EnemyRating: 10, EnemyReactionSpeed: 5, GameMode: legacy.GameWaterFatal}
	core := legacy.GenerateWorldWithRules(baseline, rules)
	core.GenerateOlympianTerrain(level.RandomSeed, bundle.HillParameters)
	townRules, err := DecodeTownRules(bundle.Executable, land)
	if err != nil {
		return nil, err
	}
	core.OlympianTowns = townRules
	w := &World{Level: level, Core: core, Landscape: land, Spells: bundle.Spells, ManaRules: bundle.ManaRules, GroundRules: bundle.GroundRules, RoadRules: bundle.RoadRules, SceneryBank: bundle.Scenery, BatholithRange: bundle.BatholithRange, WallRules: bundle.WallRules, Custom: custom, Random: level.Seed}
	w.Experience[1] = level.OpponentExperience
	w.initializeScenery()
	core.TileBlocked = func(pos int) bool { return w.sceneryAt(pos) >= 0 }
	core.HabitatBlocked = func(pos int) bool { i := w.sceneryAt(pos); return i >= 0 && w.Scenery[i].Kind == SceneryBoulder }
	var followers [2]legacy.InitialFollowers
	for player, p := range level.Players {
		followers[player] = legacy.InitialFollowers{Groups: p.InitialGroups(), Population: p.InitialPopulation(), Intelligence: p.SearchIntelligence(), Speed: p.MovementSpeed()}
	}
	core.PlaceOlympianPeople(followers)
	w.bindHeroCombat()
	w.bindWallMovement()
	return w, nil
}

func (w *World) bindWallMovement() {
	w.Core.MovementAllowed = func(index, target int, apply bool) bool {
		if index < 0 || index >= len(w.Core.Peeps) || target < 0 || target >= 4096 {
			return false
		}
		slot := w.Walls.At(target%64, target/64)
		if slot < 0 {
			return true
		}
		p := w.Core.Peeps[index]
		decision := w.WallRules.DecideCrossing(int(p.Player), int(w.Walls.Actors[slot].Player), w.Experience[p.Player][Earth], p.Population, w.Heroes[index].Active)
		if decision.Crossing == WallBlocked {
			return false
		}
		if apply && decision.Crossing == WallBreak {
			w.Walls.Break(&w.WallRules, slot)
		}
		return true
	}
}

func (w *World) Available(player int, id SpellID) bool {
	if player < 0 || player > 1 || int(id) >= 36 {
		return false
	}
	_, exists := SpellByID(w.Spells, id)
	return exists && (w.Custom || w.Level.Players[player].Powers[id])
}

func (w *World) Sculpt(player, x, y int, raise bool) bool {
	if !w.Available(player, RaiseLower) || x < 0 || y < 0 || x > legacy.MapWidth || y > legacy.MapHeight {
		return false
	}
	if !w.Core.HasBuildPresenceAt(player, clamp(x-4, 0, 56), clamp(y-4, 0, 56), 8, 8, x, y) {
		return false
	}
	cost := w.ManaCost(player, RaiseLower)
	if w.Core.Magnets[player].Mana < cost {
		return false
	}
	// Terrain propagation charges the inherited engine's terrain price once.
	// Adapt that debit without scaling the entire balance or losing odd mana.
	before := w.Core.Magnets[player].Mana
	altitudes := w.Core.Alt
	hasWalls := false
	for _, wall := range w.Walls.Actors {
		if wall.Active {
			hasWalls = true
			break
		}
	}
	var mapAlt, mapBlk, mapBk2 [4096]byte
	var mapSteps [4096]uint16
	if hasWalls {
		mapAlt, mapBlk, mapBk2, mapSteps = w.Core.MapAlt, w.Core.MapBlk, w.Core.MapBk2, w.Core.MapSteps
	}
	w.Core.Magnets[player].Mana = before - cost + legacy.ManaPointCost
	var applied bool
	if raise {
		applied = w.Core.RaiseAt(player, x, y)
	} else {
		applied = w.Core.LowerAt(player, x, y)
	}
	w.Core.Magnets[player].Mana = before
	if applied {
		// $d32a validates all propagated vertices against wall actor chains
		// before $cdca applies them. Roll back the inherited height adapter if
		// any touched tile contains a wall, leaving mana and overlays intact.
		blocked := false
		for _, wall := range w.Walls.Actors {
			if !wall.Active {
				continue
			}
			a := wall.X + wall.Y*65
			if altitudes[a] != w.Core.Alt[a] || altitudes[a+1] != w.Core.Alt[a+1] || altitudes[a+65] != w.Core.Alt[a+65] || altitudes[a+66] != w.Core.Alt[a+66] {
				blocked = true
				break
			}
		}
		if blocked {
			w.Core.Alt = altitudes
			w.Core.MapAlt, w.Core.MapBlk, w.Core.MapBk2, w.Core.MapSteps = mapAlt, mapBlk, mapBk2, mapSteps
			return false
		}
		changed := 0
		minX, minY, maxX, maxY := 65, 65, 0, 0
		for i, altitude := range w.Core.Alt {
			changed += abs(altitude - altitudes[i])
			if altitude != altitudes[i] {
				minX = min(minX, i%65)
				maxX = max(maxX, i%65)
				minY = min(minY, i/65)
				maxY = max(maxY, i/65)
			}
		}
		// $175fa charges every propagated height change and clamps at zero.
		if changed == 0 {
			return false
		}
		w.Core.Magnets[player].Mana = max(0, before-cost*changed)
		// Native point edits rewrite every touched tile's shape code. Clear
		// ground-effect overrides on all four adjacent cells, including those
		// changed indirectly by slope propagation.
		for yy := max(0, minY-1); yy <= min(63, maxY); yy++ {
			for xx := max(0, minX-1); xx <= min(63, maxX); xx++ {
				a := xx + yy*65
				if altitudes[a] != w.Core.Alt[a] || altitudes[a+1] != w.Core.Alt[a+1] || altitudes[a+65] != w.Core.Alt[a+65] || altitudes[a+66] != w.Core.Alt[a+66] {
					w.Marks[xx+yy*64] = Mark{}
				}
			}
		}
	}
	if applied && x < 64 && y < 64 {
		w.Marks[x+y*64] = Mark{}
	}
	return applied
}

func (w *World) Cast(player int, id SpellID, target Target) bool {
	spell, exists := SpellByID(w.Spells, id)
	if !exists || !w.Available(player, id) || id == RaiseLower || w.Core.War {
		return false
	}
	spell.Cost = w.ManaCost(player, id)
	if w.Core.Magnets[player].Mana < spell.Cost {
		return false
	}
	if spell.Aim != AimGlobal && spell.Aim != AimLeader && !inside(target.X, target.Y) {
		return false
	}
	if spell.Aim == AimLine && !inside(target.X2, target.Y2) {
		return false
	}
	if target.Direction < 0 || target.Direction > 7 {
		return false
	}
	if id.IsHero() {
		index := w.Core.Magnets[player].Carried - 1
		if index < 0 || index >= len(w.Core.Peeps) || w.Core.Peeps[index].Population <= 0 {
			return false
		}
		if !w.Core.PromoteHero(index) {
			return false
		}
		p := &w.Core.Peeps[index]
		population, speed, _ := HeroAttributes(id, p.Population, p.MovementSpeed, w.Experience[player])
		p.Population = population
		p.MovementSpeed = speed
		w.Heroes[index] = Hero{Spell: id, Active: true, Player: player, Population: p.Population, Speed: speed}
		w.Core.Magnets[player].Mana -= spell.Cost
		w.recordCast(player, id)
		return true
	}
	applied := false
	switch id {
	case PapalMagnet:
		index := w.Core.Magnets[player].Carried - 1
		if index < 0 || index >= len(w.Core.Peeps) || w.Core.Peeps[index].Population <= 0 {
			return false
		}
		applied = w.legacyPower(player, spell.Cost, legacy.ManaMagnetCost, func() bool { return w.Core.SetMagnetToTile(player, target.X, target.Y) })
	case Earthquake:
		applied = w.legacyPower(player, spell.Cost, legacy.ManaQuakeCost, func() bool { return w.Core.QuakeAtTile(player, target.X, target.Y) })
	case Volcano:
		applied = w.legacyPower(player, spell.Cost, legacy.ManaVolcanoCost, func() bool { return w.Core.VolcanoAtTile(player, target.X, target.Y) })
	case Armageddon:
		applied = w.legacyPower(player, spell.Cost, legacy.ManaWarCost, func() bool { return w.Core.WarPower(player) })
		if applied {
			w.removePlagueVictims()
		}
	case Plague:
		applied = w.castPlague(player, target.X+target.Y*64)
	case Swamp, Flowers, Baptism:
		applied = w.castGroundEffect(player, id, target.X, target.Y)
	case Trees:
		applied = w.plantScenery(SceneryTree, player, target.X, target.Y) > 0
	case Fungus:
		for _, p := range diskArea(target.X, target.Y, 2) {
			if w.isWaterAt(p) {
				continue
			}
			if id == Plague && !w.enemyAt(player, p) {
				continue
			}
			w.Marks[p] = Mark{Spell: id, Player: player, Life: 240}
			applied = true
		}
	case Road:
		applied = w.castRoad(player, target.X, target.Y)
	case Wall:
		applied = w.Walls.Place(&w.WallRules, player, target.X, target.Y, w.nativeTileAt)
	case Batholith:
		applied = w.castBatholith(target.X, target.Y)
	case Basalt:
		dx, dy := direction(target.Direction)
		for i := 0; i < 12; i++ {
			x, y := target.X+i*dx, target.Y+i*dy
			if !inside(x, y) {
				break
			}
			for _, vertex := range [][2]int{{x, y}, {x + 1, y}, {x, y + 1}, {x + 1, y + 1}} {
				if w.Core.Alt[vertex[0]+vertex[1]*65] == 0 {
					w.Core.PaintRaiseAt(vertex[0], vertex[1])
				}
			}
			w.Marks[x+y*64] = Mark{Spell: id, Player: player, Life: 2000}
			applied = true
		}
	case Lightning:
		applied = w.damageArea(player, target.X, target.Y, 1, 600, false)
		if applied {
			w.Effects = append(w.Effects, Effect{Spell: id, Player: player, X: target.X, Y: target.Y, Life: 4})
		}
	case Whirlwind, Storm, Wind, FireColumn, FireRain, Whirlpool, Tsunami:
		if (id == Whirlpool || id == Tsunami) && !w.isWaterAt(target.X+target.Y*64) {
			return false
		}
		dx, dy := direction(target.Direction)
		life := 32
		if id == Storm || id == FireRain {
			life = 16
		}
		w.Effects = append(w.Effects, Effect{Spell: id, Player: player, X: target.X, Y: target.Y, DX: dx, DY: dy, Life: life})
		applied = true
	}
	if !applied {
		return false
	}
	if id != PapalMagnet && id != Earthquake && id != Volcano && id != Armageddon {
		w.Core.Magnets[player].Mana -= spell.Cost
	}
	w.recordCast(player, id)
	return true
}

func (w *World) legacyPower(player, cost, oldCost int, action func() bool) bool {
	before := w.Core.Magnets[player].Mana
	w.Core.Magnets[player].Mana = before - cost + oldCost
	mode := w.Core.Computer[player].Mode
	w.Core.Computer[player].Mode |= 0x1ff
	applied := action()
	w.Core.Computer[player].Mode = mode
	w.Core.Magnets[player].Mana = before
	if applied {
		w.Core.Magnets[player].Mana -= cost
	}
	return applied
}

func (w *World) recordCast(player int, id SpellID) {
	w.LastSpell = id
	w.LastPlayer = player
	w.SpellSerial++
}

func (w *World) Tick() {
	w.rebuildCaptiveIndex()
	w.tickEffects()
	w.applyGroundEffects()
	w.spreadPlague()
	w.Core.TickWithComputer([2]bool{w.Demo, true})
	w.tickScenery()
	w.Walls.Tick(&w.WallRules, w.nativeTileAt)
	w.followHelenCaptives()
	w.applyGroundEffects()
	w.spreadPlague()
	for i, hero := range w.Heroes {
		if !hero.Active {
			continue
		}
		if i >= len(w.Core.Peeps) || w.Core.Peeps[i].Population <= 0 || int(w.Core.Peeps[i].Player) != hero.Player || w.Core.Peeps[i].Status != legacy.KnightStatus {
			w.Heroes[i] = Hero{}
			continue
		}
		p := &w.Core.Peeps[i]
		if hero.Spell == Achilles {
			w.Marks[p.AtPos] = Mark{Spell: FireColumn, Player: hero.Player, Life: 80}
			w.damageArea(hero.Player, p.AtPos%64, p.AtPos/64, 1, 50, true)
		}
		w.Heroes[i].Population = p.Population
	}
	if w.Core.GameTurn%24 == 0 {
		w.computerPower(1)
		if w.Demo {
			w.computerPower(0)
		}
	}
}

func (w *World) tickEffects() {
	for p, mark := range w.Marks {
		if mark.Life <= 0 {
			continue
		}
		if mark.Persistent {
			continue
		}
		mark.Life--
		w.Marks[p] = mark
		if mark.Life == 0 {
			if mark.Spell == Wall && w.Core.MapBk2[p] == legacy.RockBlock {
				w.Core.MapBk2[p] = 0
			}
			continue
		}
		if w.Core.GameTurn%4 != 0 {
			continue
		}
		switch mark.Spell {
		case Trees, Flowers:
			if w.friendlyTown(mark.Player, p) {
				bonus := 2
				if mark.Spell == Flowers {
					bonus = 5
				}
				w.Core.Magnets[mark.Player].Mana += bonus
			}
		case Plague:
			for i := range w.Core.Peeps {
				peep := w.Core.Peeps[i]
				if peep.Population > 0 && int(peep.Player) != mark.Player && peep.AtPos == p {
					w.Core.DamagePeep(i, max(1, peep.Population/16))
				}
			}
		case Fungus:
			w.damageArea(mark.Player, p%64, p/64, 0, 30, false)
			if w.Core.GameTurn%16 == 0 {
				q := p + []int{-64, 1, 64, -1}[w.random()%4]
				if q >= 0 && q < 4096 && distance(p, q) == 1 && !w.isWaterAt(q) && w.Marks[q].Life == 0 {
					w.Marks[q] = Mark{Spell: Fungus, Player: mark.Player, Life: 80}
				}
			}
		case FireColumn, FireRain:
			w.damageArea(mark.Player, p%64, p/64, 0, 25, true)
		}
	}
	live := w.Effects[:0]
	for _, effect := range w.Effects {
		effect.Age++
		effect.Life--
		if effect.Life <= 0 || !inside(effect.X, effect.Y) {
			continue
		}
		if effect.Age%2 == 0 {
			radius, damage := 1, 150
			if effect.Spell == Storm || effect.Spell == FireRain {
				radius, damage = 3, 100
			}
			burn := effect.Spell == FireColumn || effect.Spell == FireRain
			w.damageArea(effect.Player, effect.X, effect.Y, radius, damage, burn)
			if burn {
				for _, p := range diskArea(effect.X, effect.Y, radius) {
					if !w.isWaterAt(p) {
						w.Marks[p] = Mark{Spell: effect.Spell, Player: effect.Player, Life: 100}
						if w.Core.MapWho[p] == 0 {
							w.Core.MapBlk[p] = legacy.BadLand
						}
					}
				}
			}
			if effect.Spell == Tsunami {
				w.Core.PaintLowerAt(effect.X, effect.Y)
			}
			if effect.Spell == FireColumn || effect.Spell == Whirlwind {
				d := w.random() % 8
				effect.DX, effect.DY = direction(d)
			}
			if effect.Spell != Storm && effect.Spell != FireRain && effect.Spell != Whirlpool && effect.Spell != Lightning {
				effect.X += effect.DX
				effect.Y += effect.DY
			}
		}
		live = append(live, effect)
	}
	w.Effects = live
}

func (w *World) computerPower(player int) {
	if w.Core.War {
		return
	}
	target := -1
	population := 0
	for _, p := range w.Core.Peeps {
		if int(p.Player) != player && p.Population > population {
			target = p.AtPos
			population = p.Population
		}
	}
	if target < 0 {
		return
	}
	for _, id := range []SpellID{Armageddon, Achilles, Heracles, Perseus, Volcano, Storm, FireColumn, Earthquake, Plague, Lightning} {
		if id == Armageddon && w.Core.PlayerPopulation(player) < w.Core.PlayerPopulation(player^1)*2 {
			continue
		}
		if w.Cast(player, id, Target{X: target % 64, Y: target / 64, Direction: w.random() % 8}) {
			return
		}
	}
}

func (w *World) enemyAt(player, pos int) bool {
	for _, p := range w.Core.Peeps {
		if p.Population > 0 && int(p.Player) != player && p.AtPos == pos {
			return true
		}
	}
	return false
}
func (w *World) friendlyTown(player, pos int) bool {
	for _, p := range w.Core.Peeps {
		if p.Population > 0 && int(p.Player) == player && p.Flags == legacy.InTown && distance(pos, p.AtPos) <= 3 {
			return true
		}
	}
	return false
}
func (w *World) damageArea(player, x, y, radius, amount int, both bool) bool {
	applied := false
	for i := range w.Core.Peeps {
		p := w.Core.Peeps[i]
		if p.Population > 0 && (both || int(p.Player) != player) && distance(p.AtPos, x+y*64) <= radius {
			applied = w.Core.DamagePeep(i, amount) || applied
		}
	}
	return applied
}
func (w *World) random() int {
	w.Random = uint16(w.Core.NextRandom())
	return int(w.Random)
}
func inside(x, y int) bool    { return x >= 0 && y >= 0 && x < 64 && y < 64 }
func clamp(n, lo, hi int) int { return max(lo, min(hi, n)) }
func distance(a, b int) int   { return max(abs(a%64-b%64), abs(a/64-b/64)) }
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func direction(n int) (int, int) {
	d := [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}
	return d[n&7][0], d[n&7][1]
}
func diskArea(x, y, r int) []int {
	result := []int{}
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if inside(x+dx, y+dy) && dx*dx+dy*dy <= r*r+1 {
				result = append(result, x+dx+(y+dy)*64)
			}
		}
	}
	return result
}
func line(x0, y0, x1, y1 int) []int {
	result := []int{}
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		result = append(result, x0+y0*64)
		if x0 == x1 && y0 == y1 {
			return result
		}
		e := err * 2
		if e >= dy {
			err += dy
			x0 += sx
		}
		if e <= dx {
			err += dx
			y0 += sy
		}
	}
}
