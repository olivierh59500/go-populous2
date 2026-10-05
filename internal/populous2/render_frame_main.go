package populous2

import (
	"errors"
	"fmt"
)

var errNativeMainChildWait = errors.New("native main renderer is waiting in a child")

// NativeMainRenderState retains the shared mutable rendering state between
// frames. An editor child can retain the exact selected-rendering continuation.
type NativeMainRenderState struct {
	World            NativeWorldRenderState
	Actor            NativeActorRenderState // Shared mutable CODEE8CE across all render children.
	Alternate        NativeAlternateRenderState
	Selection        NativeSelectedRenderContinuation
	Traversal        NativeWorldRenderContinuation
	Step             uint8
	View             uint16
	painting         bool
	selecting        bool
	alternateCleared bool
	failed           error
}

// Begin resets only the frame-local program position. Shared CODE fields such
// as the town hit height remain live, as they do in the original executable.
func (s *NativeMainRenderState) Begin() error {
	if s == nil {
		return fmt.Errorf("native main render state missing")
	}
	if s.failed != nil {
		return s.failed
	}
	if s.Step != 0 && s.Step != 11 {
		return fmt.Errorf("native main render frame is incomplete")
	}
	s.Step, s.View = 0, 0
	s.painting = false
	s.selecting = false
	s.Selection = NativeSelectedRenderContinuation{}
	s.Traversal = NativeWorldRenderContinuation{}
	s.alternateCleared = false
	return nil
}

type NativeMainRenderCallbacks struct {
	World    NativeWorldRenderCallbacks
	Selected NativeRenderFrameChildren
	// CODE is distinct from BSS: $2e3a is the debug formatter's mutable target.
	Code FollowerCleanupMemory
	// Alternate optionally replaces the concrete C826/C204 composition.
	Alternate    func(uint16, *NativeFrameRegisterContext) error
	DebugOverlay func(*NativeFrameRegisterContext) error // Actual2AE2 editor overlay.
	// PaintingAdvance owns the actual346A editor/modal continuation. False
	// retains the source call, including when F0E changes during the wait.
	// The host starts a fresh editor state for each new invocation/frame.
	PaintingAdvance func(*NativeFrameRegisterContext) (bool, error)
	// RefreshTargets rebinds the actual$1e/$22 buffers after a selected/editor
	// child returns; real modal swaps can change them within one Advance call.
	RefreshTargets    func(*NativeWorldRenderCallbacks) error
	SelectedOwnership func(bool, *NativeFrameRegisterContext) error
}

// MainFrame composes the exact EA0..10B6 rendering order. World/pointer
// registers and the shared image/audio bank remain live across every child.
// Required editor/modal operations are explicit source boundaries. A callback
// must complete synchronously; after an error, discard this frame rather than
// replaying a partially executed selected actor or world traversal.
func (r *NativeActorRenderRules) MainFrame(cb NativeMainRenderCallbacks, state *NativeMainRenderState) error {
	done, err := r.AdvanceMain(cb, state)
	if err == nil && !done {
		return fmt.Errorf("native main renderer requires editor continuation")
	}
	return err
}

// AdvanceMain resumes editor, selected-actor and world-traversal calls without
// repeating their completed work. A source error keeps its mutated prefix.
func (r *NativeActorRenderRules) AdvanceMain(cb NativeMainRenderCallbacks, state *NativeMainRenderState) (bool, error) {
	if state == nil {
		return false, fmt.Errorf("native main render state missing")
	}
	if state.failed != nil {
		return false, state.failed
	}
	err := r.mainFrame(cb, state)
	if errors.Is(err, errNativeMainChildWait) {
		return false, nil
	}
	if err != nil {
		state.failed = err
	}
	return err == nil, err
}

