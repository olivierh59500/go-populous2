package engine

import (
	"encoding/json"
	"fmt"
	"io"
)

const SnapshotVersion = 2
const maximumSnapshotBytes = 16 << 20

// SavedWorld contains the World's exported, named simulation fields. Private
// motion, random state and allocation bookkeeping are recorded explicitly in
// Snapshot rather than disappearing during ordinary JSON serialization.
type SavedWorld World

type FollowerMotionSnapshot struct {
	PositionX    int  `json:"positionX,omitempty"`
	PositionY    int  `json:"positionY,omitempty"`
	VelocityX    int  `json:"velocityX,omitempty"`
	VelocityY    int  `json:"velocityY,omitempty"`
	LegRemaining int  `json:"legRemaining,omitempty"`
	PositionSet  bool `json:"positionSet,omitempty"`
	Moving       bool `json:"moving,omitempty"`
}

type ConstructionSnapshot struct {
	TerrainTargets       [MapSize * MapSize]uint8       `json:"terrainTargets"`
	TerrainTargetSet     [MapSize * MapSize]bool        `json:"terrainTargetSet"`
	DevelopmentTargets   [CornerSize * CornerSize]uint8 `json:"developmentTargets"`
	DevelopmentTargetSet [CornerSize * CornerSize]bool  `json:"developmentTargetSet"`
}

// Snapshot is a versioned semantic save format. Indices identify game actors
// and geometric cells; no executable bytes or machine-address aliases appear.
type Snapshot struct {
	Version      int                                      `json:"version"`
	World        SavedWorld                               `json:"world"`
	Motion       [FollowerCapacity]FollowerMotionSnapshot `json:"motion"`
	Random       uint32                                   `json:"random"`
	Reservations [EffectCapacity]EffectReservation        `json:"reservations"`
	// Construction is accepted when migrating version-one saves. The retired
	// AI construction cache is discarded; version-two saves omit it. Active
	// world state, AI state, motion and random state are preserved unchanged.
	Construction *ConstructionSnapshot `json:"construction,omitempty"`
}

func (w *World) Snapshot() Snapshot {
	if w == nil {
		return Snapshot{Version: SnapshotVersion}
	}
	s := Snapshot{Version: SnapshotVersion, World: SavedWorld(*w), Random: uint32(w.random), Reservations: w.effects.Slots}
	for id, f := range w.Followers {
		s.Motion[id] = FollowerMotionSnapshot{f.positionX, f.positionY, f.velocityX, f.velocityY, f.legRemaining, f.positionSet, f.moving}
	}
	return s
}

func WriteSnapshot(output io.Writer, world *World) error {
	if output == nil || world == nil {
		return fmt.Errorf("snapshot writer or world is missing")
	}
	s := world.Snapshot()
	if _, err := s.Restore(); err != nil {
		return fmt.Errorf("cannot save invalid world: %w", err)
	}
	encoder := json.NewEncoder(output)
	return encoder.Encode(s)
}

