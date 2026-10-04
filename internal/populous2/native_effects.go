package populous2

// NativeEffectFrame selects the original animation bank for both drawing and
// audio. A whirlwind and a fire column do not share an animation controller.
func (b *Bundle) NativeEffectFrame(actor NativeEffectActor) (AnimationFrame, bool) {
	if b == nil {
		return AnimationFrame{}, false
	}
	switch actor.Kind {
	case 0x22:
		frame, ok := b.FireColumns.Frames[actor.Animation]
		return frame, ok
	case 0x20:
		frame, ok := b.Whirlwinds.Frames[actor.Animation]
		return frame, ok
	}
	return AnimationFrame{}, false
}

func (b *Bundle) validNativeEffectPhase(actor NativeEffectActor) bool {
	start, length := -1, 0
	switch actor.Kind {
	case 0x22:
		switch actor.State {
		case 2:
			start = 0x1a0
		case 4:
			start = 0x4b8
		case 6:
			start = 0x660
		}
		length = b.FireColumns.SequenceLengths[start]
	case 0x20:
		switch actor.State {
		case 8, 10:
			start = 0x4c8
		case 12:
			start = 0x6cc
		}
		length = b.Whirlwinds.SequenceLengths[start]
	}
	return start >= 0 && actor.Animation >= start && actor.Animation < start+length*4 && (actor.Animation-start)%4 == 0
}
