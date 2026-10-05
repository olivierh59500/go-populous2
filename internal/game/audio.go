package game

import (
	"github.com/hajimehoshi/ebiten/v2/audio"
	"go-populous2/internal/populous2"
)

func (g *Game) initializeAudio() error {
	context := audio.CurrentContext()
	if context == nil {
		context = audio.NewContext(populous2.AudioSampleRate)
	}
	g.audioReplay = populous2.NewAudioReplay(g.Bundle.Audio, context.SampleRate())
	g.audioReplay.SetMusic(!g.specialMusicDisabled)
	player, err := context.NewPlayer(g.audioReplay)
	if err != nil {
		return err
	}
	g.audioPlayer = player
	player.SetVolume(.35)
	player.Play()
	return nil
}

func (g *Game) playPowerSound(id populous2.SpellID, player int) {
	if g.audioReplay == nil {
		return
	}
	if id.IsHero() {
		return // The native conversion emits its cue through World events.
	}
	for _, cue := range g.Bundle.CastSoundCues(id, player) {
		g.audioReplay.PlayCue(cue)
	}
}
