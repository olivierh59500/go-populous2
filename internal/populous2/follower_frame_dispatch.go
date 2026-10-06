package populous2

import "fmt"

type NativeFollowerFrameBindings struct {
	Aftermath *NativeFollowerAftermathFrameRules
	Motion    *FollowerMotionFrameRules
	Entry     *NativeFollowerEntryFrameRules
	Combat    *FollowerCombatFrameRules
	Town      *NativeFollowerTownFrameRules
	// TownState retains the evaluator property cache and parcel scratch across
	// actors. Only its outer flag and minimap variant reset at dispatch.
	TownState   *NativeTownFrameState
	Decision    *FollowerDecisionFrameRules
	Hero        *NativeFollowerHeroFrameRules
	Magnet      *NativeFollowerMagnetFrameRules
	Terrain     *NativeFollowerTerrainFrameRules
	WaitContact *NativeFollowerWaitContactFrameRules
	Siege       *NativeFollowerSiegeFrameRules
	Retained    *NativeFollowerRetainedFrameRules
	Neutral     *NativeFollowerNeutralFrameRules
	Commands    NativeCommandWorldBindings
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
			if actorState == 2 {
				return w.nativeFollowerFrameContinuation(ref, 0x1131c, frame, state, bindings)
			}
			if actorState == 0x44 {
				if bindings.Neutral == nil {
					return NativeFollowerNext, fmt.Errorf("native neutral follower frame rules missing")
				}
				boundary, err := bindings.Neutral.Tick(ref, w.nativeNeutralFrameCallbacks(frame, bindings.Commands))
				if err != nil {
					return NativeFollowerNext, err
				}
				return w.nativeFollowerFrameContinuation(ref, boundary, frame, state, bindings)
			}
			if actorState == 0x34 || actorState == 0x46 {
				if bindings.Retained == nil || bindings.Hero == nil {
					return NativeFollowerNext, fmt.Errorf("native captive/ruin frame rules missing")
				}
				boundary, err := bindings.Retained.Tick(ref, w.nativeRetainedFrameCallbacks(frame, bindings.Hero))
				if err != nil {
					return NativeFollowerNext, err
				}
				return w.nativeFollowerFrameContinuation(ref, boundary, frame, state, bindings)
			}
			if actorState == 0x1c || actorState == 0x1e || actorState == 0x22 {
				if bindings.Siege == nil {
					return NativeFollowerNext, fmt.Errorf("native follower siege frame rules missing")
				}
				boundary, err := bindings.Siege.Tick(ref, w.nativeSiegeFrameCallbacks(frame))
				if err != nil {
					return NativeFollowerNext, err
				}
				return w.nativeFollowerFrameContinuation(ref, boundary, frame, state, bindings)
			}
			if actorState == 0x0a || actorState == 0x0c {
				if bindings.WaitContact == nil {
					return NativeFollowerNext, fmt.Errorf("native follower waiting/contact frame rules missing")
				}
				cb := w.nativeWaitContactFrameCallbacks(frame)
				var step NativeFollowerWaitContactFrameStep
				if actorState == 0x0a {
					step, err = bindings.WaitContact.TickWaiting(ref, cb)
				} else {
					step, err = bindings.WaitContact.CompleteContact(ref, cb)
				}
				if err != nil {
					return NativeFollowerNext, err
				}
				return w.nativeFollowerFrameContinuation(ref, step.Continuation, frame, state, bindings)
			}
			if actorState == 0x16 || actorState == 0x36 || actorState == 0x3c {
				if bindings.Terrain == nil {
					return NativeFollowerNext, fmt.Errorf("native follower terrain frame rules missing")
				}
				boundary, err := bindings.Terrain.Tick(ref, w.nativeTerrainFrameCallbacks(frame))
				if err != nil {
					return NativeFollowerNext, err
				}
				return w.nativeFollowerFrameContinuation(ref, boundary, frame, state, bindings)
			}
			if actorState == 0x24 || actorState == 0x26 {
				if bindings.Hero == nil {
					return NativeFollowerNext, fmt.Errorf("native follower hero frame rules missing")
				}
				step, err := bindings.Hero.Tick(ref, w.nativeHeroFrameCallbacks(frame))
				if err != nil {
					return NativeFollowerNext, err
				}
				return w.nativeFollowerFrameContinuation(step.RedispatchSource, step.Continuation, frame, state, bindings)
			}
			if actorState == 0x12 || actorState == 0x3a {
				if bindings.Magnet == nil || bindings.Hero == nil {
					return NativeFollowerNext, fmt.Errorf("native follower magnet/planner frame rules missing")
				}
				boundary, err := bindings.Magnet.Tick(ref, w.nativeMagnetFrameCallbacks(frame, bindings.Hero))
				if err != nil {
					return NativeFollowerNext, err
				}
				return w.nativeFollowerFrameContinuation(ref, boundary, frame, state, bindings)
			}
			if actorState == 6 {
				return w.nativeTownFrameBody(ref, frame, state, bindings)
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
	case 0x11738:
		return w.nativeTownFrameBody(ref, frame, state, bindings)
	case 0x1131c:
		if bindings.Decision == nil {
			return NativeFollowerNext, fmt.Errorf("native follower search frame rules missing")
		}
		step, err := bindings.Decision.Search(ref, FollowerDecisionFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: frame, AttritionFrame: w.nativeFollowerAttritionFrame})
		if err != nil {
			return NativeFollowerNext, err
		}
		return w.nativeFollowerFrameContinuation(ref, step.Boundary, frame, state, bindings)
	case 0x1156c:
		return w.nativeMotionFrameBody(ref, frame, state, bindings)
	case 0x12044:
		if bindings.Hero == nil {
			return NativeFollowerNext, fmt.Errorf("native follower hero selection frame rules missing")
		}
		_, err := bindings.Hero.Select(ref, w.nativeHeroFrameCallbacks(frame))
		return NativeFollowerCount, err
	case 0x1204e:
		if bindings.Hero == nil {
			return NativeFollowerNext, fmt.Errorf("native follower hero chase frame rules missing")
		}
		step, err := bindings.Hero.Chase(ref, w.nativeHeroFrameCallbacks(frame))
		if err != nil {
			return NativeFollowerNext, err
		}
		return w.nativeFollowerFrameContinuation(step.RedispatchSource, step.Continuation, frame, state, bindings)
	case 0x11bb4:
		if bindings.Magnet == nil || bindings.Hero == nil {
			return NativeFollowerNext, fmt.Errorf("native follower search magnet/planner frame rules missing")
		}
		next, err := bindings.Magnet.Start(ref, w.nativeMagnetFrameCallbacks(frame, bindings.Hero))
		if err != nil {
			return NativeFollowerNext, err
		}
		return w.nativeFollowerFrameContinuation(ref, next, frame, state, bindings)
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

// nativeTownFrameBody also receives the direct founder→11738 tail. That
// source jump must not perform another common prepass or skip this town tick.
func (w *World) nativeTownFrameBody(ref NativeRecordReference, frame *NativeFrameRegisterContext, state *NativeFollowerPassState, bindings NativeFollowerFrameBindings) (NativeFollowerPassFlow, error) {
	if bindings.Town == nil || bindings.TownState == nil {
		return NativeFollowerNext, fmt.Errorf("native follower town frame rules/state missing")
	}
	bindings.TownState.OuterFlag13350 = state.TownCacheFlag
	bindings.TownState.MinimapVariant = state.MinimapVariant
	step, err := bindings.Town.Tick(ref, w.nativeTownFrameCallbacks(frame, bindings.TownState))
	state.TownCacheFlag = bindings.TownState.OuterFlag13350
	state.MinimapVariant = bindings.TownState.MinimapVariant
	if err != nil {
		return NativeFollowerNext, err
	}
	return w.nativeFollowerFrameContinuation(ref, step.Continuation, frame, state, bindings)
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
