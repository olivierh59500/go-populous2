package populous2

import "fmt"

// NativeSelectedRenderContinuation retains the normal $1e18 parent while
// its actual actor call is waiting. The saved command word remains hidden
// until the actor returns; timer and fallback selection work runs only once.
type NativeSelectedRenderContinuation struct {
	Started, Complete bool
	At                int
	SavedD2           uint32
	Actor             NativeActorRenderContinuation
	Registers         [8]uint32
	Failed            error
}

type NativeSelectedRenderCallbacks struct {
	Effects     NativeActorEffectsCallbacks
	Children    NativeActorRenderChildren
	SelectedHit func(*NativeFrameRegisterContext) error
	// Ownership supplies the real$e28 release/$e4c acquisition handoff around
	// the actor. Its source wrapper preserves all eight caller data registers.
	Ownership func(owned bool, frame *NativeFrameRegisterContext) error
	// RefreshTargets resolves current$1e after the actor's genuine modal
	// returns. It refreshes backing only, without replaying source drawing.
	RefreshTargets func(*NativeActorEffectsCallbacks) error
}

// AdvanceSelected resumes the normal selected-panel branch across the
// actor's real protection wait. The $346a editor branch is owned by AdvanceMain.
// Each call returns only the pixels emitted during that invocation.
func (r *NativeActorRenderRules) AdvanceSelected(cb NativeSelectedRenderCallbacks, shared *NativeActorRenderState, state *NativeSelectedRenderContinuation) (p NativeRenderFramePlan, complete bool, failure error) {
	p = NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if r == nil || shared == nil || state == nil || cb.Effects.Frame == nil || cb.Effects.Image == nil || !winMemoryValid(cb.Effects.Memory) || len(cb.Effects.Bitmap) != 32000 {
		return p, false, fmt.Errorf("native retained selected renderer backing missing")
	}
	if state.Failed != nil {
		return p, false, state.Failed
	}
	if state.Complete {
		return p, true, nil
	}
	c := cb.Effects.Frame
	defer func() {
		state.Registers = c.D
		if failure != nil {
			state.Failed = failure
		}
	}()
	if !state.Started {
		edit, err := cb.Effects.Memory.Read16(0xf0e)
		if err != nil {
			return p, false, err
		}
		if edit != 0 {
			return p, false, fmt.Errorf("native selected editor requires the main editor continuation")
		}
		state.Started = true
		hit := cb.SelectedHit
		if hit == nil {
			hit = func(*NativeFrameRegisterContext) error {
				return r.Frames.SelectedHit(cb.Effects.NativeRenderFrameCallbacks)
			}
		}
		at, saved, draw, err := r.Frames.selectedPrefix(cb.Effects.NativeRenderFrameCallbacks, NativeRenderFrameChildren{SelectedHit: hit}, &p)
		if err != nil {
			return p, false, err
		}
		if !draw {
			state.Complete = true
			return p, true, nil
		}
		state.At, state.SavedD2 = at, saved
		if err := selectedRenderOwnership(cb, false); err != nil {
			return p, false, err
		}
	} else {
		c.D = state.Registers
	}
	actor, done, err := r.AdvanceActor(state.At, cb.Effects, shared, &state.Actor, cb.Children)
	p.Drawn = true
	p.Pixels = append(p.Pixels, actor.Pixels...)
	p.Sprites = append(p.Sprites, actor.Sprites...)
	p.HardwarePending = actor.HardwarePending
	if err != nil {
		return p, false, err
	}
	if !done {
		return p, false, nil
	}
	if err := selectedRenderOwnership(cb, true); err != nil {
		return p, false, err
	}
	if cb.RefreshTargets != nil {
		if err := cb.RefreshTargets(&cb.Effects); err != nil {
			return p, false, err
		}
		if cb.Effects.Frame != c || cb.Effects.Image == nil || !winMemoryValid(cb.Effects.Memory) || len(cb.Effects.Bitmap) != 32000 {
			return p, false, fmt.Errorf("native selected refreshed rendering context missing")
		}
	}
	if err := r.Frames.selectedSuffix(cb.Effects.NativeRenderFrameCallbacks, state.At, state.SavedD2, &p, func(owned bool, _ *NativeFrameRegisterContext) error { return selectedRenderOwnership(cb, owned) }); err != nil {
		return p, false, err
	}
	state.Complete = true
	return p, true, nil
}

func selectedRenderOwnership(cb NativeSelectedRenderCallbacks, owned bool) error {
	if cb.Ownership == nil {
		return fmt.Errorf("native selected blitter ownership handoff missing")
	}
	saved := cb.Effects.Frame.D
	err := cb.Ownership(owned, cb.Effects.Frame)
	cb.Effects.Frame.D = saved
	return err
}