// ReadSnapshot validates a detached candidate before returning it. Callers can
// replace their live world only after success, so failed loads cannot mutate a
// running game or partially apply its random and allocation state.
func ReadSnapshot(input io.Reader) (*World, error) {
	if input == nil {
		return nil, fmt.Errorf("snapshot reader is missing")
	}
	limited := &io.LimitedReader{R: input, N: maximumSnapshotBytes + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("invalid snapshot JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("snapshot contains trailing data")
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("snapshot exceeds the size limit")
	}
	return snapshot.Restore()
}

func (s Snapshot) Restore() (*World, error) {
	if s.Version != 1 && s.Version != SnapshotVersion {
		return nil, fmt.Errorf("unsupported snapshot version %d", s.Version)
	}
	if s.Version == SnapshotVersion && s.Construction != nil {
		return nil, fmt.Errorf("version-two snapshot contains a retired construction cache")
	}
	w := World(s.World)
	w.random = randomState(s.Random)
	w.effects.Slots = s.Reservations
	for id, motion := range s.Motion {
		f := &w.Followers[id]
		f.positionX, f.positionY = motion.PositionX, motion.PositionY
		f.velocityX, f.velocityY = motion.VelocityX, motion.VelocityY
		f.legRemaining, f.positionSet, f.moving = motion.LegRemaining, motion.PositionSet, motion.Moving
	}
	if err := validateSnapshotWorld(&w); err != nil {
		return nil, err
	}
	return &w, nil
}

func validateSnapshotWorld(w *World) error {
	if err := w.SelectionTransfers.validate(); err != nil {
		return err
	}
	if err := validateActorRegistry(&w.Actors); err != nil {
		return err
	}
	if err := validateLiveActorRegistry(w); err != nil {
		return err
	}
	if int(w.Scenario.Cursor) > len(w.Scenario.Events) {
		return fmt.Errorf("snapshot scenario cursor is invalid")
	}
	for _, event := range w.Scenario.Events {
		if event.Kind > ScenarioMonster || event.Direction > 3 || event.Time != 0 && !inside(int(event.X), int(event.Y)) {
			return fmt.Errorf("snapshot scenario event is invalid")
		}
	}
	if w.Result < 0 || w.Result > 2 || w.Level.Number < 0 || w.Level.Number >= 1000 || w.Level.Landscape < 0 || w.Level.Landscape > 3 {
		return fmt.Errorf("snapshot campaign state is invalid")
	}
	for stage := 1; stage < TownStages; stage++ {
		if w.Landscape.WorkTicks[stage] < 1 || w.Landscape.EmigrationDivisor[stage] < 1 {
			return fmt.Errorf("snapshot economy stage %d is invalid", stage)
		}
	}
	for vertex, height := range w.Heights {
		if height > 8 {
			return fmt.Errorf("snapshot terrain vertex %d exceeds height eight", vertex)
		}
	}
	for at, cell := range w.Tiles {
		if cell.BaseAltitude > 8 || cell.Shape > 15 || w.Farms[at] > 2 {
			return fmt.Errorf("snapshot parcel %d is invalid", at)
		}
		for _, height := range cell.Corners {
			if height > 8 {
				return fmt.Errorf("snapshot parcel %d has an invalid corner", at)
			}
		}
	}
	for owner, player := range w.Players {
		request := w.AI[owner]
		if request.WaterRequestFollower < 0 || request.WaterRequestFollower >= FollowerCapacity {
			return fmt.Errorf("snapshot water request follower is invalid")
		}
		if request.TerrainRequestFollower < 0 || request.TerrainRequestFollower >= FollowerCapacity || request.TerrainRequestFollower != 0 && !inside(request.TerrainRequestX, request.TerrainRequestY) {
			return fmt.Errorf("snapshot crossing terrain request is invalid")
		}
		if player.Mode > Fight || !inside(player.RallyX, player.RallyY) || player.Leader < 0 || player.Leader >= FollowerCapacity {
			return fmt.Errorf("snapshot player %d is invalid", owner)
		}
		if player.Leader != 0 && (w.Followers[player.Leader].State == Inactive || int(w.Followers[player.Leader].Owner) != owner) {
			return fmt.Errorf("snapshot leader does not identify a living owned follower")
		}
	}
	for id, f := range w.Followers {
		if f.State == Inactive {
			continue
		}
		validOwner := f.Owner < 2 || f.Owner == 2 && f.Neutral.Kind > NeutralNone && f.Neutral.Kind <= NeutralMonster
		if id == 0 || !validOwner || f.Neutral.Kind > NeutralMonster || f.Neutral.VictimTime < 0 || f.Neutral.VictimTime > 400 || f.State > Converting || !inside(int(f.X), int(f.Y)) || f.Stage >= TownStages || f.Hero.Kind > HeroHelen || f.Hero.Phase > HeroDying || f.NextFollower < 0 || f.NextFollower >= FollowerCapacity || f.PreviousFollower < 0 || f.PreviousFollower >= FollowerCapacity || f.BattleWith < 0 || f.BattleWith >= FollowerCapacity || f.ContactWith < 0 || f.ContactWith >= FollowerCapacity || f.Hero.Target < 0 || f.Hero.Target >= FollowerCapacity || f.Hero.ClaimedBy < 0 || f.Hero.ClaimedBy >= FollowerCapacity || f.Hero.CaptiveOf < 0 || f.Hero.CaptiveOf >= FollowerCapacity {
			return fmt.Errorf("snapshot follower %d contains an invalid state or relation", id)
		}
		if f.Conversion.Active && (f.Conversion.SourceOwner > 1 || f.Conversion.Frame >= 12) {
			return fmt.Errorf("snapshot follower %d conversion is invalid", id)
		}
		if f.AppearanceVariant > 7 {
			return fmt.Errorf("snapshot follower appearance is invalid")
		}
		if f.ContactWaiting && (f.ContactWait < -32768 || f.ContactWait > 32767) {
			return fmt.Errorf("snapshot contact waiting timer is invalid")
		}
		if f.TerrainDeath.Active && (f.TerrainDeath.Frames < 1 || f.TerrainDeath.Frame >= f.TerrainDeath.Frames) {
			return fmt.Errorf("snapshot terrain-death lifecycle is invalid")
		}
		aftermath := f.CombatAftermath
		if aftermath.Kind > CombatCollateralDeath {
			return fmt.Errorf("snapshot combat aftermath kind is invalid")
		}
		if aftermath.Kind == CombatTownRuin {
			if aftermath.Frame != 0 || aftermath.Frames != 0 || aftermath.RuinTime < 1 || aftermath.RuinTime > 400 {
				return fmt.Errorf("snapshot town ruin countdown is invalid")
			}
		} else if aftermath.Kind != CombatAftermathNone && (aftermath.Frames < 1 || aftermath.Frame >= aftermath.Frames || aftermath.RuinTime < 0 || aftermath.RuinTime > 400) {
			return fmt.Errorf("snapshot combat aftermath animation is invalid")
		}
		if f.positionSet && (f.positionX < 0 || f.positionY < 0 || f.positionX >= MapSize*256 || f.positionY >= MapSize*256 || f.positionX>>8 != int(f.X) || f.positionY>>8 != int(f.Y)) || f.velocityX < -32768 || f.velocityX > 32767 || f.velocityY < -32768 || f.velocityY > 32767 || f.legRemaining < -32768 || f.legRemaining > 32767 {
			return fmt.Errorf("snapshot follower %d contains invalid continuous motion", id)
		}
	}
	var seen [FollowerCapacity]bool
	for at, head := range w.Occupants {
		previous := 0
		for current, visits := int(head), 0; current != 0; visits++ {
			if current <= 0 || current >= FollowerCapacity || visits >= FollowerCapacity || seen[current] {
				return fmt.Errorf("snapshot occupancy chain at parcel %d is invalid", at)
			}
			f := w.Followers[current]
			if f.State == Inactive || int(f.X)+int(f.Y)*MapSize != at || f.PreviousFollower != previous {
				return fmt.Errorf("snapshot occupancy membership is inconsistent at follower %d", current)
			}
			seen[current] = true
			previous, current = current, f.NextFollower
		}
	}
	for id, f := range w.Followers {
		if f.State != Inactive && !seen[id] {
			return fmt.Errorf("snapshot living follower %d is absent from occupancy", id)
		}
	}
	for id, reservation := range w.effects.Slots {
		if reservation.Kind > EffectPlague || reservation.Owner > 2 {
			return fmt.Errorf("snapshot effect reservation %d is invalid", id)
		}
		if reservation.Kind == EffectNone {
			if w.Fire.Columns[id].Active || w.Fire.Rain[id].Active || w.Fire.Volcano[id].Active || w.Fire.Lava[id].Active || w.Nature.Fungi[id].Active || w.Air.Markers[id].Active || w.Air.Bolts[id].Active || w.Air.Whirlwinds[id].Active || w.Air.Storms[id].Active || w.Water.Basalt[id].Active || w.Water.Whirlpools[id].Active || w.Water.Waves[id].Active || w.Wind[id].Active {
				return fmt.Errorf("snapshot active effect %d has no reservation", id)
			}
			continue
		}
		switch reservation.Kind {
		case EffectFireColumn:
			if !w.Fire.Columns[id].Active {
				return fmt.Errorf("snapshot fire column reservation has no controller")
			}
		case EffectFireRain:
			if !w.Fire.Rain[id].Active {
				return fmt.Errorf("snapshot meteor reservation has no controller")
			}
		case EffectVolcano:
			if !w.Fire.Volcano[id].Active {
				return fmt.Errorf("snapshot volcano reservation has no controller")
			}
		case EffectLava:
			if !w.Fire.Lava[id].Active {
				return fmt.Errorf("snapshot lava reservation has no controller")
			}
		case EffectFungus:
			if !w.Nature.Fungi[id].Active {
				return fmt.Errorf("snapshot fungus reservation has no controller")
			}
		case EffectLightning:
			if !w.Air.Markers[id].Active && !w.Air.Bolts[id].Active {
				return fmt.Errorf("snapshot lightning reservation has no controller")
			}
		case EffectWhirlwind:
			if !w.Air.Whirlwinds[id].Active {
				return fmt.Errorf("snapshot whirlwind reservation has no controller")
			}
		case EffectStorm:
			if !w.Air.Storms[id].Active {
				return fmt.Errorf("snapshot storm reservation has no controller")
			}
		case EffectBasalt:
			if !w.Water.Basalt[id].Active {
				return fmt.Errorf("snapshot basalt reservation has no controller")
			}
		case EffectWhirlpool:
			if !w.Water.Whirlpools[id].Active {
				return fmt.Errorf("snapshot whirlpool reservation has no controller")
			}
		case EffectTidalWave:
			if !w.Water.Waves[id].Active {
				return fmt.Errorf("snapshot tidal-wave reservation has no controller")
			}
		case EffectHurricane:
			if !w.Wind[id].Active {
				return fmt.Errorf("snapshot wind reservation has no controller")
			}
		}
		for _, effect := range []FireEffect{w.Fire.Columns[id], w.Fire.Rain[id], w.Fire.Volcano[id], w.Fire.Lava[id]} {
			if effect.Active && (effect.Owner > 2 || effect.Phase > LavaFlowing || effect.Stage < 0 || effect.Stage > 8 || effect.Direction < 0 || effect.Direction > 3 || effect.X < 0 || effect.Y < 0 || effect.X >= MapSize*256 || effect.Y >= MapSize*256) {
				return fmt.Errorf("snapshot fire controller %d is invalid", id)
			}
		}
		if fungus := w.Nature.Fungi[id]; fungus.Active && (fungus.Owner > 1 || fungus.Period < 3 || fungus.Period > 10) {
			return fmt.Errorf("snapshot fungus period is invalid")
		}
		if marker := w.Air.Markers[id]; marker.Active && (marker.Owner > 1 || marker.Phase > LightningDisappearing || marker.FirstBolt < 0 || marker.FirstBolt > EffectCapacity || marker.Frame < 0 || marker.Frame >= 9) {
			return fmt.Errorf("snapshot lightning marker is invalid")
		}
		if bolt := w.Air.Bolts[id]; bolt.Active && (bolt.Owner > 2 || bolt.Marker < 1 || bolt.Marker > EffectCapacity || bolt.Next < 0 || bolt.Next > EffectCapacity) {
			return fmt.Errorf("snapshot lightning bolt is invalid")
		}
		if wind := w.Air.Whirlwinds[id]; wind.Active && (wind.Owner > 2 || wind.Phase > WhirlwindDisappearing || wind.Frame < 0 || wind.Frame >= 4) {
			return fmt.Errorf("snapshot whirlwind phase is invalid")
		}
		if basalt := w.Water.Basalt[id]; basalt.Active && (basalt.Owner > 2 || !inside(basalt.X, basalt.Y) || basalt.Direction < 0 || basalt.Direction > 3) {
			return fmt.Errorf("snapshot basalt direction is invalid")
		}
		if wave := w.Water.Waves[id]; wave.Active && (wave.Owner > 2 || wave.Direction < 0 || wave.Direction > 3) {
			return fmt.Errorf("snapshot tidal-wave direction is invalid")
		}
		if wind := w.Wind[id]; wind.Active && (wind.Owner > 2 || wind.Direction < 0 || wind.Direction > 3 || wind.Life < -32768 || wind.Life > 32767) {
			return fmt.Errorf("snapshot wind controller is invalid")
		}
	}
	for _, reference := range w.Air.MarkerSlots {
		if reference < 0 || reference > EffectCapacity || reference != 0 && !w.Air.Markers[reference-1].Active {
			return fmt.Errorf("snapshot lightning marker relation is invalid")
		}
	}
	for id, carry := range w.Air.Carry {
		if carry.Phase > AirCarryLanding || carry.Phase != AirCarryNone && (id == 0 || w.Followers[id].State == Inactive || carry.Effect < 1 || carry.Effect > EffectCapacity || carry.Frames < 1 || carry.Frame < 0 || carry.Frame >= carry.Frames) {
			return fmt.Errorf("snapshot carried follower %d is invalid", id)
		}
		victim := w.AirVictims[id]
		if victim.Phase > LightningVictimRecovery || victim.Bolt < 0 || victim.Bolt > EffectCapacity || victim.Hero > HeroHelen || victim.Phase != LightningVictimNone && (victim.Frames < 1 || victim.Frame < 0 || victim.Frame >= victim.Frames) {
			return fmt.Errorf("snapshot lightning victim %d is invalid", id)
		}
		death := w.FireDamage.Deaths[id]
		if death.Mode > FireVictimBurning || death.Mode != FireVictimAlive && (death.Frames < 1 || death.Frame < 0 || death.Frame >= death.Frames || death.TownStage >= TownStages) {
			return fmt.Errorf("snapshot fire victim %d is invalid", id)
		}
	}
	return nil
}

// validateActorRegistry verifies every typed chain and reciprocal link. The
// total visit bound comes from semantic pool capacities, not record lengths.
func validateActorRegistry(registry *ActorRegistry) error {
	if registry == nil {
		return fmt.Errorf("snapshot actor registry is missing")
	}
	seen := make(map[ActorRef]bool, FollowerCapacity+EffectCapacity+SceneryCapacity+WallCapacity+2)
	const maximumActors = FollowerCapacity + EffectCapacity + SceneryCapacity + WallCapacity + 2
	validEmpty := func(ref ActorRef) bool { return ref.Kind == ActorNone && ref.Index == 0 }
	for at, head := range registry.Heads {
		previous := ActorRef{}
		for current, visits := head, 0; current.Kind != ActorNone; visits++ {
			if visits >= maximumActors || seen[current] {
				return fmt.Errorf("snapshot actor chain at parcel %d is cyclic", at)
			}
			node := registry.node(current)
			if node == nil || !node.Linked || node.Previous != previous {
				return fmt.Errorf("snapshot actor reference or reciprocal link is invalid")
			}
			cell, inside := actorCell(node.X, node.Y)
			if !inside || cell != at {
				return fmt.Errorf("snapshot actor is linked to the wrong parcel")
			}
			if node.Next.Kind == ActorNone && !validEmpty(node.Next) {
				return fmt.Errorf("snapshot actor chain has an invalid terminal reference")
			}
			seen[current] = true
			previous, current = current, node.Next
		}
		if head.Kind == ActorNone && !validEmpty(head) {
			return fmt.Errorf("snapshot null actor reference has a nonzero index")
		}
	}
	for kind, size := range map[ActorKind]int{ActorFollower: FollowerCapacity, ActorEffect: EffectCapacity, ActorScenery: SceneryCapacity, ActorWall: WallCapacity, ActorMagnet: 2} {
		for index := 0; index < size; index++ {
			if kind == ActorFollower && index == 0 {
				continue
			}
			ref := ActorRef{Kind: kind, Index: uint16(index)}
			node := registry.node(ref)
			if node.Linked && !seen[ref] {
				return fmt.Errorf("snapshot linked actor is absent from its parcel chain")
			}
			if !node.Linked && (!validEmpty(node.Next) || !validEmpty(node.Previous)) {
				return fmt.Errorf("snapshot detached actor retains occupancy links")
			}
		}
	}
	return nil
}

func validateLiveActorRegistry(w *World) error {
	check := func(ref ActorRef, active bool, x, y int, exact bool) error {
		node := w.Actors.node(ref)
		if node == nil || !node.Linked {
			return nil
		}
		if !active {
			return fmt.Errorf("snapshot mixed registry links an inactive actor kind%d index%d", ref.Kind, ref.Index)
		}
		if exact && (node.X != x || node.Y != y) || !exact && (node.X>>8 != x || node.Y>>8 != y) {
			return fmt.Errorf("snapshot mixed registry position differs for kind%d index%d: registry(%d,%d), actor(%d,%d), exact=%t", ref.Kind, ref.Index, node.X, node.Y, x, y, exact)
		}
		return nil
	}
	for id := 1; id < FollowerCapacity; id++ {
		f := w.Followers[id]
		x, y := f.Position()
		if err := check(ActorRef{ActorFollower, uint16(id)}, f.State != Inactive, int(x*256), int(y*256), true); err != nil {
			return err
		}
	}
	for id, a := range w.Nature.Scenery {
		if err := check(ActorRef{ActorScenery, uint16(id)}, a.Kind != SceneryNone, int(a.X), int(a.Y), false); err != nil {
			return err
		}
	}
	for id, a := range w.Earth.Walls {
		if err := check(ActorRef{ActorWall, uint16(id)}, a.Active, a.X, a.Y, false); err != nil {
			return err
		}
	}
	for id, a := range w.Magnets {
		if err := check(ActorRef{ActorMagnet, uint16(id)}, a.Owner < 2, a.X, a.Y, true); err != nil {
			return err
		}
	}
	for id, reservation := range w.effects.Slots {
		mapped, active, x, y := false, false, 0, 0
		switch reservation.Kind {
		case EffectFireColumn:
			a := w.Fire.Columns[id]
			mapped, active, x, y = true, a.Active, a.X, a.Y
		case EffectFireRain:
			a := w.Fire.Rain[id]
			mapped, active, x, y = true, a.Active && a.Phase != MeteorWaiting, a.X, a.Y
		case EffectLava:
			a := w.Fire.Lava[id]
			mapped, active, x, y = true, a.Active, a.X, a.Y
		case EffectWhirlwind:
			a := w.Air.Whirlwinds[id]
			mapped, active, x, y = true, a.Active, a.X, a.Y
		case EffectLightning:
			if a := w.Air.Markers[id]; a.Active {
				mapped, active, x, y = true, true, a.X, a.Y
			} else {
				a := w.Air.Bolts[id]
				mapped, active, x, y = true, a.Active, a.X, a.Y
			}
		case EffectStorm:
			a := w.Air.Storms[id]
			mapped, active, x, y = true, a.Active, a.X, a.Y
		case EffectBasalt:
			a := w.Water.Basalt[id]
			mapped, active, x, y = true, a.Active, a.X*256+128, a.Y*256+128
		case EffectTidalWave:
			a := w.Water.Waves[id]
			mapped, active, x, y = true, a.Active, a.X, a.Y
		}
		ref := ActorRef{ActorEffect, uint16(id)}
		if !mapped && w.Actors.Effects[id].Linked {
			return fmt.Errorf("snapshot mixed registry links an unmapped effect controller")
		}
		if err := check(ref, mapped && active, x, y, true); err != nil {
			return err
		}
	}
	return nil
}
