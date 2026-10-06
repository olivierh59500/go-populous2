package music

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
)

type audioEnvelope struct {
	data                      Envelope
	segment, steps, repeats   uint8
	value, offset             int16
	newSegment, loop, stopped bool
}

func (e *audioEnvelope) reset(data Envelope, loop bool) {
	*e = audioEnvelope{data: data, newSegment: true, loop: loop}
}

// advance evaluates four envelope stages with signed deltas and sustain looping.
func (e *audioEnvelope) advance(base int) int {
	if e.newSegment {
		e.value += int16(e.data.Stages[e.segment].Delta)
		e.newSegment = false
	}
	value := max(0, base+int(e.value)-int(e.offset))
	if e.stopped {
		return value
	}
	e.steps++
	if e.steps != uint8(e.data.Stages[e.segment].Ticks) {
		return value
	}
	e.steps = 0
	e.repeats++
	if e.repeats != uint8(e.data.Stages[e.segment].Repeats) {
		// Repeated stages apply a fresh delta after each completed interval.
		e.newSegment = true
		return value
	}
	e.repeats = 0
	e.segment++
	if e.segment == 4 {
		if !e.loop {
			e.stopped = true
			e.segment = 3
			return value
		}
		e.offset += int16(e.data.LoopOffset)
		e.segment = 1
	}
	e.newSegment = true
	return value
}

type audioVoice struct {
	sequence                       []int
	seq, at                        int
	pattern                        []Event
	delay, length                  uint8
	transpose                      uint8
	period, volume                 int
	basePeriod, masterVolume       int
	sample                         int
	position                       float64
	inLoop, active                 bool
	volumeEnvelope, periodEnvelope audioEnvelope
	channel                        uint8
}

// Player is a bounded, deterministic PCM reader. Its four score voices
// retain independent note, sample and envelope state. Effects use separate
// voices so an input event cannot restart the background soundtrack.
type Player struct {
	mu         sync.Mutex
	bank       *Bank
	voices     [12]audioVoice
	sampleRate int
	clock      float64
	tempo      uint8
	music      bool
	paused     bool
	volume     float64
	pending    [4]byte
	pendingAt  int
	pendingLen int
}

// NewPlayer creates an infinite signed 16-bit little-endian stereo PCM reader.
func NewPlayer(bank *Bank, sampleRate int) (*Player, error) {
	if sampleRate < 8000 || sampleRate > 192000 {
		return nil, fmt.Errorf("unsupported audio sample rate")
	}
	if err := bank.Validate(); err != nil {
		return nil, err
	}
	r := &Player{bank: bank, sampleRate: sampleRate, volume: 1}
	r.playTheme()
	return r, nil
}

// PlayTheme restarts the score; effects already sounding remain independent.
func (r *Player) PlayTheme() { r.mu.Lock(); defer r.mu.Unlock(); r.playTheme() }
func (r *Player) playTheme() {
	r.tempo, r.music = uint8(r.bank.Tempo), true
	for i := range 4 {
		r.voices[i] = audioVoice{sequence: r.bank.Sequences[i], sample: -1, masterVolume: 63, active: true, channel: uint8(i)}
	}
}

// SetPaused silences output and freezes the complete score and sample timeline.
func (r *Player) SetPaused(paused bool) { r.mu.Lock(); defer r.mu.Unlock(); r.paused = paused }

// SetVolume sets the master gain in the inclusive 0..1 range.
func (r *Player) SetVolume(volume float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if math.IsNaN(volume) {
		volume = 0
	}
	r.volume = max(0, min(1, volume))
}

func (r *Player) PlayPattern(id int, volume uint8) bool {
	return r.playPattern(id, volume, 0)
}

func (r *Player) playPattern(id int, volume, channel uint8) bool {
	if r == nil || id < 0 || id >= len(r.bank.Patterns) {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 4; i < len(r.voices); i++ {
		if !r.voices[i].active {
			r.voices[i] = audioVoice{sequence: []int{id}, sample: -1, active: true, masterVolume: min(63, int(volume)), channel: channel & 3}
			return true
		}
	}
	return false
}

func (r *Player) TriggerCue(id int) bool {
	if r == nil || id < 0 || id >= len(r.bank.Cues) {
		return false
	}
	cue := r.bank.Cues[id]
	applied := r.playPattern(cue.Pattern, uint8(cue.Volume), uint8(cue.Channel))
	if cue.Companion > 0 && cue.Companion != id {
		companion := r.bank.Cues[cue.Companion]
		r.playPattern(companion.Pattern, uint8(companion.Volume), uint8(companion.Channel))
	}
	return applied
}

func (r *Player) SetMusic(enabled bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.music = enabled
}

func (r *Player) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.paused {
		clear(p)
		return len(p), nil
	}
	written := 0
	if r.pendingLen > 0 {
		n := copy(p, r.pending[r.pendingAt:r.pendingAt+r.pendingLen])
		r.pendingAt += n
		r.pendingLen -= n
		written += n
		p = p[n:]
	}
	if len(p) == 0 {
		return written, nil
	}
	aligned := len(p) / 4 * 4
	if aligned > 0 {
		r.renderPCM(p[:aligned])
		written += aligned
		p = p[aligned:]
	}
	if len(p) > 0 {
		r.renderPCM(r.pending[:])
		n := copy(p, r.pending[:])
		r.pendingAt = n
		r.pendingLen = 4 - n
		written += n
	}
	return written, nil
}

