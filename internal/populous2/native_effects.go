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
	case BasaltActorKind:
		frame, ok := b.BasaltRules.Frames[actor.Animation]
		return frame, ok
	case 0x28:
		frame, ok := b.LightningRules.Frames[actor.Animation]
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
	case BasaltActorKind:
		if actor.State == 0x38 || actor.State == 0x3a {
			start, length = 0x5ec, b.BasaltRules.SequenceLength
		}
	}
	return start >= 0 && actor.Animation >= start && actor.Animation < start+length*4 && (actor.Animation-start)%4 == 0
}

// FollowerDeathFrame is shared by the renderer, sound events and saved death
// actors. Their images remain visible after removal from live population.
func (b *Bundle) FollowerDeathFrame(animation int) (AnimationFrame, bool) {
	if b == nil {
		return AnimationFrame{}, false
	}
	if frame, ok := b.FireColumns.Frames[animation]; ok {
		return frame, true
	}
	frame, ok := b.FungusHazards.Frames[animation]
	return frame, ok
}

func (b *Bundle) validFollowerDeathSpan(animation, end int) bool {
	span := func(start, length int) bool {
		return length > 0 && end == start+length*4 && animation >= start && animation < end && (animation-start)%4 == 0
	}
	for _, start := range append([]int{0x178, 0x2bd4}, b.FireColumns.TownDeath[:]...) {
		if span(start, b.FireColumns.SequenceLengths[start]) {
			return true
		}
	}
	for start, length := range b.FungusHazards.SequenceLengths {
		if span(start, length) {
			return true
		}
	}
	return false
}
