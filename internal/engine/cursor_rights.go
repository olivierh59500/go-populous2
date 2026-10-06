package engine

import "fmt"

// Viewport describes the parcels actually presented to a human player. The
// legacy eight-by-eight form retains the original desktop rules. Mobile uses
// an exclusive rectangle and one row bitset per map row to exclude parcels
// hidden outside the projected playfield or beneath controls.
type Viewport struct {
	X, Y, Size    int
	Width, Height int      `json:",omitempty"`
	Visible       []uint64 `json:",omitempty"`
}

func (v Viewport) Valid() bool {
	if v.Size == 8 {
		return v.Width == 0 && v.Height == 0 && len(v.Visible) == 0 && v.X >= 0 && v.Y >= 0 && v.X+v.Size <= MapSize && v.Y+v.Size <= MapSize
	}
	if v.Size != 0 || v.X < 0 || v.Y < 0 || v.Width < 1 || v.Height < 1 || v.X+v.Width > MapSize || v.Y+v.Height > MapSize || len(v.Visible) != MapSize {
		return false
	}
	allowed := ^uint64(0)
	if v.Width < MapSize {
		allowed = (uint64(1)<<uint(v.Width) - 1) << uint(v.X)
	}
	any := false
	for y, row := range v.Visible {
		if y < v.Y || y >= v.Y+v.Height {
			if row != 0 {
				return false
			}
		} else if row&^allowed != 0 {
			return false
		}
		any = any || row != 0
	}
	return any
}

func (v Viewport) ContainsCell(x, y int) bool {
	if !v.Valid() || x < 0 || y < 0 || x >= MapSize || y >= MapSize {
		return false
	}
	return v.containsCell(x, y)
}

func (v Viewport) containsCell(x, y int) bool {
	if v.Size == 8 {
		return x >= v.X && y >= v.Y && x < v.X+v.Size && y < v.Y+v.Size
	}
	return x >= v.X && y >= v.Y && x < v.X+v.Width && y < v.Y+v.Height && v.Visible[y]&(uint64(1)<<uint(x)) != 0
}

func (v Viewport) ContainsCorner(x, y int) bool {
	if !v.Valid() || !insideCorner(x, y) {
		return false
	}
	if v.Size == 8 {
		return x >= v.X && y >= v.Y && x <= v.X+v.Size && y <= v.Y+v.Size
	}
	for _, cell := range [4][2]int{{x, y}, {x - 1, y}, {x, y - 1}, {x - 1, y - 1}} {
		if cell[0] >= 0 && cell[1] >= 0 && cell[0] < MapSize && cell[1] < MapSize && v.containsCell(cell[0], cell[1]) {
			return true
		}
	}
	return false
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
		if !mapped || !view.containsCell(int(f.X), int(f.Y)) {
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
