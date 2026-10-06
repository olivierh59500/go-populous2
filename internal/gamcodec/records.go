package gamcodec

import (
	"encoding/binary"
	"fmt"
	"strings"

	"go-populous2/internal/engine"
)

func decodeFollower(record []byte, id int, snapshot *engine.Snapshot, catalog Catalog) (engine.Follower, engine.FollowerMotionSnapshot, error) {
	word := func(at int) uint16 { return binary.BigEndian.Uint16(record[at:]) }
	f := engine.Follower{Owner: record[12] - 1, X: record[6], Y: record[8], PreviousX: record[6], PreviousY: record[8], MovementSpeed: record[18], Weapons: int(record[25]), Population: int(int32(binary.BigEndian.Uint32(record[26:]))), Stage: record[1], FoundedAt: uint64(word(46))}
	motion := engine.FollowerMotionSnapshot{PositionX: int(word(6)), PositionY: int(word(8)), VelocityX: int(int16(word(14))), VelocityY: int(int16(word(16))), LegRemaining: int(int16(word(20))), PositionSet: true}
	f.State = engine.Walking
	if record[0] == 0x3c {
		f.Owner = 2
		selector := word(40)
		if selector < 2 || selector > 12 || selector&1 != 0 {
			return f, motion, fmt.Errorf("invalid GAM neutral selector")
		}
		f.Neutral.Kind = engine.NeutralKind(selector / 2)
	} else if record[0] == 4 {
		f.State = engine.Town
	}
	state := record[22]
	switch state {
	case 0x44:
		if f.Neutral.Kind == engine.NeutralNone {
			return f, motion, fmt.Errorf("GAM neutral phase has no invention kind")
		}
	case 0x46:
		f.State = engine.Ruin
		f.Neutral.VictimTime = int(word(20))
	case 2, 18:
	case 4:
		motion.Moving = true
	case 6:
		f.State = engine.Town
		f.Work = word(20)
	case 14, 16:
		f.State = engine.Fighting
		f.BattleWasTown = record[0] == 4
		f.BattleAggressor = state == 14
		contact, err := reference(word(30))
		if err != nil {
			return f, motion, err
		}
		if contact.Kind != engine.ActorFollower {
			return f, motion, fmt.Errorf("GAM combat partner is not a follower")
		}
		f.BattleWith = int(contact.Index)
	case 0x16:
		f.State = engine.Drowning
	case 0x18, 0x2c, 0x2e, 0x32, 0x38, 0x3e, 0x40:
		f.State = engine.Ruin
	case 0x42:
		f.State = engine.Walking
	case 0x24, 0x26:
		f.State = engine.Walking
	case 0x34:
		f.State = engine.Walking
		f.Hero.Phase = engine.HeroCaptive
	case 0x14, 0x1a:
		f.State = engine.Airborne
		effect, err := effectIndex(word(32))
		if err != nil {
			return f, motion, err
		}
		carry := engine.AirCarryState{Phase: engine.AirCarryFlying, Effect: effect, Frames: 12}
		if state == 0x1a {
			carry.Phase, carry.Frames = engine.AirCarryLanding, 7
		}
		snapshot.World.Air.Carry[id] = carry
	case 0x1c, 0x1e, 0x20, 0x22:
		phase := engine.LightningVictimWalkingHit
		if state == 0x1e {
			phase = engine.LightningVictimTownHit
		}
		if state == 0x20 {
			phase = engine.LightningVictimDeath
			f.State = engine.Ruin
		}
		if state == 0x22 {
			phase = engine.LightningVictimRecovery
		}
		bolt, err := effectIndex(word(32))
		if err != nil {
			return f, motion, err
		}
		snapshot.World.AirVictims[id] = engine.LightningVictimState{Phase: phase, Bolt: bolt, Frames: 2}
	case 0x36:
		f.State = engine.Converting
		f.Conversion = engine.ConversionState{Active: true, SourceOwner: f.Owner}
	case 0x3a:
		f.Consecrated = true
	case 0x3c:
		f.State = engine.Ruin
		snapshot.World.FireDamage.Deaths[id] = engine.FireVictimDeath{Mode: engine.FireVictimBurning, Frames: 21}
	case 0x28:
		f.State = engine.Ruin
		f.CombatAftermath = engine.CombatAftermathState{Kind: engine.CombatTownCollapse, Frames: 13, RuinTime: 400}
	case 0x30:
		f.State = engine.Ruin
		f.CombatAftermath = engine.CombatAftermathState{Kind: engine.CombatTownRuin, RuinTime: int(word(20))}
	default:
		if record[0] == 6 && state == 8 {
			f.State = engine.Ruin
			snapshot.World.FireDamage.Deaths[id] = engine.FireVictimDeath{Mode: engine.FireVictimDying, Frames: 9}
		} else {
			return f, motion, fmt.Errorf("GAM follower%d kind%d phase%d is not mapped yet", id, record[0], state)
		}
	}
	if record[13]&2 != 0 {
		hero := word(40)
		if hero > 10 || hero&1 != 0 {
			return f, motion, fmt.Errorf("GAM hero selector is invalid")
		}
		f.Hero.Kind = engine.HeroKind(hero/2 + 1)
		f.Hero.Phase = engine.HeroFindTarget
	}
	if f.Hero.Kind != engine.HeroNone {
		if target, err := reference(word(30)); err == nil && target.Kind == engine.ActorFollower {
			f.Hero.Target = int(target.Index)
			f.Hero.Phase = engine.HeroPursuing
		}
		if capture, err := reference(word(42)); err == nil && capture.Kind == engine.ActorFollower {
			f.Hero.CaptiveOf = int(capture.Index)
		}
		if claim, err := reference(word(44)); err == nil && claim.Kind == engine.ActorFollower {
			f.Hero.ClaimedBy = int(claim.Index)
		}
	}
	if f.State == engine.Ruin && f.Population <= 0 && (snapshot.World.FireDamage.Deaths[id].Mode != engine.FireVictimAlive || f.TerrainDeath.Active || f.CombatAftermath.Kind != engine.CombatAftermathNone) {
		f.CleanupPrepared = true
	}
	if record[13]&0x20 != 0 {
		f.Disease.Infected = true
	}
	f.Frame = word(10) / 4
	if f.State == engine.Walking {
		f.Frame %= 4
	}
	if state != 2 && state != 4 && state != 6 && state != 18 && state != 0x30 && state != 0x46 && state != 0x3a {
		role, ok := followerAnimationRole(catalog.AnimationRoles[word(10)], state)
		if !ok {
			return f, motion, fmt.Errorf("GAM follower%d phase%d animation%d has no semantic catalog role", id, state, word(10))
		}
		f.Frame = uint16(role.Frame)
		if strings.HasPrefix(role.Name, "combat/death") {
			f.State = engine.Ruin
			f.CombatAftermath = engine.CombatAftermathState{Kind: engine.CombatDefeated, Frame: uint16(role.Frame), Frames: uint16(catalogAnimationLength(catalog, role.Name))}
		}
		if strings.HasPrefix(role.Name, "combat/hero-death") {
			f.State = engine.Ruin
			f.CombatAftermath = engine.CombatAftermathState{Kind: engine.CombatHeroDefeated, Frame: uint16(role.Frame), Frames: uint16(catalogAnimationLength(catalog, role.Name))}
		}
		if strings.HasPrefix(role.Name, "combat/victory") {
			f.State = engine.Walking
			f.CombatAftermath = engine.CombatAftermathState{Kind: engine.CombatVictorious, Frame: uint16(role.Frame), Frames: uint16(catalogAnimationLength(catalog, role.Name))}
		}
		if strings.HasPrefix(role.Name, "death/water") || strings.HasPrefix(role.Name, "death/fatal") {
			f.State = engine.Ruin
			f.TerrainDeath = engine.TerrainDeathState{Active: true, Frame: uint16(role.Frame), Frames: uint16(catalogAnimationLength(catalog, role.Name))}
		}
		if f.State == engine.Ruin && f.Population <= 0 && (snapshot.World.FireDamage.Deaths[id].Mode != engine.FireVictimAlive || f.TerrainDeath.Active || f.CombatAftermath.Kind != engine.CombatAftermathNone) {
			f.CleanupPrepared = true
		}
		switch f.State {
		case engine.Airborne:
			carry := &snapshot.World.Air.Carry[id]
			carry.Frame = role.Frame
			if f.IsHero() && carry.Phase == engine.AirCarryFlying {
				carry.Frames = 4
			}
		case engine.Converting:
			f.Conversion.Frame = uint16(role.Frame)
			f.Conversion.Hero = f.IsHero()
		case engine.Ruin:
			if death := &snapshot.World.FireDamage.Deaths[id]; death.Mode != engine.FireVictimAlive {
				death.Frame = role.Frame
				if f.IsHero() {
					death.Frames = 9
				}
			}
			if f.CombatAftermath.Kind == engine.CombatTownCollapse {
				f.CombatAftermath.Frame = uint16(role.Frame)
				f.CombatAftermath.Frames = uint16(catalogAnimationLength(catalog, role.Name))
			}
		}
		if victim := &snapshot.World.AirVictims[id]; victim.Phase != engine.LightningVictimNone {
			victim.Hero = f.Hero.Kind
			victim.Frame = role.Frame
			victim.Frames = catalogAnimationLength(catalog, role.Name)
			if strings.HasPrefix(role.Name, "lightning/recovery") {
				victim.Sequence = engine.LightningRecoverySequence
			} else if strings.HasPrefix(role.Name, "death/") {
				victim.Sequence = engine.LightningDeathSequence
			}
		}
	}
	f.Target = int(f.X+uint8(sign(motion.VelocityX))) + int(f.Y+uint8(sign(motion.VelocityY)))*64
	return f, motion, nil
}

