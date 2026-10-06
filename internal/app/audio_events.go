package app

// AnimationSoundGate admits at most one instance of a named asset cue per
// simulation pass. Drawing the same animation on four display frames cannot
// restart its sound four times, and matching actor cues share one admission.
type AnimationSoundGate struct {
	Tick    uint64
	Started bool
	Played  [133]bool
}

func (g *AnimationSoundGate) Admit(tick uint64, cue int) bool {
	if g == nil || cue <= 0 || cue >= len(g.Played) {
		return false
	}
	if !g.Started || g.Tick != tick {
		g.Tick, g.Started = tick, true
		g.Played = [133]bool{}
	}
	if g.Played[cue] {
		return false
	}
	g.Played[cue] = true
	return true
}

// playAnimationCue uses the original cue associated with the visible artwork
// frame. The cue is an index in the semantic music asset, not a machine address.
func (g *Game) playAnimationCue(name string, frame, actor int) {
	if g.World == nil || g.Assets == nil || g.Assets.Visual == nil || g.music == nil {
		return
	}
	sequence, ok := g.Assets.Visual.Animations[name]
	if !ok || len(sequence.Frames) == 0 {
		return
	}
	if sequence.Loop {
		frame %= len(sequence.Frames)
	}
	if frame < 0 || frame >= len(sequence.Frames) {
		return
	}
	cue := sequence.Frames[frame].SoundCue
	if g.AnimationSounds.Admit(g.World.Tick, cue) {
		g.music.TriggerCue(cue)
	}
}
