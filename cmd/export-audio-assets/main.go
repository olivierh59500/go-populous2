// Command export-audio-assets converts the original score into portable,
// semantic musical events and separate signed eight-bit PCM blocks.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go-populous2/internal/music"
	"go-populous2/internal/populous2"
)

func main() {
	output := flag.String("output", ".local/audio-assets", "destination for the score manifest and PCM blocks")
	flag.Parse()
	if err := export(*output); err != nil {
		log.Fatal(err)
	}
}

func export(output string) error {
	bundle, err := populous2.Load()
	if err != nil {
		return err
	}
	bank, err := convert(bundle.Audio)
	if err != nil {
		return err
	}
	if err := bank.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	for i, sample := range bank.Samples {
		asset := bank.SampleAssets[i]
		for _, part := range []struct {
			name string
			pcm  []byte
		}{{asset.InitialFile, sample.Initial}, {asset.LoopFile, sample.Loop}} {
			if err := os.WriteFile(filepath.Join(output, part.name), part.pcm, 0644); err != nil {
				return err
			}
		}
	}
	manifest, err := json.MarshalIndent(bank, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "score.json"), append(manifest, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("Exported %d patterns, %d cues, %d PCM samples to %s\n", len(bank.Patterns), len(bank.Cues), len(bank.Samples), output)
	return nil
}

func convert(source *populous2.AudioBank) (*music.Bank, error) {
	if source == nil {
		return nil, fmt.Errorf("original score is missing")
	}
	bank := &music.Bank{Version: music.AssetVersion, Tempo: int(source.Tempo)}
	for _, cue := range source.Cues {
		bank.Cues = append(bank.Cues, music.Cue{Priority: cue.Flags, Channel: int(cue.Channel), Volume: int(cue.Volume), Pattern: cue.Pattern, Companion: cue.Companion})
	}
	for i, sample := range source.Samples {
		bank.Samples = append(bank.Samples, music.Sample{Initial: append([]byte(nil), sample.Initial...), Loop: append([]byte(nil), sample.Loop...)})
		bank.SampleAssets = append(bank.SampleAssets, music.SampleAsset{InitialFile: fmt.Sprintf("sample-%02d-attack.pcm", i), LoopFile: fmt.Sprintf("sample-%02d-sustain.pcm", i)})
	}
	for _, period := range source.Periods {
		bank.Periods = append(bank.Periods, int(period))
	}
	for _, duration := range source.Lengths {
		bank.Durations = append(bank.Durations, int(duration))
	}
	for _, envelope := range source.VolumeEnvelopes {
		bank.VolumeEnvelopes = append(bank.VolumeEnvelopes, convertEnvelope(envelope))
	}
	for _, envelope := range source.PeriodEnvelopes {
		bank.PeriodEnvelopes = append(bank.PeriodEnvelopes, convertEnvelope(envelope))
	}
	for channel, sequence := range source.Channels {
		for _, pattern := range sequence {
			bank.Sequences[channel] = append(bank.Sequences[channel], int(pattern))
		}
	}
	for i, pattern := range source.Patterns {
		events, err := convertPattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("pattern %d: %w", i, err)
		}
		bank.Patterns = append(bank.Patterns, events)
	}
	return bank, nil
}

func convertEnvelope(data [13]byte) music.Envelope {
	envelope := music.Envelope{Loop: data[0]&128 != 0, LoopOffset: int(data[0] & 127)}
	for i := range envelope.Stages {
		envelope.Stages[i] = music.EnvelopeStage{Repeats: int(data[1+i*3]), Delta: int(int8(data[2+i*3])), Ticks: int(data[3+i*3])}
	}
	return envelope
}

func convertPattern(data []byte) ([]music.Event, error) {
	var events []music.Event
	for at := 0; at < len(data); at++ {
		command := data[at]
		event := music.Event{}
		switch {
		case command == 255:
			event.Kind = "end"
		case command < 128:
			event.Kind, event.Value = "note", int(command)
		case command < 160:
			event.Kind, event.Value = "duration", int(command&15)
		case command < 192:
			event.Kind, event.Value = "sample", int(command&31)
		case command < 224:
			event.Kind, event.Value = "volume-envelope", int(command&31)
		case command == 224 || command == 225 || command == 226:
			at++
			if at >= len(data) {
				return nil, fmt.Errorf("musical event has no argument")
			}
			event.Kind = map[byte]string{224: "transpose", 225: "period-envelope-loop", 226: "period-envelope"}[command]
			event.Value = int(data[at])
		case command == 227:
			event.Kind = "hold"
		default:
			return nil, fmt.Errorf("unknown score event %d", command)
		}
		events = append(events, event)
	}
	return events, nil
}
