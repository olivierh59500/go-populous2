package populous2

// Fungus controllers mutate the original tile codes. They have no independent
// image, random-area damage or map-occupancy link.
func (w *World) castFungus(player, x, y int) FungusPlacement {
	return w.FungusRules.Create(&w.NativeEffects, &w.FungusState, player, x, y, w.Experience[player][Plants], w.fungusTile, w.setFungusTile)
}

func (w *World) fungusTile(pos int) uint8 {
	if pos < 0 || pos >= len(w.Marks) {
		return 0
	}
	return w.nativeTileAt(pos%64, pos/64)
}

func (w *World) setFungusTile(pos int, tile uint8) {
	if pos < 0 || pos >= len(w.Marks) {
		return
	}
	w.Marks[pos] = Mark{Spell: Fungus, Life: 1, Persistent: true, NativeTile: tile}
}

func (w *World) tickFungus(index int) FungusStep {
	return w.FungusRules.Tick(&w.NativeEffects[index], &w.FungusState, index, w.fungusTile, w.setFungusTile)
}

// validSavedFungus distinguishes collecting references and byte rectangles
// from the moving effects' fixed-point positions. Native edge rectangles can
// contain wrapped coordinates; their accesses are bounded by FungusRules.
func validSavedFungus(pool *[NativeEffectCapacity]NativeEffectActor, state *FungusState) bool {
	for player, ref := range state.Pending {
		if ref == 0 {
			continue
		}
		index := int(ref) - 1
		if index < 0 || index >= len(pool) {
			return false
		}
		actor := pool[index]
		if !actor.Active || actor.Kind != FungusActorKind || actor.Player != uint8(player) || actor.State != FungusCollecting {
			return false
		}
	}
	for index, actor := range pool {
		if !actor.Active || actor.Kind != FungusActorKind {
			continue
		}
		if actor.Player > 1 || actor.Speed < 3 || actor.Speed > 10 {
			return false
		}
		switch actor.State {
		case FungusCollecting:
			if actor.Timer <= 0 || actor.Timer > 100 || state.Pending[actor.Player] != uint16(index+1) {
				return false
			}
		case FungusEvolving:
			if actor.Timer < 0 || actor.Timer > int16(actor.Speed) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
