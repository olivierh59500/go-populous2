package populous2

import "fmt"

type NativeFrameStage uint8

const (
	NativeFrameBegin NativeFrameStage = iota
	NativeFrameFollowers
	NativeFrameAI
	NativeFrameFX
	NativeFrameWalls
	NativeFrameForest
	NativeFrameScenario
	NativeFrameAudio
	NativeFrameSwap
	NativeFrameCommands
	NativeFrameFinished
)

// NativeFramePassState retains the source stage while a caller-owned modal or
// transport operation is pending. Its callback owns any inner continuation;
// prior physics/audio stages must not be replayed on the next resume.
type NativeFramePassState struct {
	Stage             NativeFrameStage
	Paused, SkipEarly bool
}

type NativeFrameStageCallback func(*NativeFrameRegisterContext) (bool, error)

type NativeFrameCallbacks struct {
	Memory                                                            FollowerCleanupMemory
	Followers, AI, FX, Walls, Forest, Scenario, Audio, Swap, Commands NativeFrameStageCallback
}

// TickFramePass follows $10b6-$1108. F3C skips every physics stage and audio;
// F3E skips only followers/AI/FX. Swap and deferred commands follow both gates.
// OS lock/unlock and $1a58e preserve all data registers at these boundaries.
func (s *NativeFramePassState) TickFramePass(c *NativeFrameRegisterContext, cb NativeFrameCallbacks) (bool, error) {
	if s == nil || c == nil || cb.Memory.Read16 == nil {
		return false, fmt.Errorf("native frame pass state/context missing")
	}
	if s.Stage == NativeFrameBegin {
		pause, e := cb.Memory.Read16(0xf3c)
		if e != nil {
			return false, e
		}
		s.Paused = pause != 0
		if s.Paused {
			s.Stage = NativeFrameSwap
		} else {
			skip, e := cb.Memory.Read16(0xf3e)
			if e != nil {
				return false, e
			}
			s.SkipEarly = skip != 0
			s.Stage = NativeFrameFollowers
			if s.SkipEarly {
				s.Stage = NativeFrameWalls
			}
		}
	}
	for s.Stage < NativeFrameFinished {
		var callback NativeFrameStageCallback
		switch s.Stage {
		case NativeFrameFollowers:
			callback = cb.Followers
		case NativeFrameAI:
			callback = cb.AI
		case NativeFrameFX:
			callback = cb.FX
		case NativeFrameWalls:
			callback = cb.Walls
		case NativeFrameForest:
			callback = cb.Forest
		case NativeFrameScenario:
			callback = cb.Scenario
		case NativeFrameAudio:
			callback = cb.Audio
		case NativeFrameSwap:
			callback = cb.Swap
		case NativeFrameCommands:
			callback = cb.Commands
		default:
			return false, fmt.Errorf("native frame stage%d missing", s.Stage)
		}
		if callback == nil {
			return false, fmt.Errorf("native frame stage%d callback missing", s.Stage)
		}
		complete, e := callback(c)
		if e != nil {
			return false, e
		}
		if !complete {
			return false, nil
		}
		s.Stage++
	}
	return true, nil
}

type NativeFrameContinuationBindings struct {
	World                     NativeFrameWorldBindings
	Followers, Swap, Commands NativeFrameStageCallback
	Audio                     NativeFrameAudioCallbacks
}

// nativeFrameCallbacks supplies concrete register-bearing World bodies. The
// caller supplies real follower/presentation/command continuations, including
// inner resumable modal state, and retains the audio software bank across ticks.
func (w *World) nativeFrameCallbacks(bindings NativeFrameContinuationBindings) NativeFrameCallbacks {
	immediate := func(body func(*NativeFrameRegisterContext) error) NativeFrameStageCallback {
		return func(c *NativeFrameRegisterContext) (bool, error) { e := body(c); return e == nil, e }
	}
	return NativeFrameCallbacks{Memory: w.nativeCleanupMemory(), Followers: bindings.Followers, Swap: bindings.Swap, Commands: bindings.Commands,
		AI: immediate(func(c *NativeFrameRegisterContext) error {
			_, e := w.NativeAI.TickFrame(c, w.nativeAICallbacks(nil))
			return e
		}),
		FX: immediate(func(c *NativeFrameRegisterContext) error { return w.tickNativeFrameFX(c, bindings.World) }),
		Walls: immediate(func(c *NativeFrameRegisterContext) error {
			if bindings.World.Commands.WallRules == nil {
				return fmt.Errorf("native frame Wall rules missing")
			}
			cb := w.nativeWallCallbacks(uint16(c.D[7]))
			cb.Frame = c
			_, e := bindings.World.Commands.WallRules.TickPool(cb)
			return e
		}),
		Forest: immediate(func(c *NativeFrameRegisterContext) error {
			cb := w.nativeForestCallbacks()
			cb.Frame = c
			clock, e := cb.Memory.Read16(0xf42)
			if e != nil {
				return e
			}
			for slot := 0; slot < SceneryCapacity; slot++ {
				if _, e := w.ForestNative.Tick(nativeActorReference(NativeSceneryPool, slot), clock, cb); e != nil {
					return e
				}
			}
			return nil
		}),
		Scenario: immediate(func(c *NativeFrameRegisterContext) error {
			_, e := w.tickNativeFrameScenario(c, bindings.World)
			return e
		}),
		Audio: immediate(func(c *NativeFrameRegisterContext) error {
			if bindings.World.Audio == nil {
				return fmt.Errorf("native frame Audio bank missing")
			}
			return bindings.World.Audio.TickAudio(c, bindings.Audio)
		}),
	}
}
