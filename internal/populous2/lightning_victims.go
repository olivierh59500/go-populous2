package populous2

import "fmt"

// LightningVictim carries only fields changed or inspected by the original
// strike and victim handlers. Next and Owner remain raw native metadata.
type LightningVictim struct {
	Kind, Owner, Flags, State uint8
	Next                      NativeRecordReference
	HeroType                  uint16
	Animation                 int
	Population                int32
	EffectReference           NativeRecordReference
}

func (r *LightningRules) tickBolt(state *LightningState, pool *[NativeEffectCapacity]NativeEffectActor, slot int, cb LightningCallbacks) error {
	if cb.Random == nil {
		return fmt.Errorf("lightning bolt RNG missing")
	}
	a := &pool[slot]
	random := uint16(cb.Random())
	state.RandomWords[slot] = random
	a.X = (a.X &^ 255) | int16(uint8(random>>2))
	x, y := int(a.X)>>8, int(a.Y)>>8
	var head NativeRecordReference
	if cb.Head != nil {
		head = cb.Head(x, y)
	}
	seen := make(map[NativeRecordReference]bool)
	for head != 0 {
		if cb.Record == nil || cb.SetRecord == nil || seen[head] {
			return fmt.Errorf("invalid lightning occupancy callbacks/chain")
		}
		seen[head] = true
		v, ok := cb.Record(head)
		if !ok {
			return fmt.Errorf("unknown lightning victim reference")
		}
		if v.Kind == 26 {
			return nil
		} // A wall aborts the rest of the scan and scorching.
		changed := false
		if v.Kind == 2 {
			if v.State != 0x3a && v.State != 0x1c {
				animation := 0x738
				if v.Flags&2 != 0 {
					if v.HeroType%2 != 0 || v.HeroType/2 >= 6 {
						return fmt.Errorf("invalid lightning hero type")
					}
					animation = r.HeroHit[v.HeroType/2]
				}
				if animation != 0 {
					v.State, v.Animation = 0x1c, animation
				}
			}
			v.EffectReference = lightningReference(slot)
			changed = true
		} else if v.Kind == 4 {
			if v.State != 0x30 && v.State != 0x1e {
				v.State, v.Animation = 0x1e, 0x744
			}
			v.EffectReference = lightningReference(slot)
			changed = true
		}
		next := v.Next
		if changed {
			cb.SetRecord(head, v)
		}
		head = next
	}
	if cb.Burn != nil {
		cb.Burn(x, y)
	}
	return nil
}

type LightningVictimCallbacks struct {
	BoltAlive func(NativeRecordReference) bool
	// Cleanup translates $124a2 with mode1 for a walker's retained death image
	// and mode0 for a finished town. Death-frame final removal is separate.
	Cleanup    func(bool)
	Remove     func()
	ReformTown func()
}

type LightningVictimStep struct{ NeedsDecision bool }

func (r *LightningRules) loopVictim(v *LightningVictim) error {
	next := v.Animation + 4
	if _, ok := r.Frames[next]; !ok {
		start := -1
		for base, count := range r.SequenceLengths {
			if v.Animation >= base && v.Animation < base+count*4 {
				start = base
				break
			}
		}
		if start < 0 {
			return fmt.Errorf("unknown lightning victim animation")
		}
		next += r.LoopOffsets[start]
	}
	v.Animation = next
	return nil
}

// TickVictim translates the gradual lightning states $1c/$1e and their
// ordinary recovery/death images. Positive population loses (pop<<3)>>7+4;
// the signed 32-bit shift/wrap is retained rather than widened arithmetic.
func (r *LightningRules) TickVictim(v *LightningVictim, cb LightningVictimCallbacks) (LightningVictimStep, error) {
	var step LightningVictimStep
	if r == nil || v == nil {
		return step, fmt.Errorf("lightning victim missing")
	}
	if v.State == 0x20 || v.State == 0x22 {
		next := v.Animation + 4
		if _, ok := r.Frames[next]; ok {
			v.Animation = next
			return step, nil
		}
		if v.State == 0x20 {
			if cb.Remove != nil {
				cb.Remove()
			}
			return step, nil
		}
		v.State, v.Animation = 2, 0
		step.NeedsDecision = true
		return step, nil
	}
	if v.State != 0x1c && v.State != 0x1e {
		return step, nil
	}
	if err := r.loopVictim(v); err != nil {
		return step, err
	}
	if v.Population > 0 {
		shifted := int32(uint32(v.Population) << r.DamageShift)
		loss := (shifted >> 7) + 4
		v.Population -= loss
	}
	if cb.BoltAlive == nil {
		return step, fmt.Errorf("lightning victim bolt lookup missing")
	}
	if cb.BoltAlive(v.EffectReference) {
		return step, nil
	}
	if v.State == 0x1e {
		if v.Population <= 0 {
			if cb.Cleanup != nil {
				cb.Cleanup(false)
			}
		} else if cb.ReformTown != nil {
			cb.ReformTown()
		}
		return step, nil
	}
	if v.Population <= 0 {
		v.State = 0x20
		animation := 0x738
		if v.Flags&2 != 0 {
			if v.HeroType%2 != 0 || v.HeroType/2 >= 6 {
				return step, fmt.Errorf("invalid lightning hero")
			}
			animation = r.HeroDeath[v.HeroType/2]
		}
		if animation != 0 {
			v.Animation = animation
			if cb.Cleanup != nil {
				cb.Cleanup(true)
			}
		}
	} else {
		v.State = 0x22
		animation := 0x750
		if v.Flags&2 != 0 {
			if v.HeroType%2 != 0 || v.HeroType/2 >= 6 {
				return step, fmt.Errorf("invalid lightning hero")
			}
			animation = r.HeroRecovery[v.HeroType/2]
		}
		if animation != 0 {
			v.Animation = animation
		}
	}
	return step, nil
}

type LightningPoint struct{ X, Y int16 }
type LightningSegment struct {
	From, To     LightningPoint
	PaletteIndex uint8
}

// MarkerEndpoint follows $ebfc-$ec4e. The height is the current bolt cell's
// Header low bits, as read at -4(A4), not the marker cell's interpolated height.
func LightningMarkerEndpoint(markerX, markerY, cameraX, cameraY int, boltHeader uint8) LightningPoint {
	x, y := markerX-cameraX, markerY-cameraY
	return LightningPoint{X: int16(192 + 16*(x-y)), Y: int16(max(0, 72+8*(x+y)-8*int(boltHeader&7)-90))}
}

// BeamSegments translates $ec50-$ecd2 after the caller projects the bolt and
// raised marker endpoint. Three alternating horizontal kinks divide the beam
// into four native lines, all drawn with original palette index5.
func LightningBeamSegments(from, to LightningPoint, random uint16, tick uint16) [4]LightningSegment {
	var lines [4]LightningSegment
	dx, dy := int16(to.X-from.X)>>2, int16(to.Y-from.Y)>>2
	current := from
	direction := int16(1)
	if tick&1 != 0 {
		direction = -1
	}
	for i := 0; i < 3; i++ {
		next := LightningPoint{X: current.X + dx, Y: current.Y + dy}
		jitter := int16(random&7) + 2
		random >>= 1
		direction = -direction
		if direction < 0 {
			jitter = -jitter
		}
		next.X += jitter
		lines[i] = LightningSegment{From: current, To: next, PaletteIndex: 5}
		current = next
	}
	lines[3] = LightningSegment{From: current, To: to, PaletteIndex: 5}
	return lines
}
