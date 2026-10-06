package gamcodec

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/engine"
)

var heroNames = [7]string{"", "perseus", "adonis", "heracles", "odysseus", "achilles", "helen"}

func roleToken(catalog Catalog, name string, frame int) (uint16, error) {
	found := false
	value := uint16(0)
	for token, roles := range catalog.AnimationRoles {
		for _, role := range roles {
			if role.Name == name && role.Frame == frame && (!found || token < value) {
				value, found = token, true
			}
		}
	}
	if !found {
		return 0, fmt.Errorf("GAM artwork role%s frame%d is unavailable", name, frame)
	}
	return value, nil
}

func encodeFollowerLifecycle(record []byte, w *engine.World, id int, catalog Catalog) error {
	f := w.Followers[id]
	word := func(at int, value int) { binary.BigEndian.PutUint16(record[at:], uint16(value)) }
	name := ""
	frame := int(f.Frame)
	if f.Neutral.Kind != engine.NeutralNone {
		record[0], record[12], record[22] = 0x3c, 3, 0x44
		word(40, int(f.Neutral.Kind)*2)
		name = "neutral/" + [7]string{"", "road-maker", "land-lowerer", "whirlwind-maker", "tree-planter", "fire-maker", "monster"}[f.Neutral.Kind]
	} else if f.Neutral.VictimTime > 0 {
		record[0], record[22] = 0x3e, 0x46
		word(20, f.Neutral.VictimTime)
		return nil
	} else if carry := w.Air.Carry[id]; carry.Phase != engine.AirCarryNone {
		record[0], record[22] = 8, 0x14
		reference, err := effectReference(carry.Effect)
		if err != nil {
			return err
		}
		word(32, reference)
		name = "airborne/follower"
		frame = carry.Frame
		if f.IsHero() {
			name = "airborne/" + heroNames[f.Hero.Kind]
		}
		if carry.Phase == engine.AirCarryLanding {
			record[22] = 0x1a
			name = "airborne/landing"
		}
	} else if victim := w.AirVictims[id]; victim.Phase != engine.LightningVictimNone {
		record[0], record[22] = 2, 0x1c
		name = "lightning/hit"
		frame = victim.Frame
		if victim.Phase == engine.LightningVictimTownHit {
			record[0], record[22] = 4, 0x1e
			name = "lightning/town-hit"
		}
		if victim.Phase == engine.LightningVictimDeath {
			record[22] = 0x20
			name = "lightning/hit"
			if f.IsHero() {
				name = "death/fire"
			}
		}
		if victim.Phase == engine.LightningVictimRecovery {
			record[22] = 0x22
			name = "lightning/recovery"
		}
		if f.IsHero() {
			name += "/" + heroNames[f.Hero.Kind]
		}
		reference, err := effectReference(victim.Bolt)
		if err != nil {
			return err
		}
		word(32, reference)
	} else if f.Conversion.Active {
		record[0], record[22] = 2, 0x36
		name = "conversion/blue"
		if f.Conversion.SourceOwner == 1 {
			name = "conversion/red"
		}
		if f.Conversion.Hero {
			name = "conversion/hero"
		}
		frame = int(f.Conversion.Frame)
	} else if death := w.FireDamage.Deaths[id]; death.Mode != engine.FireVictimAlive {
		record[0], record[22] = 6, 8
		name = "death/fire"
		frame = death.Frame
		if death.Mode == engine.FireVictimBurning {
			record[0], record[22] = 2, 0x3c
			name = "death/burning"
		}
		if death.Mode == engine.FireVictimTownRuin {
			record[0], record[22] = 4, 0x28
			name = fmt.Sprintf("ruin/town/%d", death.TownStage)
		} else if f.IsHero() {
			name += "/" + heroNames[f.Hero.Kind]
		}
	} else if f.CombatAftermath.Kind == engine.CombatTownRuin {
		record[0], record[22] = 4, 0x30
		word(20, f.CombatAftermath.RuinTime)
		return nil
	} else if f.CombatAftermath.Kind == engine.CombatTownCollapse {
		record[0], record[22] = 4, 0x28
		name = fmt.Sprintf("ruin/town/%d", f.Stage)
		frame = int(f.CombatAftermath.Frame)
	} else if f.CombatAftermath.Kind != engine.CombatAftermathNone {
		record[0], record[22] = 2, 0x18
		name = "combat/death"
		frame = int(f.CombatAftermath.Frame)
		if f.CombatAftermath.Kind == engine.CombatHeroDefeated {
			record[22] = 0x40
			name = "combat/hero-death"
		}
		if f.CombatAftermath.Kind == engine.CombatVictorious {
			record[22] = 0x42
			name = "combat/victory-blue"
			if f.Owner == 1 {
				name = "combat/victory-red"
			}
		}
		if f.CombatAftermath.Kind == engine.CombatCollateralDeath {
			record[0], record[22] = 6, 8
			name = "death/fire"
		}
	} else if f.TerrainDeath.Active {
		record[0], record[22] = 2, 0x32
		name = "death/water"
		frame = int(f.TerrainDeath.Frame)
		if f.IsHero() {
			name += "/" + heroNames[f.Hero.Kind]
		}
	} else if f.State == engine.Drowning {
		record[0], record[22] = 2, 0x16
		name = "swimming/follower"
		if f.IsHero() {
			name = "swimming/" + heroNames[f.Hero.Kind]
		}
	} else if f.State == engine.Fighting {
		record[0], record[22] = 2, 16
		if f.BattleWasTown {
			record[0] = 4
		}
		if f.BattleAggressor {
			record[22] = 14
		}
		name = "combat/attack"
		ref, err := fileReference(engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(f.BattleWith)})
		if err != nil {
			return err
		}
		word(30, int(ref))
	} else if f.State != engine.Walking && f.State != engine.Town {
		return fmt.Errorf("GAM follower%d lifecycle is not mapped yet", id)
	}
	if name != "" {
		token, err := roleToken(catalog, name, frame)
		if err != nil {
			return err
		}
		word(10, int(token))
	}
	if f.IsHero() {
		for offset, index := range map[int]int{30: f.Hero.Target, 42: f.Hero.CaptiveOf, 44: f.Hero.ClaimedBy} {
			ref := engine.ActorRef{}
			if index > 0 {
				ref = engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(index)}
			}
			reference, err := fileReference(ref)
			if err != nil {
				return err
			}
			word(offset, int(reference))
		}
	}
	return nil
}
