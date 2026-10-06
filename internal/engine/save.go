package engine

import (
	"encoding/json"
	"fmt"
	"io"
)

const SnapshotVersion = 1
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
	Construction ConstructionSnapshot                     `json:"construction"`
}

func (w *World) Snapshot() Snapshot {
	if w == nil {
		return Snapshot{Version: SnapshotVersion}
	}
	s := Snapshot{Version: SnapshotVersion, World: SavedWorld(*w), Random: uint32(w.random), Reservations: w.effects.Slots, Construction: ConstructionSnapshot{w.terrainTargets, w.terrainTargetSet, w.developmentTarget, w.developmentTargetSet}}
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
	if s.Version != SnapshotVersion {
		return nil, fmt.Errorf("unsupported snapshot version %d", s.Version)
	}
	w := World(s.World)
	w.random = randomState(s.Random)
	w.effects.Slots = s.Reservations
	w.terrainTargets, w.terrainTargetSet = s.Construction.TerrainTargets, s.Construction.TerrainTargetSet
	w.developmentTarget, w.developmentTargetSet = s.Construction.DevelopmentTargets, s.Construction.DevelopmentTargetSet
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
		if w.developmentTargetSet[vertex] && w.developmentTarget[vertex] > 8 {
			return fmt.Errorf("snapshot development altitude is invalid")
		}
	}
	for at, cell := range w.Tiles {
		if cell.BaseAltitude > 8 || cell.Shape > 15 || w.Farms[at] > 2 || w.terrainTargetSet[at] && w.terrainTargets[at] > 8 {
			return fmt.Errorf("snapshot parcel %d is invalid", at)
		}
		for _, height := range cell.Corners {
			if height > 8 {
				return fmt.Errorf("snapshot parcel %d has an invalid corner", at)
			}
		}
	}
	for owner, player := range w.Players {
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
		if id == 0 || f.Owner > 1 || f.State > Converting || !inside(int(f.X), int(f.Y)) || f.Stage >= TownStages || f.Hero.Kind > HeroHelen || f.Hero.Phase > HeroDying || f.NextFollower < 0 || f.NextFollower >= FollowerCapacity || f.PreviousFollower < 0 || f.PreviousFollower >= FollowerCapacity || f.BattleWith < 0 || f.BattleWith >= FollowerCapacity || f.ContactWith < 0 || f.ContactWith >= FollowerCapacity || f.Hero.Target < 0 || f.Hero.Target >= FollowerCapacity || f.Hero.ClaimedBy < 0 || f.Hero.ClaimedBy >= FollowerCapacity || f.Hero.CaptiveOf < 0 || f.Hero.CaptiveOf >= FollowerCapacity {
			return fmt.Errorf("snapshot follower %d contains an invalid state or relation", id)
		}
		if f.Conversion.Active && (f.Conversion.SourceOwner > 1 || f.Conversion.Frame >= 12) {
			return fmt.Errorf("snapshot follower %d conversion is invalid", id)
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
			if w.Fire.Columns[id].Active || w.Fire.Rain[id].Active || w.Fire.Volcano[id].Active || w.Fire.Lava[id].Active || w.Nature.Fungi[id].Active || w.Air.Markers[id].Active || w.Air.Bolts[id].Active || w.Air.Whirlwinds[id].Active || w.Air.Storms[id].Active || w.Water.Basalt[id].Active || w.Water.Whirlpools[id].Active || w.Water.Waves[id].Active {
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
