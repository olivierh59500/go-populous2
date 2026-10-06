package music

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"testing"
	"testing/fstest"
)

func testBank() *Bank {
	b := &Bank{Version: AssetVersion, Tempo: 3, Samples: []Sample{{Initial: []byte{64, 128}, Loop: []byte{32, 224}}}, Periods: []int{0, 80}, Durations: make([]int, 16), VolumeEnvelopes: make([]Envelope, 32), PeriodEnvelopes: make([]Envelope, 32)}
	b.Durations[0] = 4
	b.VolumeEnvelopes[0] = Envelope{Stages: [4]EnvelopeStage{{Repeats: 1, Delta: 63, Ticks: 255}, {Repeats: 1, Ticks: 255}, {Repeats: 1, Ticks: 255}, {Repeats: 1, Ticks: 255}}}
	b.Patterns = [][]Event{{{Kind: "sample"}, {Kind: "duration"}, {Kind: "volume-envelope"}, {Kind: "note", Value: 1}, {Kind: "end"}}}
	for i := range b.Sequences {
		b.Sequences[i] = []int{0}
	}
	b.Cues = []Cue{{Channel: 3, Volume: 63}}
	return b
}

func testPlayer(t *testing.T, b *Bank) *Player {
	t.Helper()
	p, err := NewPlayer(b, DefaultSampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlayerReadBoundariesDoNotChangeScoreTiming(t *testing.T) {
	a, b := testPlayer(t, testBank()), testPlayer(t, testBank())
	whole, parts := make([]byte, 44100*4*2+3), make([]byte, 44100*4*2+3)
	if _, err := io.ReadFull(a, whole); err != nil {
		t.Fatal(err)
	}
	for at := 0; at < len(parts); {
		count := min(1+at%173, len(parts)-at)
		if _, err := io.ReadFull(b, parts[at:at+count]); err != nil {
			t.Fatal(err)
		}
		at += count
	}
	if !bytes.Equal(whole, parts) || bytes.Equal(whole, make([]byte, len(whole))) {
		t.Fatal("read size changed the replay or produced silence")
	}
}

func TestPlayerPausePreservesPartialFrameAndTimeline(t *testing.T) {
	a, b := testPlayer(t, testBank()), testPlayer(t, testBank())
	intro := make([]byte, 999)
	a.Read(intro)
	b.Read(intro)
	b.SetPaused(true)
	silence := bytes.Repeat([]byte{99}, 10001)
	b.Read(silence)
	if !bytes.Equal(silence, make([]byte, len(silence))) {
		t.Fatal("paused playback was not silent")
	}
	b.SetPaused(false)
	left, right := make([]byte, 10001), make([]byte, 10001)
	a.Read(left)
	b.Read(right)
	if !bytes.Equal(left, right) {
		t.Fatal("pause advanced the score or discarded part of a stereo frame")
	}
}

func TestPlayerSignedAttackAndSustainBlocks(t *testing.T) {
	p := testPlayer(t, testBank())
	p.SetMusic(false)
	p.voices[4] = audioVoice{sample: 0, active: true, period: 80, volume: 64}
	pcm := make([]byte, 16)
	p.Read(pcm)
	for frame, want := range []int16{8192, -16384, 4096, -4096} {
		if got := int16(binary.LittleEndian.Uint16(pcm[frame*4:])); got != want {
			t.Errorf("PCM frame %d: %d, want %d", frame, got, want)
		}
	}
}

func TestPlayerThemeRestartDoesNotRestartEffects(t *testing.T) {
	p := testPlayer(t, testBank())
	if !p.TriggerCue(0) || p.TriggerCue(-1) || p.TriggerCue(1) {
		t.Fatal("cue admission is invalid")
	}
	p.Read(make([]byte, 2000))
	position := p.voices[4].position
	p.PlayTheme()
	if p.voices[4].position != position {
		t.Fatal("restarting the theme restarted a sound effect")
	}
	p.SetVolume(0)
	pcm := make([]byte, 2000)
	p.Read(pcm)
	if !bytes.Equal(pcm, make([]byte, len(pcm))) {
		t.Fatal("muted master gain produced sound")
	}
}

func TestLoadFSValidatesScoreAndPCMPaths(t *testing.T) {
	b := testBank()
	b.SampleAssets = []SampleAsset{{InitialFile: "attack.pcm", LoopFile: "sustain.pcm"}}
	encode := func() []byte {
		data, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	files := fstest.MapFS{"music/score.json": {Data: encode()}, "music/attack.pcm": {Data: b.Samples[0].Initial}, "music/sustain.pcm": {Data: b.Samples[0].Loop}}
	if _, err := LoadFS(files, "music/score.json"); err != nil {
		t.Fatal(err)
	}
	b.SampleAssets[0].InitialFile = "../attack.pcm"
	files["music/score.json"].Data = encode()
	if _, err := LoadFS(files, "music/score.json"); err == nil {
		t.Fatal("parent directory traversal was accepted")
	}
	b.SampleAssets[0].InitialFile = "attack.pcm"
	b.Patterns[0][0].Value = 32
	files["music/score.json"].Data = encode()
	if _, err := LoadFS(files, "music/score.json"); err == nil {
		t.Fatal("absent sample reference was accepted")
	}
}
