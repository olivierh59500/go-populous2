package engine

import "fmt"

// Viewport describes the original eight-by-eight parcel view. Membership is
// geometric, not a pixel-distance test around the pointer or a follower.
type Viewport struct{ X, Y, Size int }

func (v Viewport) Valid() bool {
	return v.Size == 8 && v.X >= 0 && v.Y >= 0 && v.X+v.Size <= MapSize && v.Y+v.Size <= MapSize
}
func (v Viewport) ContainsCorner(x, y int) bool {
	return v.Valid() && x >= v.X && y >= v.Y && x <= v.X+v.Size && y <= v.Y+v.Size
}

// CursorTerrainRights combines campaign permissions with the owned actors
// rendered in the current view. The original refreshes these rights each main
// frame, then adds sea-level rights for ordinary walkers and full rights for
// towns. This query stays read-only and never changes global game rules.
func (w *World) CursorTerrainRights(owner int, view Viewport) ScenarioOptions {
	if owner < 0 || owner > 1 {
		return ScenarioOptions{ForbidRaise: true, ForbidLower: true}
	}
	rights := w.Level.Players[owner].Scenario
	if !view.Valid() {
		return rights
	}
	for id := 1; id < FollowerCapacity; id++ {
		f := w.Followers[id]
		if f.State == Inactive || int(f.Owner) != owner || f.Neutral.Kind != NeutralNone {
			continue
		}
		_, _, mapped := w.Actors.Position(ActorRef{Kind: ActorFollower, Index: uint16(id)})
		if !mapped || int(f.X) < view.X || int(f.Y) < view.Y || int(f.X) >= view.X+view.Size || int(f.Y) >= view.Y+view.Size {
			continue
		}
		if w.Air.Carry[id].Phase != AirCarryNone || w.AirVictims[id].Phase != LightningVictimNone || w.Nature.Deaths[id] != NatureAlive || w.FireDamage.Deaths[id].Mode != FireVictimAlive {
			continue
		}
		full, sea := f.cursorTerrainEvidence()
		rights.BuildAnywhere = rights.BuildAnywhere || full
		rights.SeaLevelOnly = rights.SeaLevelOnly || sea
	}
	return rights
}

func (f Follower) cursorTerrainEvidence() (full, sea bool) {
	// Retained death, recovery, lifted and conversion sequences use separate
	// draw handlers, so their owner does not gain ordinary walking rights.
	if f.CleanupPrepared || f.CombatAftermath.Kind != CombatAftermathNone || f.TerrainDeath.Active || f.Disease.Dying || f.Conversion.Active || f.State == Ruin || f.State == Airborne || f.State == Converting {
		return false, false
	}
	if f.State == Town || f.State == Fighting && f.BattleWasTown {
		return true, false
	}
	if f.State == Drowning {
		return false, true
	}
	if f.State == Walking && !f.ContactWaiting {
		return false, true
	}
	return false, false
}

func (w *World) CursorTerrainAllowed(owner int, view Viewport, x, y int, raise bool) bool {
	if owner < 0 || owner > 1 || !view.ContainsCorner(x, y) || !insideCorner(x, y) {
		return false
	}
	rights := w.CursorTerrainRights(owner, view)
	return w.terrainPlanAllowedWithOptions(owner, x, y, raise, true, rights)
}

// CastFromViewport adds the source human cursor gate without changing global
// scenario rules or affecting AI/direct effect admission.
func (w *World) CastFromViewport(owner int, power PowerID, target PowerTarget, view Viewport) error {
	if power != RaiseLower {
		return w.Cast(owner, power, target)
	}
	return w.CastWithCursorTerrainRights(owner, target, view, w.CursorTerrainRights(owner, view))
}

// CastWithCursorTerrainRights applies the rights captured while drawing the
// displayed view. The live world still validates coordinates, power admission,
// terrain rules and mana before changing a height. Network callers derive
// rights from their synchronized live world through CastFromViewport.
func (w *World) CastWithCursorTerrainRights(owner int, target PowerTarget, view Viewport, rights ScenarioOptions) error {
	if owner < 0 || owner > 1 || !view.ContainsCorner(target.X, target.Y) {
		return fmt.Errorf("terrain target lies outside the visible view")
	}
	if !w.Level.Players[owner].Powers[RaiseLower] {
		return fmt.Errorf("terrain editing is disabled in this world")
	}
	if !w.changeHeightWithOptions(owner, target.X, target.Y, !target.Lower, true, rights) {
		return fmt.Errorf("terrain cannot be changed with the current visible followers")
	}
	return nil
}
