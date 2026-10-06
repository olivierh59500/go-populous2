package game

import (
	"fmt"
	"go-populous2/internal/populous2"
)

// resultAdvance retains381E and its actual reset UI across native frames.
// The reset has completed before cache-only LAND refresh; later AI/effects
// consume its new rules while the source frame still owns all raw memory.
func (g *NativeGame) resultAdvance(identity uint16, frame *populous2.NativeFrameRegisterContext) (bool, error) {
	complete, err := g.Result.Advance(g.Host, identity, frame)
	if err != nil || !complete {
		return complete, err
	}
	if g.Result.RefreshPending {
		if err := g.Host.RefreshResultWorldCaches(); err != nil {
			return false, err
		}
		g.Result.RefreshPending = false
		g.networkRefresh = true
	}
	return true, nil
}

func (g *NativeGame) progressionChild(call populous2.NativeStartupResetFrameCall, phase *uint32) (populous2.NativeCommandFrameResult, error) {
	callbacks := populous2.NativeRuntimeProgressionCallbacks{
		Audio: g.Operations, Ownership: g.Startup.Ownership,
		NativeFileFrameCallbacks: populous2.NativeFileFrameCallbacks{Sound: g.Operations.DirectCue},
		Child: func(child populous2.NativeStartupResetFrameCall, childPhase *uint32) (populous2.NativeCommandFrameResult, error) {
			if child.Routine == 0xb740 {
				return g.DeityEditor.AdvanceChild(g.Host, child, childPhase, g.Startup.Campaign)
			}
			switch child.Routine {
			case 0xbaee, 0xb6b6, 0xcd22, 0xd8cc, 0xe0fe, 0xe11a:
				err := populous2.RunNativeProgressionChild(child.Routine, g.Host, child.Frame, child.A)
				return populous2.NativeCommandFrameResult{Complete: err == nil}, err
			default:
				return populous2.NativeCommandFrameResult{}, fmt.Errorf("native progression child%x unavailable", child.Routine)
			}
		},
	}
	return g.Progression.AdvanceChild(g.Host, call, phase, callbacks)
}
