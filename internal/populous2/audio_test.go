package populous2

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestOriginalAudioBank(t *testing.T) {
	b := testBundle(t).Audio
	if len(b.Samples) != 31 || len(b.Patterns) != 133 || b.Tempo != 3 {
		t.Fatal("native audio catalog incomplete")
	}
	if len(b.Samples[0].Initial) != 52 || len(b.Samples[0].Loop) != 7960 || len(b.Samples[30].Loop) != 2644 {
		t.Fatal("Paula word lengths or bank-relative addresses decoded incorrectly")
	}
	for channel, sequence := range b.Channels {
		if len(sequence) == 0 {
			t.Fatalf("original score channel %d missing", channel)
		}
	}
	if b.Periods[69] != 508 || b.Lengths[0] != 2 || b.Lengths[11] != 96 {
		t.Fatal("native pitch/duration lookup changed")
	}
	fx := append([]byte(nil), testBundle(t).Raw["fx.dat"]...)
	fx[4], fx[5], fx[6], fx[7] = 255, 255, 255, 255
	if _, err := DecodeAudioBank(testBundle(t).Executable, fx); err == nil {
		t.Fatal("out-of-bounds sample table accepted")
	}
}

func TestNativeScoreReplayIsIndependentOfReadSize(t *testing.T) {
	bank := testBundle(t).Audio
	a, b := NewAudioReplay(bank, AudioSampleRate), NewAudioReplay(bank, AudioSampleRate)
	whole := make([]byte, AudioSampleRate*4*2)
	if n, err := a.Read(whole); err != nil || n != len(whole) {
		t.Fatal("PCM replay failed")
	}
	var chunks []byte
	for len(chunks) < len(whole) {
		p := make([]byte, min(172, len(whole)-len(chunks)))
		if n, err := b.Read(p); err != nil || n != len(p) {
			t.Fatal("chunked PCM replay failed")
		}
		chunks = append(chunks, p...)
	}
	if !bytes.Equal(whole, chunks) {
		t.Fatal("reader size changed replay timing")
	}
	if bytes.Equal(whole, make([]byte, len(whole))) {
		t.Fatal("original score produced silence")
	}
	for id := range bank.Patterns {
		r := NewAudioReplay(bank, AudioSampleRate)
		r.SetMusic(false)
		if !r.PlayPattern(id, 63) {
			t.Fatalf("original effect %d rejected", id)
		}
		for range 12 {
			r.Read(make([]byte, 4096))
		}
	}
}

func TestSignedPCMAndInitialLoopTransition(t *testing.T) {
	bank := &AudioBank{Tempo: 3, Samples: []AudioSample{{Initial: []byte{64, 128}, Loop: []byte{32, 224}}}}
	r := NewAudioReplay(bank, AudioSampleRate)
	r.voices[4] = audioVoice{sample: 0, active: true, period: 80, volume: 64}
	pcm := make([]byte, 16)
	r.Read(pcm)
	if got := int16(binary.LittleEndian.Uint16(pcm)); got != 8192 {
		t.Fatalf("positive sample %d", got)
	}
	if got := int16(binary.LittleEndian.Uint16(pcm[4:])); got != -16384 {
		t.Fatalf("signed sample %d", got)
	}
	if got := int16(binary.LittleEndian.Uint16(pcm[8:])); got != 4096 {
		t.Fatalf("loop sample %d", got)
	}
}
