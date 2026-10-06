package app

import (
	"fmt"
	"go-populous2/internal/engine"
)

func (g *Game) drawFollowerActor(id, land int) {
	w := g.World
	f := w.Followers[id]
	ax, ay := g.followerRenderAnchor(f)
	if state := f.CombatAftermath; state.Kind != engine.CombatAftermathNone {
		key := "combat/death"
		switch state.Kind {
		case engine.CombatHeroDefeated:
			key = "combat/hero-death"
		case engine.CombatVictorious:
			key = "combat/victory-blue"
			if f.Owner != 0 {
				key = "combat/victory-red"
			}
		case engine.CombatTownCollapse, engine.CombatTownRuin:
			key = fmt.Sprintf("ruin/town/%d", f.Stage)
		case engine.CombatCollateralDeath:
			key = "death/fire"
		}
		g.animation(key, int(state.Frame), ax, ay, land)
		return
	}
	if f.TerrainDeath.Active || f.State == engine.Drowning {
		key := "swimming/follower"
		frame := int(f.Frame)
		if f.TerrainDeath.Active {
			key, frame = "death/water", int(f.TerrainDeath.Frame)
		}
		if f.IsHero() {
			key += "/" + heroNames[f.Hero.Kind]
		}
		g.animation(key, frame, ax, ay, land)
		return
	}
	if f.Neutral.Kind != engine.NeutralNone || f.Neutral.VictimTime > 0 {
		name := "neutral/monster-victim"
		if f.Neutral.Kind != engine.NeutralNone {
			name = "neutral/" + neutralNames[f.Neutral.Kind]
			if f.Neutral.Paired && f.Neutral.Kind == engine.NeutralLandLowerer {
				name = "neutral/land-lowerer-paired"
			}
		}
		g.animation(name, int(f.Frame), ax, ay, land)
		return
	}
	if f.Conversion.Active {
		key := "conversion/blue"
		if f.Conversion.SourceOwner != 0 {
			key = "conversion/red"
		}
		if f.Conversion.Hero {
			key = "conversion/hero"
		}
		g.animation(key, int(f.Conversion.Frame), ax, ay, land)
		return
	}
	if g.drawAirborne(id, land, ax, ay) || g.lightningVictim(id, land, ax, ay) {
		return
	}
	if g.drawActiveBattle(id, ax, ay, land) {
		return
	}
	if f.ContactWaiting {
		key := "contact/waiting"
		if f.IsHero() {
			key = "contact/" + heroNames[f.Hero.Kind]
		}
		g.animation(key, int(f.Frame), ax, ay, land)
		return
	}
	if victim := w.FireDamage.Deaths[id]; victim.Mode != engine.FireVictimAlive {
		name := "death/fire"
		if victim.Mode == engine.FireVictimBurning {
			name = "death/burning"
		}
		if victim.Mode == engine.FireVictimTownRuin {
			name = fmt.Sprintf("ruin/town/%d", victim.TownStage)
		} else if f.IsHero() {
			name += "/" + heroNames[f.Hero.Kind]
		}
		g.animation(name, victim.Frame, ax, ay, land)
	} else if f.State == engine.Town {
		g.drawTownCenter(f, ax, ay, land)
	} else if death := w.Nature.Deaths[id]; death != engine.NatureAlive {
		name := "swamp"
		if death == engine.NatureFungusDeath {
			name = "fungus"
		}
		key := "death/" + name
		if f.IsHero() {
			key += "/" + heroNames[f.Hero.Kind]
		}
		g.animation(key, int(w.Nature.DeathFrames[id]), ax, ay, land)
	} else {
		ax, ay = g.followerRenderAnchor(f)
		name := fmt.Sprintf("follower/%d/%d/%s", f.Owner, f.AppearanceVariant, compassNames[f.Direction&7])
		if f.IsHero() {
			name = fmt.Sprintf("hero/%s/%s", heroNames[f.Hero.Kind], compassNames[f.Direction&7])
		}
		g.animation(name, int(f.Frame), ax, ay, land)
		g.drawLeaderMarker(id, ax, ay, land, name, int(f.Frame))
	}
	if f.Disease.Infected {
		g.animation("plague", int(f.Disease.Frame), ax, ay, land)
	}
}