func followerAnimationRole(roles []AnimationRole, state uint8) (AnimationRole, bool) {
	prefix := ""
	aftermath := false
	switch state {
	case 0x14:
		prefix = "airborne/"
	case 0x1a:
		prefix = "airborne/landing"
	case 0x1c, 0x1e:
		prefix = "lightning/"
	case 0x20:
		prefix = "death/"
	case 0x22:
		prefix = "lightning/recovery"
	case 0x36:
		prefix = "conversion/"
	case 0x3c:
		prefix = "death/burning"
	case 0x28:
		prefix = "ruin/town/"
	case 8:
		prefix = "death/"
	case 0x44:
		prefix = "neutral/"
	case 14, 16:
		prefix = "combat/attack"
	case 0x16:
		prefix = "swimming/"
	case 0x18, 0x2c, 0x2e, 0x32, 0x38, 0x3e, 0x40:
		prefix = ""
		aftermath = true
	case 0x42:
		prefix = "combat/victory"
	case 0x24, 0x26, 0x34:
		prefix = "hero/"
	}
	for _, role := range roles {
		if aftermath && (strings.HasPrefix(role.Name, "combat/death") || strings.HasPrefix(role.Name, "combat/hero-death") || strings.HasPrefix(role.Name, "death/water") || strings.HasPrefix(role.Name, "death/fatal")) {
			return role, true
		}
		if (strings.HasPrefix(role.Name, prefix) || state == 0x20 && role.Name == "lightning/hit") && prefix != "" {
			return role, true
		}
	}
	return AnimationRole{}, false
}
func catalogAnimationLength(catalog Catalog, name string) int {
	length := 0
	for _, roles := range catalog.AnimationRoles {
		for _, role := range roles {
			if role.Name == name {
				length = max(length, role.Frame+1)
			}
		}
	}
	return length
}
func sign(v int) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

