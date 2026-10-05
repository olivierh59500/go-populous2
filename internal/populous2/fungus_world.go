package populous2

// validSavedFungus distinguishes collecting references and byte rectangles
// from the moving effects' fixed-point positions. Native edge rectangles can
// contain wrapped coordinates; raw World access retains adjacent BSS aliases.
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