func (r *NativeActorRenderRules) mainFrame(cb NativeMainRenderCallbacks, state *NativeMainRenderState) error {
	if r == nil || state == nil || cb.World.Effects.Frame == nil || !winMemoryValid(cb.World.Effects.Memory) {
		return fmt.Errorf("native main render frame backing missing")
	}
	frames, render := &r.Frames, cb.World.Effects.NativeRenderFrameCallbacks
	m, c := render.Memory, render.Frame
	var err error
	if state.Step == 0 {
		state.View, err = m.Read16(0xf0c)
		if err != nil {
			return err
		}
		if state.View == 8 {
			state.Step = 1
		} else {
			state.Step = 7
		}
	}
	if state.Step == 1 {
		if _, err := frames.BackgroundCopy(m, c); err != nil {
			return err
		}
		if len(cb.World.Background) != 32000 || len(render.Bitmap) != 32000 {
			return fmt.Errorf("native main background buffers missing")
		}
		copy(render.Bitmap, cb.World.Background)
		state.Step = 2
	}
	if state.Step == 2 {
		if err := frames.Highlights(render); err != nil {
			return err
		}
		state.Step = 3
	}
	if state.Step == 3 {
		if _, err := frames.HUD(render, true); err != nil {
			return err
		}
		state.Step = 4
	}
	if state.Step == 4 {
		retainedSelected := cb.Selected.DrawActor == nil && cb.SelectedOwnership != nil
		if retainedSelected && !state.selecting {
			edit, err := m.Read16(0xf0e)
			if err != nil {
				return err
			}
			retainedSelected = edit == 0
		}
		if !state.painting && !state.selecting && cb.PaintingAdvance != nil {
			edit, err := m.Read16(0xf0e)
			if err != nil {
				return err
			}
			state.painting = edit != 0
		}
		if state.painting {
			if cb.PaintingAdvance == nil {
				return fmt.Errorf("native retained editor child346a missing")
			}
			done, err := cb.PaintingAdvance(c)
			if err != nil {
				return err
			}
			if !done {
				return errNativeMainChildWait
			}
			state.painting = false
		} else if state.selecting || retainedSelected {
			state.selecting = true
			selected := NativeSelectedRenderCallbacks{Effects: cb.World.Effects, Children: cb.World.Children, SelectedHit: cb.Selected.SelectedHit, Ownership: cb.SelectedOwnership}
			if cb.RefreshTargets != nil {
				selected.RefreshTargets = func(effects *NativeActorEffectsCallbacks) error {
					world := cb.World
					world.Effects = *effects
					if err := cb.RefreshTargets(&world); err != nil {
						return err
					}
					*effects = world.Effects
					return nil
				}
			}
			_, done, err := r.AdvanceSelected(selected, &state.Actor, &state.Selection)
			if err != nil {
				return err
			}
			if !done {
				return errNativeMainChildWait
			}
			state.selecting = false
		} else {
			children := cb.Selected
			if children.SelectedHit == nil {
				children.SelectedHit = func(*NativeFrameRegisterContext) error { return frames.SelectedHit(render) }
			}
			if children.DrawActor == nil {
				children.DrawActor = func(at int, _ *NativeFrameRegisterContext) error {
					_, err := r.Actor(at, cb.World.Effects, &state.Actor, cb.World.Children)
					return err
				}
			}
			if _, err := frames.Selected(render, children); err != nil {
				return err
			}
		}
		if cb.RefreshTargets != nil {
			if err := cb.RefreshTargets(&cb.World); err != nil {
				return err
			}
			render = cb.World.Effects.NativeRenderFrameCallbacks
			if render.Frame != c || !winMemoryValid(render.Memory) || len(render.Bitmap) != 32000 {
				return fmt.Errorf("native main refreshed rendering context missing")
			}
			m = render.Memory
		}
		state.Step = 5
	}
	if state.Step == 5 {
		edit, err := m.Read16(0xf0e)
		if err != nil {
			return err
		}
		if edit == 0 {
			if err := frames.Countdown(render); err != nil {
				return err
			}
		}
		state.Step = 6
	}
	if state.Step == 6 {
		state.World.Actor = state.Actor
		if !state.Traversal.Started && cb.Code.Write32 != nil {
			if err := cb.Code.Write32(0xe458, 0x00c00048); err != nil {
				return err
			}
		}
		_, done, err := r.AdvanceWorldDraw(cb.World, &state.World, &state.Traversal)
		state.Actor = state.World.Actor
		if err != nil {
			return err
		}
		if !done {
			return errNativeMainChildWait
		}
		if cb.RefreshTargets != nil {
			if err := cb.RefreshTargets(&cb.World); err != nil {
				return err
			}
			render = cb.World.Effects.NativeRenderFrameCallbacks
		}
		if cb.Code.Write32 != nil {
			if err := cb.Code.Write32(0xe458, uint32(state.World.ProjectionX)<<16|uint32(state.World.ProjectionY)); err != nil {
				return err
			}
		}
		mode, err := m.Read16(0xeb44)
		if err != nil {
			return err
		}
		if mode == 8 {
			pointer, err := m.Read32(0x1e)
			if err != nil {
				return err
			}
			if cb.Code.Write32 == nil {
				return fmt.Errorf("native main debug CODE backing missing")
			}
			if err := cb.Code.Write32(0x2e3a, pointer); err != nil {
				return err
			}
			clock, err := m.Read32(0xf40)
			if err != nil {
				return err
			}
			c.D[0] = clock
			if cb.DebugOverlay == nil {
				return fmt.Errorf("native main editor overlay2AE2 missing")
			}
			if err := cb.DebugOverlay(c); err != nil {
				return err
			}
		}
		state.Step = 8
	}
	if state.Step == 7 {
		if cb.Alternate != nil {
			if err := cb.Alternate(state.View, c); err != nil {
				return err
			}
		} else {
			if !state.alternateCleared {
				if err := frames.AlternateClear(render); err != nil {
					return err
				}
				view, err := m.Read16(0xf0c)
				if err != nil {
					return err
				}
				c.Word(0, view)
				state.alternateCleared = true
			}
			state.Alternate.World.Actor = state.Actor
			_, done, err := r.AdvanceAlternateDraw(cb.World, &state.Alternate, &state.Traversal)
			state.Actor = state.Alternate.World.Actor
			if err != nil {
				return err
			}
			if cb.Code.Write32 != nil {
				if err := cb.Code.Write32(0xe458, uint32(state.Alternate.World.ProjectionX)<<16|uint32(state.Alternate.World.ProjectionY)); err != nil {
					return err
				}
			}
			if cb.Code.Write16 != nil {
				for i, value := range state.Alternate.Scratch {
					if err := cb.Code.Write16(0xc132+i*2, value); err != nil {
						return err
					}
				}
			}
			if !done {
				return errNativeMainChildWait
			}
		}
		if cb.RefreshTargets != nil {
			if err := cb.RefreshTargets(&cb.World); err != nil {
				return err
			}
			render = cb.World.Effects.NativeRenderFrameCallbacks
		}
		state.Step = 8
	}
	if state.Step == 8 {
		if _, err := frames.MapCursor(render); err != nil {
			return err
		}
		state.Step = 9
	}
	if state.Step == 9 {
		if _, err := frames.CameraMarker(render); err != nil {
			return err
		}
		state.Step = 10
	}
	if state.Step == 10 {
		if _, err := frames.Cursor(render); err != nil {
			return err
		}
		state.Step = 11
	}
	return nil
}