func decodePlayers(r fileReader, s *engine.Snapshot) error {
	for owner := 0; owner < 2; owner++ {
		god := 0xe76a + (owner+1)*314
		p := &s.World.Players[owner]
		p.Mana = int(int32(r.long(god)))
		mode := r.word(god + 12)
		switch mode {
		case 14:
			p.Mode = engine.Settle
		case 16:
			p.Mode = engine.Rally
		case 18:
			p.Mode = engine.Join
		case 20:
			p.Mode = engine.Fight
		default:
			return fmt.Errorf("GAM player mode%d is invalid", mode)
		}
		leader, err := reference(r.word(god + 8))
		if err != nil {
			return err
		}
		if leader.Kind == engine.ActorFollower {
			p.Leader = int(leader.Index)
		} else {
			p.Leader = 0
		}
		for i := range p.Experience {
			p.Experience[i] = r.byte(god + 0x52 + i)
		}
		for id := range s.World.Level.Players[owner].Powers {
			s.World.Level.Players[owner].Powers[id] = int8(r.byte(god+0x70+id)) > 0
		}
		s.World.Level.Players[owner].Attrition = int(int32(r.long(god + 20)))
		magnet := r.record(0xe74e, owner, 14)
		s.World.Magnets[owner] = engine.MagnetActor{X: int(binary.BigEndian.Uint16(magnet[6:])), Y: int(binary.BigEndian.Uint16(magnet[8:])), Owner: uint8(owner)}
		p.RallyX, p.RallyY = s.World.Magnets[owner].X/256, s.World.Magnets[owner].Y/256
		p.Computer = owner == 1
		if marker, err := effectIndex(r.word(god + 0x12)); err == nil {
			s.World.Air.MarkerSlots[owner] = marker
		} else {
			return err
		}
		pending, err := effectIndex(r.word(god + 14))
		if err != nil {
			return err
		}
		s.World.Nature.PendingFungus[owner] = uint16(pending)
		if err := decodeAIStatistics(r, s, owner); err != nil {
			return err
		}
	}
	return nil
}