func (r *Player) renderPCM(p []byte) {
	frames := len(p) / 4
	for frame := range frames {
		r.clock += TickRate / float64(r.sampleRate)
		for r.clock >= 1 {
			r.tick()
			r.clock--
		}
		left, right := 0.0, 0.0
		for i := range r.voices {
			if i < 4 && !r.music {
				continue
			}
			v := &r.voices[i]
			if !v.active || v.sample < 0 || v.sample >= len(r.bank.Samples) || v.period <= 0 {
				continue
			}
			wave := r.bank.Samples[v.sample].Initial
			if v.inLoop {
				wave = r.bank.Samples[v.sample].Loop
			}
			if len(wave) == 0 {
				continue
			}
			for int(v.position) >= len(wave) {
				v.position -= float64(len(wave))
				v.inLoop = true
				wave = r.bank.Samples[v.sample].Loop
				if len(wave) == 0 {
					break
				}
			}
			if len(wave) == 0 {
				continue
			}
			value := float64(int8(wave[int(v.position)])) * float64(min(64, v.volume)) * 2.0
			v.position += 3546895.0 / float64(v.period) / float64(r.sampleRate)
			if v.channel == 0 || v.channel == 3 {
				left += value
				right += value * .25
			} else {
				right += value
				left += value * .25
			}
		}
		binary.LittleEndian.PutUint16(p[frame*4:], uint16(int16(max(-32768, min(32767, int(math.Round(left*r.volume)))))))
		binary.LittleEndian.PutUint16(p[frame*4+2:], uint16(int16(max(-32768, min(32767, int(math.Round(right*r.volume)))))))
	}
}

func (r *Player) tick() {
	if r.tempo > 0 {
		r.tempo--
	}
	advance := r.tempo == 0
	for i := range r.voices {
		v := &r.voices[i]
		if !v.active {
			continue
		}
		if advance && v.delay > 0 {
			v.delay--
		}
		if v.delay == 0 && !r.nextNote(v, i < 4) {
			continue
		}
		v.volume = max(0, min(64, v.volumeEnvelope.advance(0)-(63-v.masterVolume)))
		v.period = v.periodEnvelope.advance(v.basePeriod)
	}
	if advance {
		r.tempo = uint8(r.bank.Tempo)
	}
}

func (r *Player) nextNote(v *audioVoice, loop bool) bool {
	for commands := 0; commands < 1024; commands++ {
		if v.pattern == nil || v.at >= len(v.pattern) {
			if v.seq >= len(v.sequence) {
				if !loop || len(v.sequence) == 0 {
					v.active = false
					return false
				}
				v.seq = 0
			}
			v.pattern = r.bank.Patterns[v.sequence[v.seq]]
			v.seq++
			v.at = 0
		}
		event := v.pattern[v.at]
		v.at++
		switch event.Kind {
		case "end":
			v.pattern = nil
		case "note":
			note := uint8(event.Value)
			if note != 0 {
				note += v.transpose
			}
			if int(note) >= len(r.bank.Periods) {
				v.active = false
				return false
			}
			v.basePeriod = r.bank.Periods[note]
			v.delay = v.length
			v.volumeEnvelope.restart()
			v.periodEnvelope.restart()
			return true
		case "duration":
			v.length = uint8(r.bank.Durations[event.Value])
			v.delay = v.length
		case "sample":
			v.sample = event.Value
			v.position = 0
			v.inLoop = false
		case "volume-envelope":
			envelope := r.bank.VolumeEnvelopes[event.Value]
			v.volumeEnvelope.reset(envelope, envelope.Loop)
		case "transpose":
			v.transpose = uint8(event.Value)
		case "period-envelope", "period-envelope-loop":
			v.periodEnvelope.reset(r.bank.PeriodEnvelopes[event.Value], event.Kind == "period-envelope-loop")
		case "hold":
		default:
			v.active = false
			return false
		}
	}
	v.active = false
	return false
}

func (e *audioEnvelope) restart() {
	e.value, e.offset, e.segment, e.steps, e.repeats = 0, 0, 0, 0, 0
	e.stopped, e.newSegment = false, true
}
