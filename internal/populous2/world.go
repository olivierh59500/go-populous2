package populous2

import (
	"fmt"

	legacy "go-populous2/internal/legacy"
)

// World composes native Populous II routines and retained records with the
// supplied Go terrain/presentation baseline. Remaining inherited AI, menus,
// scoring and incomplete power paths are tracked in docs/FEATURES.md.
type World struct {
	Level                    Level
	Rules                    [2]ScenarioRules
	Core                     *legacy.World
	Landscape                Landscape
	Spells                   []Spell
	ManaRules                ManaRules
	GroundRules              GroundEffectRules
	NativeGround             NativeGroundRules
	RoadRules                RoadRules
	SceneryBank              *SceneryBank
	BatholithRange           int
	WallRules                WallRules
	FireColumns              FireColumnRules
	NativeFireColumn         NativeFireColumnRules
	Whirlwinds               WhirlwindRules
	NativeWhirlwind          NativeWhirlwindRules
	WhirlwindFollower        WhirlwindFollowerRules
	Whirlpools               WhirlpoolRules
	BasaltRules              BasaltRules
	BasaltState              BasaltState
	LightningRules           LightningRules
	LightningState           LightningState
	LightningVictims         [legacy.MaxPeeps]NativeLightningFollower
	FungusRules              FungusRules
	NativeFungus             NativeFungusRules
	FungusHazards            FungusHazardRules
	FungusState              FungusState
	FollowerMotion           FollowerMotionRules
	FollowerDecision         FollowerDecisionRules
	TownEvaluator            NativeTownEvaluator
	MagnetRules              NativeMagnetRules
	FollowerEntry            FollowerEntryRules
	FollowerCombat           FollowerCombatRules
	TownCombat               TownCombatRules
	FollowerAftermath        FollowerAftermathRules
	CommonPrepass            CommonPrepassRules
	FollowerHero             FollowerHeroRules
	FollowerTerrain          FollowerTerrainRules
	FollowerMagnet           FollowerMagnetRules
	FollowerRuin             FollowerRuinVictimRules
	FollowerCrossing         FollowerCrossingRules
	PrimitiveCreators        NativePrimitiveCreatorRules
	NeutralRules             NativeNeutralRules
	TownEconomy              NativeTownEconomyRules
	NativeCreatureDeadline   uint32
	EarthquakeRules          EarthquakeRules
	VolcanoRules             VolcanoRules
	LavaRules                NativeLavaRules
	NativeEnvironment        [NativeEffectCapacity]NativeEnvironmentController
	NativeEnvironmentDirty   uint16
	NativeEnvironmentShake   uint16
	NativeCommandBytes       [0x78]byte  // BSS $eb18..$eb90, including command aliases.
	NativeControlBytes       [0x180]byte // BSS $dc4..$f44, script/scratch/control aliases.
	StormRules               StormRules
	FireRainRules            FireRainRules
	HurricaneRules           HurricaneRules
	TsunamiRules             TsunamiRules
	PlagueRules              PlagueRules
	ArmageddonRules          ArmageddonRules
	ForestNative             ForestNativeRules
	RenewNative              RenewNativeRules
	CampaignResult           CampaignResultRules
	NativeGameMode           uint16
	NativeProfileSide        uint8
	NativeClock              uint32
	NativeFreeCommands       uint16
	NativeResult             NativeGameResult
	HeroArt                  HeroRules
	FollowerWin              FollowerWinRules
	NativeBirthBlocked       bool
	NativeRaiseEnabled       uint16
	NativeGlobals            NativeGlobalImage
	NativeSelected           NativeRecordReference
	nativeCallDepth          int
	nativeEntryCrossing      func(int) FollowerEntryStep
	NativeOverlays           [4096]uint8
	NativeViewBytes          [12]byte // BSS $5f44..$5f50, camera and pointer/hit-test words.
	NativeFollowers          [legacy.MaxPeeps]NativeFollower
	NativeEffects            [NativeEffectCapacity]NativeEffectActor
	Occupancy                NativeWorldOccupancy
	RecordImage              NativeRecordImage
	NativeEntries            [legacy.MaxPeeps]NativeFollowerEntry
	effectViewX, effectViewY int
	effectSoundCues          []int
	FlameDeaths              []FlameDeath
	flameDeathIndex          [legacy.MaxPeeps]bool
	Walls                    WallState
	Scenery                  [SceneryCapacity]SceneryActor
	sceneryIndex             [4096]uint16
	Experience               [2][6]uint8
	Deity                    Deity
	Custom                   bool
	Demo                     bool
	Effects                  []Effect
	Marks                    [legacy.MapWidth * legacy.MapHeight]Mark
	Heroes                   [legacy.MaxPeeps]Hero
	Random                   uint16
	LastSpell                SpellID
	LastPlayer               int
	SpellSerial              int
	HazardSerial             int
	LastHazardCue            int
	captiveIndex             [legacy.MaxPeeps]bool
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
	Spell           SpellID
	Player          int
	Life            int
	Persistent      bool
	NativeTile      uint8
	NativeCodeValid bool
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
	w := &World{Level: level, Core: core, Landscape: land, Spells: bundle.Spells, ManaRules: bundle.ManaRules, GroundRules: bundle.GroundRules, RoadRules: bundle.RoadRules, SceneryBank: bundle.Scenery, BatholithRange: bundle.BatholithRange, WallRules: bundle.WallRules, FireColumns: bundle.FireColumns, Whirlwinds: bundle.Whirlwinds, Custom: custom, Random: level.Seed}
	w.Experience[1] = level.OpponentExperience
	w.NativeGround = bundle.NativeGround
	w.Whirlpools, w.BasaltRules = bundle.Whirlpools, bundle.BasaltRules
	w.NativeFireColumn = bundle.NativeFireColumn
	w.NativeWhirlwind, w.WhirlwindFollower = bundle.NativeWhirlwind, bundle.WhirlwindFollower
	w.LightningRules = bundle.LightningRules
	w.FungusRules = bundle.FungusRules
	w.NativeFungus = bundle.NativeFungus
	w.FungusHazards = bundle.FungusHazards
	w.FollowerMotion = bundle.FollowerMotion
	w.FollowerDecision = bundle.FollowerDecision
	w.TownEvaluator = bundle.TownEvaluator
	w.MagnetRules = bundle.MagnetRules
	w.FollowerEntry, w.FollowerCombat, w.TownCombat, w.FollowerAftermath = bundle.FollowerEntry, bundle.FollowerCombat, bundle.TownCombat, bundle.FollowerAftermath
	w.CommonPrepass = bundle.CommonPrepass
	w.FollowerHero = bundle.FollowerHero
	w.FollowerTerrain = bundle.FollowerTerrain
	w.FollowerMagnet = bundle.FollowerMagnet
	w.FollowerRuin = bundle.FollowerRuin
	w.FollowerCrossing = bundle.FollowerCrossing
	w.PrimitiveCreators, w.NeutralRules = bundle.PrimitiveCreators, bundle.NeutralRules
	w.EarthquakeRules = bundle.EarthquakeRules
	w.VolcanoRules, w.LavaRules = bundle.VolcanoRules, bundle.LavaRules
	w.StormRules = bundle.StormRules
	w.FireRainRules = bundle.FireRainRules
	w.HurricaneRules, w.TsunamiRules = bundle.HurricaneRules, bundle.TsunamiRules
	w.PlagueRules, w.ArmageddonRules = bundle.PlagueRules, bundle.ArmageddonRules
	w.ForestNative, w.RenewNative = bundle.ForestNative, bundle.RenewNative
	w.CampaignResult = bundle.CampaignResult
	w.NativeGameMode, w.NativeProfileSide = 2, 1
	if custom {
		w.NativeGameMode = 4
	}
	w.TownEconomy, err = DecodeNativeTownEconomyRules(land)
	if err != nil {
		return nil, err
	}
	w.HeroArt = bundle.HeroRules
	w.FollowerWin, err = DecodeFollowerWinRules(bundle.Executable, land)
	if err != nil {
		return nil, err
	}
	for player, p := range level.Players {
		w.Rules[player] = p.ScenarioRules()
	}
	w.bindScenarioRuntime()
	w.initializeActorGraph()
	w.initializeScenery()
	core.TileBlocked = func(pos int) bool { return w.sceneryAt(pos) >= 0 }
	core.HabitatBlocked = func(pos int) bool { i := w.sceneryAt(pos); return i >= 0 && w.Scenery[i].Kind == SceneryBoulder }
	w.bindHabitatTerrain()
	var followers [2]legacy.InitialFollowers
	for player, p := range level.Players {
		followers[player] = legacy.InitialFollowers{Groups: p.InitialGroups(), Population: p.InitialPopulation(), SearchIndex: p.InitialSearchIndex(), Weapons: p.InitialWeapons(), Speed: p.MovementSpeed()}
	}
	w.bindFollowerMotion()
	core.PlaceOlympianPeople(followers)
	w.initializeScenarioBalances()
	w.bindHeroCombat()
	w.bindWallMovement()
	w.bindFlameDeaths()
	w.bindFollowerHazards()
	w.bindLightningVictims()
	w.bindActorGraphHooks()
	w.bindNativeTownEvaluator()
	w.initializeNativeRuntime()
	w.initializeNativeCampaignStatistics()
	if err := LoadScenarioScript(w.Level.WorldParameters, w.nativeCleanupMemory()); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *World) bindHabitatTerrain() {
	w.Core.HabitatTerrainAllowed = func(player, pos int) bool {
		if pos < 0 || pos >= 4096 || player < 0 || player > 1 {
			return false
		}
		property := w.GroundRules.Properties[w.nativeTileAt(pos%64, pos/64)]
		if property&0x17 == 0 || property&0x10 != 0 {
			return false
		}
		return property&(1<<uint((player^1)+1)) == 0
	}
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
	if !w.Rules[player].TerrainEditAllowed(w.Core.Alt[x+y*65], raise) {
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
	var protectedFarms [4096]bool
	if w.Rules[player].ForbidEnemyTerrain {
		enemy := uint8(47 + 16*(player^1))
		for pos := range protectedFarms {
			protectedFarms[pos] = w.nativeTileAt(pos%64, pos/64) == enemy
		}
	}
	hasWalls := false
	for _, wall := range w.Walls.Actors {
		if wall.Active {
			hasWalls = true
			break
		}
	}
	var mapAlt, mapBlk, mapBk2 [4096]byte
	var mapSteps [4096]uint16
	if hasWalls || w.Rules[player].ForbidEnemyTerrain {
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
		if !blocked && w.Rules[player].ForbidEnemyTerrain {
			for pos, protected := range protectedFarms {
				if !protected {
					continue
				}
				a := pos%64 + (pos/64)*65
				if altitudes[a] != w.Core.Alt[a] || altitudes[a+1] != w.Core.Alt[a+1] || altitudes[a+65] != w.Core.Alt[a+65] || altitudes[a+66] != w.Core.Alt[a+66] {
					blocked = true
					break
				}
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
					mark := w.Marks[xx+yy*64]
					if mark.NativeTile&0xf0 != 0xe0 {
						w.Marks[xx+yy*64] = Mark{}
					}
				}
			}
		}
	}
	if applied && x < 64 && y < 64 {
		mark := w.Marks[x+y*64]
		if mark.NativeTile&0xf0 != 0xe0 {
			w.Marks[x+y*64] = Mark{}
		}
	}
	if applied {
		w.refreshChangedTerrain(altitudes)
	}
	return applied
}

