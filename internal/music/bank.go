// Package music replays the game's musical score and sound effects using
// semantic score data and signed PCM samples, independently of game logic.
package music

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
)

const AssetVersion = 1
const DefaultSampleRate = 44100

// TickRate is the score sequencer rate, independent of graphics and simulation.
const TickRate = 709379.0 / 0x1900

type Event struct {
	Kind  string `json:"kind"`
	Value int    `json:"value,omitempty"`
}

type EnvelopeStage struct {
	Repeats int `json:"repeats"`
	Delta   int `json:"delta"`
	Ticks   int `json:"ticks"`
}

type Envelope struct {
	Loop       bool             `json:"loop,omitempty"`
	LoopOffset int              `json:"loopOffset,omitempty"`
	Stages     [4]EnvelopeStage `json:"stages"`
}

type Cue struct {
	Priority  uint16 `json:"priority,omitempty"`
	Channel   int    `json:"channel"`
	Volume    int    `json:"volume"`
	Pattern   int    `json:"pattern"`
	Companion int    `json:"companion,omitempty"`
}

type SampleAsset struct {
	InitialFile string `json:"initialFile"`
	LoopFile    string `json:"loopFile"`
}

// Sample contains signed eight-bit PCM. The attack block plays once before the
// sustain block repeats; the two portions may have different lengths.
type Sample struct{ Initial, Loop []byte }

type Bank struct {
	Version         int           `json:"version"`
	Tempo           int           `json:"tempo"`
	Cues            []Cue         `json:"cues"`
	Patterns        [][]Event     `json:"patterns"`
	Sequences       [4][]int      `json:"sequences"`
	VolumeEnvelopes []Envelope    `json:"volumeEnvelopes"`
	PeriodEnvelopes []Envelope    `json:"periodEnvelopes"`
	Periods         []int         `json:"periods"`
	Durations       []int         `json:"durations"`
	SampleAssets    []SampleAsset `json:"samples"`
	Samples         []Sample      `json:"-"`
}

// LoadFS reads a score manifest and its PCM blocks relative to that manifest.
// Neither executable instructions nor emulated address spaces are accepted.
func LoadFS(files fs.FS, name string) (*Bank, error) {
	if files == nil || !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid music asset location")
	}
	f, err := files.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 8<<20))
	decoder.DisallowUnknownFields()
	var bank Bank
	if err := decoder.Decode(&bank); err != nil {
		return nil, fmt.Errorf("music manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("music manifest contains trailing data")
	}
	if len(bank.SampleAssets) == 0 || len(bank.SampleAssets) > 32 {
		return nil, fmt.Errorf("invalid music sample catalog")
	}
	bank.Samples = make([]Sample, len(bank.SampleAssets))
	for i, asset := range bank.SampleAssets {
		for part, file := range []string{asset.InitialFile, asset.LoopFile} {
			if !fs.ValidPath(file) || file == "." {
				return nil, fmt.Errorf("sample %d has an invalid PCM path", i)
			}
			pcm, err := fs.ReadFile(files, path.Join(path.Dir(name), file))
			if err != nil {
				return nil, fmt.Errorf("sample %d: %w", i, err)
			}
			if len(pcm) == 0 || len(pcm) > 2<<20 {
				return nil, fmt.Errorf("sample %d PCM length is invalid", i)
			}
			if part == 0 {
				bank.Samples[i].Initial = pcm
			} else {
				bank.Samples[i].Loop = pcm
			}
		}
	}
	if err := bank.Validate(); err != nil {
		return nil, err
	}
	return &bank, nil
}

func (b *Bank) Validate() error {
	if b == nil || b.Version != AssetVersion || b.Tempo < 1 || b.Tempo > 255 || len(b.Patterns) == 0 || len(b.Patterns) > 256 || len(b.Samples) == 0 || len(b.Samples) > 32 || len(b.Periods) == 0 || len(b.Periods) > 256 || len(b.Durations) != 16 || len(b.VolumeEnvelopes) != 32 || len(b.PeriodEnvelopes) != 32 {
		return fmt.Errorf("incomplete or unsupported music asset")
	}
	for i, duration := range b.Durations {
		if duration < 0 || duration > 255 {
			return fmt.Errorf("duration %d exceeds the score range", i)
		}
	}
	for i, period := range b.Periods {
		if period < 0 || period > 65535 {
			return fmt.Errorf("period %d exceeds the score range", i)
		}
	}
	for i, sample := range b.Samples {
		if len(sample.Initial) == 0 || len(sample.Loop) == 0 || len(sample.Initial) > 2<<20 || len(sample.Loop) > 2<<20 {
			return fmt.Errorf("sample %d has invalid attack or sustain data", i)
		}
	}
	for i, sequence := range b.Sequences {
		if len(sequence) == 0 || len(sequence) > 4096 {
			return fmt.Errorf("channel %d has an invalid score sequence", i)
		}
		for _, pattern := range sequence {
			if pattern < 0 || pattern >= len(b.Patterns) {
				return fmt.Errorf("channel %d references an absent pattern", i)
			}
		}
	}
	for i, cue := range b.Cues {
		if cue.Channel < 0 || cue.Channel > 3 || cue.Volume < 0 || cue.Volume > 64 || cue.Pattern < 0 || cue.Pattern >= len(b.Patterns) || cue.Companion < 0 || cue.Companion >= len(b.Cues) {
			return fmt.Errorf("invalid sound cue %d", i)
		}
	}
	for _, envelopes := range [][]Envelope{b.VolumeEnvelopes, b.PeriodEnvelopes} {
		for i, envelope := range envelopes {
			if envelope.LoopOffset < 0 || envelope.LoopOffset > 127 {
				return fmt.Errorf("envelope %d has an invalid loop offset", i)
			}
			for _, stage := range envelope.Stages {
				if stage.Repeats < 0 || stage.Repeats > 255 || stage.Ticks < 0 || stage.Ticks > 255 || stage.Delta < -128 || stage.Delta > 127 {
					return fmt.Errorf("envelope %d has an invalid stage", i)
				}
			}
		}
	}
	for i, pattern := range b.Patterns {
		if len(pattern) == 0 || len(pattern) > 4096 || pattern[len(pattern)-1].Kind != "end" {
			return fmt.Errorf("pattern %d is not terminated", i)
		}
		for _, event := range pattern {
			limit := 0
			switch event.Kind {
			case "note":
				limit = len(b.Periods)
			case "duration":
				limit = len(b.Durations)
			case "sample":
				limit = len(b.Samples)
			case "volume-envelope", "period-envelope", "period-envelope-loop":
				limit = 32
			case "transpose":
				limit = 256
			case "hold", "end":
				limit = 1
			default:
				return fmt.Errorf("pattern %d contains an unknown musical event", i)
			}
			if event.Value < 0 || event.Value >= limit {
				return fmt.Errorf("pattern %d contains an invalid %s value", i, event.Kind)
			}
		}
	}
	return nil
}