func decodeChains(r fileReader, s *engine.Snapshot) error {
	seen := make(map[engine.ActorRef]bool)
	for cell := 0; cell < 64*64; cell++ {
		ref, err := reference(r.word(0xf46 + cell*4))
		if err != nil {
			return err
		}
		s.World.Actors.Heads[cell] = ref
		previous := engine.ActorRef{}
		lastFollower := 0
		for visits := 0; ref.Kind != engine.ActorNone; visits++ {
			if visits >= 1100 || seen[ref] {
				return fmt.Errorf("GAM actor chain is cyclic")
			}
			seen[ref] = true
			address, err := fileReference(ref)
			if err != nil {
				return err
			}
			at := 0x76c0 + int(int16(address))
			next, err := reference(r.word(at + 2))
			if err != nil {
				return err
			}
			back, err := reference(r.word(at + 4))
			if err != nil {
				return err
			}
			if back != previous {
				return fmt.Errorf("GAM reciprocal actor link is inconsistent")
			}
			link := engine.ActorLink{Next: next, Previous: previous, X: int(r.word(at + 6)), Y: int(r.word(at + 8)), Linked: true}
			if link.X/256+link.Y/256*64 != cell {
				return fmt.Errorf("GAM actor belongs to the wrong parcel")
			}
			switch ref.Kind {
			case engine.ActorFollower:
				id := int(ref.Index)
				s.World.Actors.Followers[id] = link
				if s.World.Followers[id].State == engine.Inactive {
					return fmt.Errorf("GAM map links an inactive follower")
				}
				if lastFollower == 0 {
					s.World.Occupants[cell] = uint16(id)
				} else {
					s.World.Followers[lastFollower].NextFollower = id
				}
				s.World.Followers[id].PreviousFollower = lastFollower
				lastFollower = id
			case engine.ActorScenery:
				s.World.Actors.Scenery[ref.Index] = link
			case engine.ActorMagnet:
				s.World.Actors.Magnets[ref.Index] = link
			case engine.ActorWall:
				s.World.Actors.Walls[ref.Index] = link
			case engine.ActorEffect:
				s.World.Actors.Effects[ref.Index] = link
			}
			previous, ref = ref, next
		}
	}
	return nil
}

func decodeWall(record []byte, catalog Catalog) (engine.WallActor, error) {
	if record[0] != 26 && record[0] != 28 || record[12] > 2 || record[1] > 8 || record[1]&1 != 0 {
		return engine.WallActor{}, fmt.Errorf("invalid wall kind/owner/variant")
	}
	w := engine.WallActor{Active: true, Broken: record[0] == 28, Owner: record[12] - 1, X: int(record[6]), Y: int(record[8]), Variant: record[1]}
	animation := binary.BigEndian.Uint16(record[10:])
	role, ok := wallAnimationRole(catalog.AnimationRoles[animation])
	if ok {
		w.Frame = uint8(role.Frame)
		w.Gate = role.Name == "wall/gate-horizontal" || role.Name == "wall/gate-vertical"
		w.GateVertical = role.Name == "wall/gate-vertical"
	} else {
		if animation >= 0xb44 && animation < 0xb50 {
			w.Gate = true
			w.Frame = uint8((animation - 0xb44) / 4)
		} else if animation >= 0xb54 && animation < 0xb60 {
			w.Gate, w.GateVertical = true, true
			w.Frame = uint8((animation - 0xb54) / 4)
		} else if !w.Broken {
			matched := false
			for _, base := range []uint16{0x5bc, 0x5cc, 0x5dc} {
				if animation >= base && animation < base+12 && (animation-base)%4 == 0 {
					w.Frame = uint8((animation - base) / 4)
					matched = true
					break
				}
			}
			if !matched {
				return w, fmt.Errorf("wall animation%d has no semantic catalog role", animation)
			}
		} else {
			return w, fmt.Errorf("broken wall animation%d has no semantic catalog role", animation)
		}
	}
	next, err := reference(binary.BigEndian.Uint16(record[14:]))
	if err != nil {
		return w, err
	}
	if next.Kind != engine.ActorNone {
		if next.Kind != engine.ActorWall {
			return w, fmt.Errorf("wall owner-list link points into another pool")
		}
		w.Next = next.Index + 1
	}
	return w, nil
}

func wallAnimationRole(roles []AnimationRole) (AnimationRole, bool) {
	for _, role := range roles {
		if strings.HasPrefix(role.Name, "wall/") {
			return role, true
		}
	}
	return AnimationRole{}, false
}
