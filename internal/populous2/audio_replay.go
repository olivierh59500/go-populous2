package populous2

import (
	"encoding/binary"
	"math"
	"sync"
)

// AudioTickRate follows the PAL CIA clock and the driver's $19xx timer reload.
// The low timer byte is not written by the executable; zero is the current
// replay setting. Hardware timing still needs an original-machine trace.
const AudioTickRate = 709379.0 / 0x1900
const AudioSampleRate = 44100

type audioEnvelope struct {
	data                      [13]byte
	segment, steps, repeats   uint8
	value, offset             int16
	newSegment, loop, stopped bool
}

func (e *audioEnvelope) reset(data [13]byte, loop bool) {
	*e = audioEnvelope{data: data, newSegment: true, loop: loop}
}

// advance translates the four three-byte envelope stages at CODE:$1956c.
func (e *audioEnvelope) advance(base int) int {
	if e.newSegment {
		e.value += int16(int8(e.data[2+int(e.segment)*3]))
		e.newSegment = false
	}
	value := max(0, base+int(e.value)-int(e.offset))
	if e.stopped {
		return value
	}
	e.steps++
	if e.steps != e.data[3+int(e.segment)*3] {
		return value
	}
	e.steps = 0
	e.repeats++
	if e.repeats != e.data[1+int(e.segment)*3] {
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
		e.offset += int16(e.data[0] & 127)
		e.segment = 1
	}
	e.newSegment = true
	return value
}

type audioVoice struct {
	sequence                       []uint8
	seq, at                        int
	pattern                        []byte
	delay, length                  uint8
	transpose                      uint8
	period, volume                 int
	basePeriod, masterVolume       int
	sample                         int
	position                       float64
	inLoop, active                 bool
	volumeEnvelope, periodEnvelope audioEnvelope
}

// AudioReplay is a bounded, deterministic PCM reader. Its four score voices
// retain independent note, sample and envelope state. Effects use separate
// voices so an input event cannot restart the background soundtrack.
type AudioReplay struct {
	mu         sync.Mutex
	bank       *AudioBank
	voices     [12]audioVoice
	sampleRate int
	clock      float64
	tempo      uint8
	Music      bool
}

func NewAudioReplay(bank *AudioBank, sampleRate int) *AudioReplay {
	if bank == nil || sampleRate < 8000 || sampleRate > 192000 {
		return nil
	}
	r := &AudioReplay{bank: bank, sampleRate: sampleRate, tempo: bank.Tempo, Music: true}
	for i := range 4 {
		r.voices[i] = audioVoice{sequence: bank.Channels[i], sample: -1, masterVolume: 63, active: len(bank.Channels[i]) > 0}
	}
	return r
}

func (r *AudioReplay) PlayPattern(id int, volume uint8) bool {
	if r == nil || id < 0 || id >= len(r.bank.Patterns) {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 4; i < len(r.voices); i++ {
		if !r.voices[i].active {
			r.voices[i] = audioVoice{sequence: []uint8{uint8(id)}, sample: -1, active: true, masterVolume: min(63, int(volume))}
			return true
		}
	}
	return false
}

func (r *AudioReplay) PlayCue(id int) bool {
	if r == nil || id < 0 || id >= len(r.bank.Cues) {
		return false
	}
	cue := r.bank.Cues[id]
	applied := r.PlayPattern(cue.Pattern, cue.Volume)
	if cue.Companion > 0 && cue.Companion != id {
		companion := r.bank.Cues[cue.Companion]
		r.PlayPattern(companion.Pattern, companion.Volume)
	}
	return applied
}

func (r *AudioReplay) SetMusic(enabled bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Music = enabled
}

func (r *AudioReplay) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	frames := len(p) / 4
	for frame := range frames {
		r.clock += AudioTickRate / float64(r.sampleRate)
		for r.clock >= 1 {
			r.tick()
			r.clock--
		}
		left, right := 0.0, 0.0
		for i := range r.voices {
			if i < 4 && !r.Music {
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
			if i%4 == 0 || i%4 == 3 {
				left += value
				right += value * .25
			} else {
				right += value
				left += value * .25
			}
		}
		binary.LittleEndian.PutUint16(p[frame*4:], uint16(int16(max(-32768, min(32767, int(math.Round(left)))))))
		binary.LittleEndian.PutUint16(p[frame*4+2:], uint16(int16(max(-32768, min(32767, int(math.Round(right)))))))
	}
	return frames * 4, nil
}

func (r *AudioReplay) tick() {
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
		v.volume = clamp(v.volumeEnvelope.advance(0)-(63-v.masterVolume), 0, 64)
		v.period = v.periodEnvelope.advance(v.basePeriod)
	}
	if advance {
		r.tempo = r.bank.Tempo
	}
}

func (r *AudioReplay) nextNote(v *audioVoice, loop bool) bool {
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
		command := v.pattern[v.at]
		v.at++
		switch {
		case command == 255:
			v.pattern = nil
		case command < 128:
			note := command
			if note != 0 {
				note += v.transpose
			}
			if int(note) >= len(r.bank.Periods) {
				v.active = false
				return false
			}
			v.basePeriod = int(r.bank.Periods[note])
			v.delay = v.length
			v.volumeEnvelope.value = 0
			v.volumeEnvelope.offset = 0
			v.volumeEnvelope.segment = 0
			v.volumeEnvelope.steps = 0
			v.volumeEnvelope.repeats = 0
			v.volumeEnvelope.stopped = false
			v.volumeEnvelope.newSegment = true
			v.periodEnvelope.value = 0
			v.periodEnvelope.offset = 0
			v.periodEnvelope.segment = 0
			v.periodEnvelope.steps = 0
			v.periodEnvelope.repeats = 0
			v.periodEnvelope.stopped = false
			v.periodEnvelope.newSegment = true
			return true
		case command < 160:
			v.length = r.bank.Lengths[command&15]
			v.delay = v.length
		case command < 192:
			v.sample = int(command & 31)
			v.position = 0
			v.inLoop = false
		case command < 224:
			env := r.bank.VolumeEnvelopes[command&31]
			v.volumeEnvelope.reset(env, env[0]&0x80 != 0)
		case command == 224:
			if v.at >= len(v.pattern) {
				v.active = false
				return false
			}
			v.transpose = v.pattern[v.at]
			v.at++
		case command == 225 || command == 226:
			if v.at >= len(v.pattern) || v.pattern[v.at] >= 32 {
				v.active = false
				return false
			}
			v.periodEnvelope.reset(r.bank.PeriodEnvelopes[v.pattern[v.at]], command == 225)
			v.at++
		case command == 227:
		default:
			v.active = false
			return false
		}
	}
	v.active = false
	return false
}
