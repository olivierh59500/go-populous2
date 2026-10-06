package populous2

import "fmt"

// The live game owns E8CE in canonical CODE. The caller state remains the
// legacy standalone backing when no CODE view is bound.
func (r *NativeActorRenderRules) setTownHitHeight(state *NativeActorRenderState, value uint16) error {
	state.TownHitHeight = value
	if r.Frames.view.data.Read8 == nil {
		return nil
	}
	if r.Frames.view.data.Write16 == nil {
		return fmt.Errorf("native town hit height live CODE writer missing")
	}
	return r.Frames.view.data.Write16(0xe8ce, value)
}

func (r *NativeActorRenderRules) townHitHeight(state *NativeActorRenderState) (uint16, error) {
	if r.Frames.view.data.Read8 == nil {
		return state.TownHitHeight, nil
	}
	value, err := r.Frames.word(0xe8ce)
	if err == nil {
		state.TownHitHeight = value
	}
	return value, err
}
