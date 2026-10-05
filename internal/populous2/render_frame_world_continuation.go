package populous2

import "fmt"

// NativeProjectedActorContinuation owns $e35e's full MOVEM save and the
// geometry computed before $e45c. The actor may wait inside $314a without
// rerunning projection or reading the actor's dispatch bytes again.
type NativeProjectedActorContinuation struct {
	Started, Complete bool
	At, Grid          int
	Saved             [8]uint32
	Actor             NativeActorRenderContinuation
	Failed            error
}

func (r *NativeActorRenderRules) AdvanceProjectedActor(at, grid int, cb NativeWorldRenderCallbacks, state *NativeWorldRenderState, s *NativeProjectedActorContinuation) (NativeActorEffectsPlan, bool, error) {
	p := NativeActorEffectsPlan{}
	if r == nil || state == nil || s == nil || cb.Effects.Frame == nil {
		return p, false, fmt.Errorf("native projected actor continuation missing")
	}
	if s.Failed != nil {
		return p, false, s.Failed
	}
	if s.Complete {
		return p, true, nil
	}
	if s.Started && (s.At != at || s.Grid != grid) {
		return p, false, fmt.Errorf("native projected actor caller changed across wait")
	}
	if !s.Started {
		s.Started = true
		s.At, s.Grid = at, grid
		s.Saved = cb.Effects.Frame.D
		if e := r.projectActorCoordinates(at, grid, cb, state); e != nil {
			s.Failed = e
			return p, false, e
		}
	}
	actor := cb.Effects
	actor.GridCursorAddress = s.Grid
	p, done, e := r.AdvanceActor(s.At, actor, &state.Actor, &s.Actor, cb.Children)
	if e != nil {
		s.Failed = e
		return p, false, e
	}
	if done {
		cb.Effects.Frame.D = s.Saved
		s.Complete = true
	}
	return p, done, nil
}

// NativeWorldRenderContinuation retains original traversal pointers and
// loop counters. TileTarget is the source A6 bitmap loaded at entry, while
// actor image targets may change through BSS1E during a genuine modal.
type NativeWorldRenderContinuation struct {
	Started, Complete              bool
	Phase                          uint8
	Row, Column, Grid, Destination int
	Rows, Columns                  int
	ProjectionX, ProjectionY       uint16
	Saved, Registers               [8]uint32
	Ref                            NativeRecordReference
	Seen                           map[NativeRecordReference]bool
	Projection                     NativeProjectedActorContinuation
	TileTarget                     []byte
	TileTargetAddress              uint32 // Original A6 loaded from BSS1E at entry.
	Failed                         error
}

func appendWorldActorPlan(p *NativeWorldRenderPlan, q NativeActorEffectsPlan) {
	p.Sprites = append(p.Sprites, q.Sprites...)
	p.Crops = append(p.Crops, q.Crops...)
	p.Pixels = append(p.Pixels, q.Pixels...)
	p.HardwarePending = p.HardwarePending || q.HardwarePending
}

// AdvanceWorldDraw resumes the original BBE0 grid/list walk and emits only
// this invocation's work. Actor projection and dispatch never replay on wait.
func (r *NativeActorRenderRules) AdvanceWorldDraw(cb NativeWorldRenderCallbacks, state *NativeWorldRenderState, s *NativeWorldRenderContinuation) (p NativeWorldRenderPlan, complete bool, failure error) {
	p = NativeWorldRenderPlan{NativeActorEffectsPlan: NativeActorEffectsPlan{NativeRenderFramePlan: NativeRenderFramePlan{Drawn: true}}, Actors: []NativeRecordReference{}, TileRequests: []NativeTileChunkRequest{}}
	if r == nil || state == nil || s == nil || cb.Effects.Frame == nil || !winMemoryValid(cb.Effects.Memory) || cb.Tiles == nil || len(cb.Effects.Bitmap) != 32000 || len(cb.Background) != 32000 {
		return p, false, fmt.Errorf("native retained world renderer backing missing")
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
		if e := r.beginWorldDraw(cb, state, s); e != nil {
			return p, false, e
		}
	} else {
		c.D = s.Registers
	}
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}
	for {
		switch s.Phase {
		case 0:
			if s.Row == 8 {
				c.D = s.Saved
				s.Complete = true
				return p, true, m.err
			}
			if cb.ResolveTargets != nil {
				if e := cb.ResolveTargets(&cb); e != nil {
					return p, false, e
				}
			}
			cell := cb
			if e := r.worldDrawCell(cell, state, s, &p); e != nil {
				return p, false, e
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
					return p, false, fmt.Errorf("native retained world next-link cycle")
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
				return p, false, fmt.Errorf("native retained world previous-link cycle")
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
			q, done, e := r.AdvanceProjectedActor(cleanupRecordAddress(s.Ref), s.Grid, cb, state, &s.Projection)
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
			if s.Column == 8 {
				s.Destination += 318 - 8*322
				s.Grid += 224
				s.Column = 0
				s.Row++
				c.Word(7, uint16(c.D[7])+1)
				if s.Row < 8 {
					c.D[6] = 0
				}
			}
			s.Phase = 0
		default:
			return p, false, fmt.Errorf("native retained world phase unavailable")
		}
	}
}
