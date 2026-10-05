package populous2

import "fmt"

// NativeActorRenderContinuation retains the source actor call while its $314a
// modal is waiting. PC is $314a during the child and $e744 at its return. The
// $e8d0 town-and-animation branch also retains its actual MOVEM.L D0/D1 values.
type NativeActorRenderContinuation struct {
	Started, Complete bool
	At, PC, Target    int
	Registers         [8]uint32
	SavedXY           [2]uint32
	Failed            error
}

// AdvanceActor emits only the drawing performed by this invocation. A false
// complete result leaves the genuine $314a call pending: subsequent calls do
// not repeat dispatch, town admission, or any previously emitted pixels.
func (r *NativeActorRenderRules) AdvanceActor(at int, cb NativeActorEffectsCallbacks, state *NativeActorRenderState, continuation *NativeActorRenderContinuation, children NativeActorRenderChildren) (NativeActorEffectsPlan, bool, error) {
	p := NativeActorEffectsPlan{NativeRenderFramePlan: NativeRenderFramePlan{Drawn: true, Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}, Crops: []NativeCroppedSpriteRequest{}}
	if r == nil || cb.Frame == nil || cb.Image == nil || state == nil || continuation == nil || !winMemoryValid(cb.Memory) {
		return p, false, fmt.Errorf("native retained actor renderer backing missing")
	}
	s := continuation
	if s.Failed != nil {
		return p, false, s.Failed
	}
	if s.Complete {
		return p, true, nil
	}
	if s.Started && s.At != at {
		return p, false, fmt.Errorf("native retained actor changed during modal")
	}
	c := cb.Frame
	finish := func(err error) (NativeActorEffectsPlan, bool, error) {
		s.Registers = c.D
		s.Failed = err
		if err == nil {
			s.Complete = true
			s.PC = 0xee30
		}
		return p, s.Complete, err
	}
	if !s.Started {
		s.Started, s.At, s.PC = true, at, 0xe45c
		m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
		c.D[2] = 0
		c.Byte(2, m.byte(at))
		branch, err := r.word(0xe46a + int(int16(c.D[2])))
		if err != nil {
			return finish(err)
		}
		c.Word(2, branch)
		target := 0xe46a + int(int16(c.D[2]))
		if m.err != nil {
			return finish(m.err)
		}
		if target != 0xe4a8 {
			p, err = r.actorBranch(at, target, cb, state, children)
			return finish(err)
		}
		c.D[2] = 0
		c.Byte(2, m.byte(at+22))
		branch, err = r.word(0xe4ba + int(int16(c.D[2])))
		if err != nil {
			return finish(err)
		}
		c.Word(2, branch)
		s.Target = 0xe4ba + int(int16(c.D[2]))
		if m.err != nil {
			return finish(m.err)
		}
		town := s.Target == 0xe72e || s.Target == 0xe8d0 || s.Target == 0xe506 && m.byte(at) == 4
		if !town {
			q, err := r.followerBranch(at, s.Target, cb.NativeRenderFrameCallbacks, state, children)
			p.NativeRenderFramePlan = q
			return finish(err)
		}
		if s.Target == 0xe8d0 {
			s.SavedXY = [2]uint32{c.D[0], c.D[1]}
		}
		s.PC = 0xe744
		if m.word(0x3b8) == 0 && m.word(0x3b0) != 0 {
			s.PC = 0x314a
		}
		if m.err != nil {
			return finish(m.err)
		}
		s.Registers = c.D
	} else {
		c.D = s.Registers
	}
	if s.PC == 0x314a {
		if children.TownInfoAdvance != nil {
			complete, err := children.TownInfoAdvance(at, c)
			s.Registers = c.D
			if err != nil {
				return finish(err)
			}
			if !complete {
				return p, false, nil
			}
		} else if children.TownInfo != nil {
			if err := children.TownInfo(at, c); err != nil {
				return finish(err)
			}
		} else {
			return finish(fmt.Errorf("native town info child314a missing"))
		}
		if children.RefreshTargets != nil {
			if err := children.RefreshTargets(&cb); err != nil {
				return finish(err)
			}
		}
		s.PC = 0xe744
	}
	q, err := r.townBody(at, cb.NativeRenderFrameCallbacks, state)
	p.NativeRenderFramePlan = q
	if err != nil {
		return finish(err)
	}
	if s.Target == 0xe8d0 {
		c.D[0], c.D[1] = s.SavedXY[0], s.SavedXY[1] // $e8da MOVEM.L
		c.Word(1, uint16(c.D[1])-8)
		c.Word(1, uint16(c.D[1])+8)
		m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
		c.Word(2, m.word(at+10))
		if m.err != nil {
			return finish(m.err)
		}
		if err := r.image(cb.NativeRenderFrameCallbacks, &p.NativeRenderFramePlan); err != nil {
			return finish(err)
		}
	}
	return finish(nil)
}
