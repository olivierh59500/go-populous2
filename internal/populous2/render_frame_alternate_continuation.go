package populous2

import "fmt"

// AdvanceAlternateDraw retains C204's clipped variable-grid walk while an
// actor waits. Original CODE scratch strides and full-D wrapper remain live.
func (r *NativeActorRenderRules) AdvanceAlternateDraw(cb NativeWorldRenderCallbacks, state *NativeAlternateRenderState, s *NativeWorldRenderContinuation) (p NativeWorldRenderPlan, complete bool, failure error) {
	p = NativeWorldRenderPlan{NativeActorEffectsPlan: NativeActorEffectsPlan{NativeRenderFramePlan: NativeRenderFramePlan{Drawn: true}}, Actors: []NativeRecordReference{}, TileRequests: []NativeTileChunkRequest{}}
	if r == nil || state == nil || s == nil || cb.Effects.Frame == nil || !winMemoryValid(cb.Effects.Memory) || cb.Tiles == nil || len(cb.Effects.Bitmap) != 32000 {
		return p, false, fmt.Errorf("native retained alternate backing missing")
	}
	if s.Failed != nil {
		return p, false, s.Failed
	}
	if s.Complete {
		return p, true, nil
	}
	c := cb.Effects.Frame
	defer func() {
		s.Registers = c.D
		if failure != nil {
			s.Failed = failure
		}
	}()
	if !s.Started {
		s.Started = true
		s.Saved = c.D
		s.TileTarget = cb.Effects.Bitmap
		address, e := cb.Effects.Memory.Read32(0x1e)
		if e != nil {
			return p, false, e
		}
		s.TileTargetAddress = address
		if e := r.beginAlternateDraw(cb, state, s); e != nil {
			return p, false, e
		}
	} else {
		c.D = s.Registers
	}
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}
	for {
		switch s.Phase {
		case 0:
			if s.Row == s.Rows || s.Grid >= 0x4f44 {
				state.World.ProjectionX, state.World.ProjectionY = s.ProjectionX, s.ProjectionY
				c.D = s.Saved
				s.Complete = true
				return p, true, m.err
			}
			if cb.ResolveTargets != nil {
				if e := cb.ResolveTargets(&cb); e != nil {
					return p, false, e
				}
			}
			skip, e := r.alternateDrawCell(cb, state, s, &p)
			if e != nil {
				return p, false, e
			}
			if skip {
				s.Grid += 2
				s.Phase = 4
				continue
			}
			head := m.word(s.Grid)
			s.Grid += 2
			if m.err != nil {
				return p, false, m.err
			}
			if head == 0 {
				s.Phase = 4
				continue
			}
			s.Ref = NativeRecordReference(head)
			seen := map[NativeRecordReference]bool{}
			for {
				if seen[s.Ref] {
					return p, false, fmt.Errorf("native retained alternate next-link cycle")
				}
				seen[s.Ref] = true
				next := m.word(cleanupRecordAddress(s.Ref) + 2)
				if m.err != nil {
					return p, false, m.err
				}
				if next == 0 {
					break
				}
				s.Ref = NativeRecordReference(next)
			}
			s.Seen = map[NativeRecordReference]bool{}
			s.Phase = 1
		case 1:
			if s.Seen[s.Ref] {
				return p, false, fmt.Errorf("native retained alternate previous-link cycle")
			}
			s.Seen[s.Ref] = true
			s.Projection = NativeProjectedActorContinuation{}
			p.Actors = append(p.Actors, s.Ref)
			s.Phase = 2
		case 2:
			if cb.ResolveTargets != nil {
				if e := cb.ResolveTargets(&cb); e != nil {
					return p, false, e
				}
			}
			q, done, e := r.AdvanceProjectedActor(cleanupRecordAddress(s.Ref), s.Grid, cb, &state.World, &s.Projection)
			appendWorldActorPlan(&p, q)
			if e != nil {
				return p, false, e
			}
			if !done {
				return p, false, nil
			}
			s.Phase = 3
		case 3:
			previous := m.word(cleanupRecordAddress(s.Ref) + 4)
			if m.err != nil {
				return p, false, m.err
			}
			if previous != 0 {
				s.Ref = NativeRecordReference(previous)
				s.Phase = 1
			} else {
				s.Phase = 4
			}
		case 4:
			s.Destination += 322
			s.Column++
			c.Word(6, uint16(c.D[6])+1)
			if s.Column == s.Columns {
				s.Destination += int(int16(state.Scratch[3]))
				s.Grid += int(int16(state.Scratch[4]))
				s.Column = 0
				s.Row++
				c.Word(7, uint16(c.D[7])+1)
				if s.Row < s.Rows {
					c.D[6] = 0
				}
			}
			s.Phase = 0
		default:
			return p, false, fmt.Errorf("native retained alternate phase unavailable")
		}
	}
}
