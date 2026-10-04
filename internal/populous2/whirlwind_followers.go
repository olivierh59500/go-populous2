package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

// WhirlwindFollower retains the native fields inspected or changed by lifted
// transport and landing. Owner is the original nonzero allocation byte, not a
// derived positive-population test. Unrelated record fields remain caller-owned.
type WhirlwindFollower struct {
	Kind, Owner, Flags, State, Weapon uint8
	Next, Previous                    NativeRecordReference
	X, Y                              uint16
	Animation                         int
	Population                        int32
	EffectReference                   NativeRecordReference
}

type WhirlwindFollowerRules struct {
	Frames          map[int]AnimationFrame
	SequenceLengths map[int]int
	LoopAt          map[int]int
	LandingEnd      int
}

func DecodeWhirlwindFollowerRules(exe *amiga.Executable) (WhirlwindFollowerRules, error) {
	var r WhirlwindFollowerRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a+0x2ad0 {
		return r, fmt.Errorf("native whirlwind follower animations missing")
	}
	code := exe.Hunks[0].Data
	r.Frames = make(map[int]AnimationFrame)
	r.SequenceLengths = make(map[int]int)
	r.LoopAt = make(map[int]int)
	starts := []int{0x4d4, 0x68c}
	for i := 0; i < 6; i++ {
		start := int(binary.BigEndian.Uint16(code[0x20a6c+i*2:]))
		if start != 0 {
			starts = append(starts, start)
		}
	}
	for _, start := range starts {
		if r.SequenceLengths[start] != 0 {
			continue
		}
		frames, err := decodeWhirlwindFollowerAnimation(code, start)
		if err != nil {
			return WhirlwindFollowerRules{}, err
		}
		r.SequenceLengths[start] = len(frames)
		end := start + len(frames)*4
		loop := int(int16(binary.BigEndian.Uint16(code[0x23d1a+end:])))
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
			r.LoopAt[start+i*4] = loop
		}
		if start == 0x68c {
			r.LandingEnd = end
		}
	}
	return r, nil
}

func decodeWhirlwindFollowerAnimation(code []byte, start int) ([]AnimationFrame, error) {
	frames := []AnimationFrame{}
	for index := 0; index < 256; index++ {
		at := 0x23d1a + start + index*4
		if at+4 > len(code) {
			return nil, fmt.Errorf("lifted animation exceeds CODE")
		}
		image := int16(binary.BigEndian.Uint16(code[at:]))
		if image < 0 {
			if len(frames) == 0 {
				return nil, fmt.Errorf("empty lifted animation")
			}
			return frames, nil
		}
		layers, err := decodeImageLayers(code, uint16(image))
		if err != nil {
			return nil, err
		}
		cue := int(binary.BigEndian.Uint16(code[at+2:]))
		// $ee5e ignores oversized cue offsets. Adonis's second lifted
		// frame has the original word $3f9c; it must not reject its image.
		if cue >= 0x532 {
			cue = 0
		}
		if cue%10 != 0 {
			return nil, fmt.Errorf("unaligned lifted animation cue")
		}
		frames = append(frames, AnimationFrame{Layers: layers, SoundCue: cue / 10})
	}
	return nil, fmt.Errorf("unterminated lifted animation")
}

type WhirlwindFollowerCallbacks struct {
	// ReadSource reads native words6/8 at the raw reference, including an inactive
	// or reused source. The original handler has no class/owner/liveness guard.
	ReadSource func(NativeRecordReference) (uint16, uint16, error)
	// Move executes $12518 on every lifted dispatch, even within the same cell.
	// The caller can update links; same-cell writes do not add movement pressure.
	Move func(*WhirlwindFollower, uint16, uint16) error
}

type WhirlwindFollowerStep struct{ Moving, ReadyNextUpdate bool }

func (r *WhirlwindFollowerRules) Tick(f *WhirlwindFollower, cb WhirlwindFollowerCallbacks) (WhirlwindFollowerStep, error) {
	var result WhirlwindFollowerStep
	if r == nil || f == nil {
		return result, fmt.Errorf("whirlwind follower missing")
	}
	if f.Owner == 0 {
		return result, nil
	}
	switch f.State {
	case 0x14:
		if _, ok := r.Frames[f.Animation]; !ok {
			return result, fmt.Errorf("unknown lifted animation")
		}
		next := f.Animation + 4
		if _, ok := r.Frames[next]; !ok {
			next += r.LoopAt[f.Animation]
		}
		f.Animation = next
		if cb.ReadSource == nil {
			return result, fmt.Errorf("lifted source callback missing")
		}
		x, y, err := cb.ReadSource(f.EffectReference)
		if err != nil {
			return result, err
		}
		if cb.Move != nil {
			if err := cb.Move(f, x, y); err != nil {
				return result, err
			}
		}
		f.X, f.Y = x, y
		result.Moving = true
	case 0x1a:
		if f.Animation < 0x68c || f.Animation >= r.LandingEnd || f.Animation%4 != 0 {
			return result, fmt.Errorf("invalid native landing animation")
		}
		next := f.Animation + 4
		if next >= r.LandingEnd {
			f.Kind, f.State, f.Animation = 2, 2, 0
			result.ReadyNextUpdate = true
		} else {
			f.Animation = next
		}
	}
	return result, nil
}
