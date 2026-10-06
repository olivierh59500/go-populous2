package populous2

// These portable host periods retain 50 Hz PAL input, interrupts and audio,
// while allowing for the work performed between original screen swaps. The
// source CPU audit measures roughly 56–78 ms for an initial gameplay pass and
// 88–105 ms for a help preview. They are presentation pacing defaults, not a
// claim of cycle-exact 68000 and shared chip-memory emulation.
const (
	NativeHostGameplayPeriod uint64 = 4
	NativeHostHelpPeriod     uint64 = 5
)

// NativeHostCadence admits a new source pass at a host VBlank boundary. A
// retained source child must continue independently of this admission gate;
// neither source counters nor input latches are modified here.
type NativeHostCadence struct {
	last, period uint64
	started      bool
}

// Ready permits the first pass immediately. Returning from a long modal or
// pause admits one pass, without running missed simulation passes in a burst.
// A changed period or restarted host clock starts a new schedule.
func (s *NativeHostCadence) Ready(vblank, period uint64) bool {
	if period == 0 {
		period = 1
	}
	if !s.started || s.period != period || vblank < s.last {
		s.started, s.last, s.period = true, vblank, period
		return true
	}
	if vblank-s.last < period {
		return false
	}
	s.last = vblank
	return true
}
