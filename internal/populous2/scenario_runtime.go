package populous2

import legacy "go-populous2/internal/legacy"

func (w *World) bindScenarioRuntime() {
	w.Core.WaterFatalForPlayer = func(player int) bool { return player >= 0 && player < 2 && w.Rules[player].FatalWater }
	w.Core.TerrainCommand = w.Sculpt
}

// TerrainEditAllowed translates the separate height admissions at $19de and
// $1b38. The original checks do not depend on nearby follower population.
func (r ScenarioRules) TerrainEditAllowed(height int, raise bool) bool {
	if height < 0 || height > 8 || raise && r.ForbidRaise || !raise && r.ForbidLower {
		return false
	}
	if r.BuildAnywhere {
		return true
	}
	if !r.BuildAnywhereAtSeaLevel {
		return false
	}
	if raise {
		return height == 0
	}
	return height <= 1
}

// Sprog precedes the ordinary lower restriction, as in CODE:$12f8a.
func (w *World) Sprog(player, x, y int) bool {
	if player < 0 || player > 1 || !inside(x, y) || w.Rules[player].DisableRightClickSprog {
		return false
	}
	return w.Core.SprogAt(player, x+y*legacy.MapWidth)
}

func (w *World) FollowerVisibleOnMap(observer, owner int) bool {
	if observer < 0 || observer > 1 || owner < 0 || owner > 1 {
		return false
	}
	return owner == observer || !w.Rules[observer].HideEnemyOnMap
}

func (w *World) EffectVisibleOnMap(observer int) bool {
	return observer >= 0 && observer < 2 && !w.Rules[observer].HideDisastersOnMap
}
