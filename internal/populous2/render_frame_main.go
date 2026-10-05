package populous2

import "fmt"

// NativeMainRenderState retains the shared mutable rendering state between
// frames. Its step is a synchronous source checkpoint, not a modal continuation.
type NativeMainRenderState struct {
	World     NativeWorldRenderState
	Actor     NativeActorRenderState // Shared mutable CODEE8CE across all render children.
	Alternate NativeAlternateRenderState
	Step      uint8
	View      uint16
}

// Begin resets only the frame-local program position. Shared CODE fields such
// as the town hit height remain live, as they do in the original executable.
func (s *NativeMainRenderState) Begin() error {
	if s == nil {
		return fmt.Errorf("native main render state missing")
	}
	if s.Step != 0 && s.Step != 11 {
		return fmt.Errorf("native main render frame is incomplete")
	}
	s.Step, s.View = 0, 0
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
}

// MainFrame composes the exact EA0..10B6 rendering order. World/pointer
// registers and the shared image/audio bank remain live across every child.
// Required editor/modal operations are explicit source boundaries. A callback
// must complete synchronously; after an error, discard this frame rather than
// replaying a partially executed selected actor or world traversal.
func (r *NativeActorRenderRules) MainFrame(cb NativeMainRenderCallbacks, state *NativeMainRenderState) error {
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
		if _, err := r.WorldDraw(cb.World, &state.World); err != nil {
			return err
		}
		state.Actor = state.World.Actor
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
			if err := frames.AlternateClear(render); err != nil {
				return err
			}
			view, err := m.Read16(0xf0c)
			if err != nil {
				return err
			}
			c.Word(0, view)
			state.Alternate.World.Actor = state.Actor
			if _, err := r.AlternateDraw(cb.World, &state.Alternate); err != nil {
				return err
			}
			state.Actor = state.Alternate.World.Actor
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
