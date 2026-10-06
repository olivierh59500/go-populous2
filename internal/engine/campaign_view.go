package engine

// StepWithViewport records the terrain permissions contributed by the towns
// and followers drawn before the population pass. Result scoring uses this
// same-frame observation, rather than the campaign's static option word.
// Headless and synchronized network callers continue to use Step.
func (w *World) StepWithViewport(view Viewport) {
	if w == nil {
		return
	}
	if w.Result != 0 {
		w.Step()
		return
	}
	var rights [2]uint16
	for owner := range rights {
		// The original copies the complete saved option word, then the actor
		// renderer adds only its two low permission bits. File-defined high
		// bits remain intact even when the named options do not use them.
		rights[owner] = w.Level.Players[owner].Extra[0]&^3 | permissionWord(w.CursorTerrainRights(owner, view))&3
	}
	w.Step()
	for owner, value := range rights {
		w.Players[owner].Statistics.ScenarioOptions = value
	}
}

func permissionWord(s ScenarioOptions) uint16 {
	values := [10]bool{s.BuildAnywhere, s.SeaLevelOnly, s.ForbidEnemyTerrain, s.ForbidRaise, s.ForbidLower, s.FatalWater, s.HideEnemy, s.DisableEmigration, s.HideDisasters, s.ShallowSwamps}
	var word uint16
	for bit, value := range values {
		if value {
			word |= 1 << uint(bit)
		}
	}
	return word
}
