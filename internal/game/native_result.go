package game

import "go-populous2/internal/populous2"

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
