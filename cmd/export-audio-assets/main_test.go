package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"go-populous2/internal/music"
	"go-populous2/internal/populous2"
)

func TestExportedSemanticScoreMatchesPreviousReplay(t *testing.T) {
	bundle, err := populous2.Load()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := export(directory); err != nil {
		t.Fatal(err)
	}
	bank, err := music.LoadFS(os.DirFS(directory), "score.json")
	if err != nil {
		t.Fatal(err)
	}
	current, err := music.NewPlayer(bank, 44100)
	if err != nil {
		t.Fatal(err)
	}
	previous := populous2.NewAudioReplay(bundle.Audio, 44100)
	for tick := 0; tick < 1000; tick++ {
		if tick == 100 || tick == 300 {
			cue := 78 + tick/300
			if !current.TriggerCue(cue) || !previous.PlayCue(cue) {
				t.Fatalf("cue %d was not admitted", cue)
			}
		}
		left, right := make([]byte, 44100/50*4), make([]byte, 44100/50*4)
		if _, err := io.ReadFull(current, left); err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadFull(previous, right); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(left, right) {
			t.Fatalf("semantic musical events changed PCM at tick %d", tick)
		}
	}
}
