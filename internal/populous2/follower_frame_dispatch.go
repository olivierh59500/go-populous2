package populous2

import "fmt"

type NativeFollowerFrameBindings struct {
	Aftermath *NativeFollowerAftermathFrameRules
	Motion    *FollowerMotionFrameRules
	Entry     *NativeFollowerEntryFrameRules
	Combat    *FollowerCombatFrameRules
	// Continue executes direct search/town/hero tails reached from a child,
	// without inventing another prepass or starting a new actor update.
	Continue func(NativeRecordReference, uint32, *NativeFrameRegisterContext, *NativeFollowerPassState) (NativeFollowerPassFlow, error)
	// Other owns state families whose complete register-bearing controller
	// has not yet been attached. Missing active bodies are explicit errors.
	Other    func(NativeRecordReference, uint16, *NativeFrameRegisterContext, *NativeFollowerPassState) (NativeFollowerPassFlow, error)
	MapPoint func(uint16, *NativeFrameRegisterContext) error
	Result   func(uint16, *NativeFrameRegisterContext) error
}

func (w *World) nativeFollowerFrameCallbacks(frame *NativeFrameRegisterContext, state *NativeFollowerPassState, bindings NativeFollowerFrameBindings) NativeFollowerPassCallbacks {
	return NativeFollowerPassCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame, State: state,
		Prepass: func(ref NativeRecordReference, frame *NativeFrameRegisterContext, _ *NativeFollowerPassState) error {
			_, err := w.CommonPrepass.Tick(ref, w.nativeCommonPrepassFrameCallbacks(frame))
			return err
		},
		Body: func(ref NativeRecordReference, target uint16, frame *NativeFrameRegisterContext, state *NativeFollowerPassState) (NativeFollowerPassFlow, error) {
			actorState, err := w.nativeCleanupMemory().Read8(cleanupRecordAddress(ref) + 22)
			if err != nil {
				return NativeFollowerNext, err
			}
			if aftermathHandler(actorState) != 0 {
				if bindings.Aftermath == nil {
					return NativeFollowerNext, fmt.Errorf("native follower aftermath frame rules missing")
				}
				return bindings.Aftermath.Tick(ref, w.nativeAftermathFrameCallbacks(frame))
			}
			if actorState == 0x14 {
				cb := w.nativeWhirlwindCallbacks()
				cb.Frame = frame
				cb.SourceD2 = uint16(frame.D[2])
				_, err := w.NativeWhirlwind.TickFollower(ref, cb)
				return NativeFollowerCount, err
			}
			if actorState == 4 {
				return w.nativeMotionFrameBody(ref, frame, state, bindings)
			}
			if actorState == 0x0e || actorState == 0x10 {
				return w.nativeCombatFrameBody(ref, actorState, frame, state, bindings)
			}
			if bindings.Other != nil {
				return bindings.Other(ref, target, frame, state)
			}
			return NativeFollowerNext, fmt.Errorf("native follower state%02x full frame controller missing", actorState)
		},
		MapPoint: bindings.MapPoint, Result: bindings.Result,
	}
}

func (w *World) nativeMotionFrameBody(ref NativeRecordReference, frame *NativeFrameRegisterContext, state *NativeFollowerPassState, bindings NativeFollowerFrameBindings) (NativeFollowerPassFlow, error) {
	if bindings.Motion == nil {
		return NativeFollowerNext, fmt.Errorf("native follower motion frame rules missing")
	}
	cb := FollowerMotionFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame,
		Move: func(ref NativeRecordReference, _, _ uint16, frame *NativeFrameRegisterContext) error {
			context := frame.CommandContext()
			err := w.commandMove(cleanupRecordAddress(ref), &context)
			frame.SetCommandContext(context)
			return err
		},
	}
	step, err := bindings.Motion.Movement(ref, cb)
	if err != nil {
		return NativeFollowerNext, err
	}
	boundary := step.Boundary
	if boundary == 0x1275a {
		if bindings.Entry == nil {
			return NativeFollowerNext, fmt.Errorf("native movement entry frame rules missing")
		}
		entryState := NativeFollowerEntryFrameState{}
		boundary, err = bindings.Entry.Enter(ref, w.nativeEntryFrameCallbacks(frame, &entryState))
		if err != nil {
			return NativeFollowerNext, err
		}
	}
	if boundary == 0x11ce8 {
		if bindings.Aftermath == nil {
			return NativeFollowerNext, fmt.Errorf("native broken-wall aftermath rules missing")
		}
		return bindings.Aftermath.Tick(ref, w.nativeAftermathFrameCallbacks(frame))
	}
	return w.nativeFollowerFrameContinuation(ref, boundary, frame, state, bindings)
}

func (w *World) nativeCombatFrameBody(ref NativeRecordReference, actorState uint8, frame *NativeFrameRegisterContext, state *NativeFollowerPassState, bindings NativeFollowerFrameBindings) (NativeFollowerPassFlow, error) {
	if bindings.Combat == nil {
		return NativeFollowerNext, fmt.Errorf("native follower combat frame rules missing")
	}
	cb := w.nativeCombatFrameCallbacks(frame, state)
	var step FollowerCombatFrameStep
	var err error
	if actorState == 0x0e {
		step, err = bindings.Combat.Aggressor(ref, cb)
	} else {
		step, err = bindings.Combat.Defender(ref, cb)
	}
	if err != nil {
		return NativeFollowerNext, err
	}
	return w.nativeFollowerFrameContinuation(ref, step.Boundary, frame, state, bindings)
}

func (w *World) nativeFollowerFrameContinuation(ref NativeRecordReference, boundary uint32, frame *NativeFrameRegisterContext, state *NativeFollowerPassState, bindings NativeFollowerFrameBindings) (NativeFollowerPassFlow, error) {
	switch boundary {
	case 0x123b4:
		return NativeFollowerCount, nil
	case 0x12462:
		return NativeFollowerNext, nil
	case 0x112b8:
		return NativeFollowerRedispatch, nil
	default:
		if bindings.Continue != nil {
			return bindings.Continue(ref, boundary, frame, state)
		}
		return NativeFollowerNext, fmt.Errorf("native follower tail%x continuation missing", boundary)
	}
}

// tickNativeFollowerFrame borrows the already authoritative raw session for
// the ordered native pass. Its parent must hydrate typed views after all
// physics stages, rather than flush them between actors or reset the frame.
func (w *World) tickNativeFollowerFrame(rules *NativeFollowerPassRules, frame *NativeFrameRegisterContext, state *NativeFollowerPassState, bindings NativeFollowerFrameBindings) error {
	if w == nil || rules == nil || frame == nil || state == nil {
		return fmt.Errorf("native follower frame context missing")
	}
	return rules.Tick(w.nativeFollowerFrameCallbacks(frame, state, bindings))
}
