package app

import (
	"go-populous2/internal/engine"
	"os"
	"path/filepath"
	"testing"
)

func TestSavedSessionRestoresProfileAndDetachedWorld(t *testing.T) {
	w := &engine.World{}
	for stage := 1; stage < engine.TownStages; stage++ {
		w.Landscape.WorkTicks[stage] = 1
		w.Landscape.EmigrationDivisor[stage] = 2
	}
	g := &Game{Assets: &Assets{Levels: make([]engine.Level, 1000)}, World: w, Profile: engine.NewDeity("PLAYER"), SavePath: filepath.Join(t.TempDir(), "game.json"), CameraX: 12, CameraY: 8, LevelIndex: 7, Selected: engine.Basalt, Direction: 2, LocalSide: 1}
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	g.Profile.Name = "CHANGED"
	g.CameraX = 0
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.Profile.Name != "PLAYER" || g.CameraX != 12 || g.LevelIndex != 7 || g.Selected != engine.Basalt || g.World == w || g.playerSide() != 1 {
		t.Fatal("saved session lost presentation or detached state")
	}
	before := g.World
	if err := os.WriteFile(g.SavePath, []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := g.loadGame(); err == nil || g.World != before || g.Profile.Name != "PLAYER" {
		t.Fatal("invalid session changed live game", err)
	}
}
