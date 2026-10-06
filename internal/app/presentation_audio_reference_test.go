package app

import (
	"image"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/music"
)

func TestPrivatePresentedCombatCueSurvivesRepeatedFrameDraws(t *testing.T) {
	path := os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if path == "" {
		t.Skip("set portable artwork/audio directory")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := music.NewPlayer(assets.Music, 44100)
	if err != nil {
		t.Fatal(err)
	}
	w := &engine.World{Editor: true}
	var heights [engine.CornerSize * engine.CornerSize]uint8
	for i := range heights {
		heights[i] = 1
	}
	if err := w.EditorSetTerrain(heights); err != nil {
		t.Fatal(err)
	}
	if err := w.EditorPlaceFollower(0, 32, 32, 1000); err != nil {
		t.Fatal(err)
	}
	w.Followers[1].State, w.Followers[1].BattleAggressor = engine.Fighting, true
	g := &Game{World: w, Assets: assets, Screen: Playing, CameraX: 28, CameraY: 28, music: replay, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.presentation.Capture(g, true)
	cue := assets.Visual.Animations["combat/attack"].Frames[0].SoundCue
	if cue <= 0 {
		t.Fatal("source combat artwork has no actual cue")
	}
	for repeat := 0; repeat < 4; repeat++ {
		g.drawFrame()
		if !g.AnimationSounds.Started || !g.AnimationSounds.Played[cue] || g.AnimationSounds.Tick != 1 {
			t.Fatal("presenter lost its live cue admission", repeat)
		}
		if g.AnimationSounds.Admit(1, cue) {
			t.Fatal("repeated displayed frame could restart the same source combat sound", repeat)
		}
	}
}