func (w *World) Cast(player int, id SpellID, target Target) bool {
	spell, exists := SpellByID(w.Spells, id)
	if !exists || !w.Available(player, id) || id == RaiseLower || w.Core.War {
		return false
	}
	if id == Lightning {
		return w.PlaceLightning(player, target.X, target.Y)
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
	if id == Wind && target.Direction&1 != 0 {
		return false
	}
	if id.IsHero() {
		created := false
		err := w.runNativeFollowerCall(func() error {
			step, err := w.HeroArt.Create(id, uint16(player+1), HeroCreationCallbacks{Memory: w.nativeCleanupMemory(), ClearLeader: w.clearNativeLeader, ClearFarms: w.clearNativeFarms, Sound: w.nativeEntryCallbacks().Sound})
			created = step.Created
			return err
		})
		if err != nil {
			panic(err)
		}
		if !created {
			return false
		}
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
		w.castNativeEarthquake(player, target.X, target.Y, uint8(target.Direction))
		applied = true
	case Volcano:
		w.castNativeVolcano(player, target.X, target.Y)
		applied = true
	case Armageddon:
		w.castNativeArmageddon()
		applied = true
	case Plague:
		applied = w.castPlague(player, target.X+target.Y*64)
	case Swamp, Baptism:
		applied = w.castGroundEffect(player, id, target.X, target.Y)
	case Flowers:
		w.castNativeRenew(player, target.X, target.Y)
		applied = true
	case Trees:
		applied = w.castNativeForest(player, target.X, target.Y)
	case Fungus:
		w.castFungus(player, target.X, target.Y)
		// The native command handler consumes the cast even when planting
		// or shared-pool allocation has no effect.
		applied = true
	case Road:
		applied = w.castRoad(player, target.X, target.Y)
	case Wall:
		slot := -1
		for index := range w.Walls.Actors {
			if !w.Walls.Actors[index].Active {
				slot = index
				break
			}
		}
		applied = w.Walls.Place(&w.WallRules, player, target.X, target.Y, w.nativeTileAt)
		if applied && slot >= 0 {
			w.placeActor(NativeWallPool, slot, uint16(target.X*256+128), uint16(target.Y*256+128))
		}
	case Batholith:
		applied = w.castBatholith(target.X, target.Y)
	case Basalt:
		applied = w.castBasalt(player, target.X, target.Y, target.Direction)
	case FireColumn:
		applied = w.castFireColumn(player, target.X, target.Y)
	case Whirlwind:
		applied = w.castWhirlwind(player, target.X, target.Y)
	case Whirlpool:
		applied = w.castWhirlpool(player, target.X, target.Y)
	case Storm:
		applied = w.castNativeStorm(player, target.X, target.Y)
	case FireRain:
		applied = w.castNativeFireRain(player, target.X, target.Y)
	case Wind:
		applied = w.castNativeHurricane(player, target.X, target.Y, uint16(target.Direction))
	case Tsunami:
		w.castNativeTsunami(player, target.X, target.Y)
		applied = true
	}
	if !applied {
		return false
	}
	if id != PapalMagnet {
		w.Core.Magnets[player].Mana -= spell.Cost
	}
	w.recordCast(player, id)
	return true
}

func (w *World) legacyPower(player, cost, oldCost int, action func() bool) bool {
	altitudes := w.Core.Alt
	before := w.Core.Magnets[player].Mana
	w.Core.Magnets[player].Mana = before - cost + oldCost
	mode := w.Core.Computer[player].Mode
	w.Core.Computer[player].Mode |= 0x1ff
	applied := action()
	w.Core.Computer[player].Mode = mode
	w.Core.Magnets[player].Mana = before
	if applied {
		w.Core.Magnets[player].Mana -= cost
		w.clearChangedGround(altitudes)
		w.refreshChangedTerrain(altitudes)
	}
	return applied
}

func (w *World) recordCast(player int, id SpellID) {
	w.recordNativePowerUse(player, id)
	w.LastSpell = id
	w.LastPlayer = player
	w.SpellSerial++
}

func (w *World) Tick() {
	if w.NativeResult.Detected {
		return
	}
	w.NativeClock++
	w.NativeBirthBlocked = false
	w.reconcileActorGraph()
	w.syncNativeRuntimeBridge()
	w.refreshNativeRecordImage()
	w.rebuildCaptiveIndex()
	w.tickEffects()
	w.Core.TickWithComputer([2]bool{w.Demo, true})
	w.detectNativeResult()
	if w.NativeResult.Detected {
		w.reconcileActorGraph()
		w.refreshNativeRecordImage()
		return
	}
	w.reconcileActorGraph()
	w.syncNativeRuntimeBridge()
	w.tickFlameDeaths()
	w.tickNativeEffects()
	w.Walls.Tick(&w.WallRules, w.nativeTileAt)
	w.tickScenery()
	if err := w.tickNativeScenarioScript(); err != nil {
		panic(err)
	}
	w.followHelenCaptives()
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
			w.burnFireCell(p.AtPos%64, p.AtPos/64)
		}
		w.Heroes[i].Population = p.Population
	}
	if w.Core.GameTurn%24 == 0 {
		w.computerPower(1)
		if w.Demo {
			w.computerPower(0)
		}
	}
	w.reconcileActorGraph()
	w.refreshNativeRecordImage()
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
			burn := effect.Spell == FireRain
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
				before := w.Core.Alt
				if w.Core.PaintLowerAt(effect.X, effect.Y) {
					w.clearChangedGround(before)
					w.refreshChangedTerrain(before)
				}
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
			if id == Lightning {
				w.ActivateLightning(player)
			}
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
	seed := w.Core.RandomState()
	for i := range 4 {
		w.NativeCommandBytes[0xeb28-0xeb18+i] = uint8(seed >> uint(24-i*8))
	}
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
