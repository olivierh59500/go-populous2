package app

import (
	"encoding/json"
	"go-populous2/internal/engine"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacySessionsRetainTheirFireSpellWithoutChangingNativePermissionSlots(t *testing.T) {
	for _, tc := range []struct {
		legacy engine.PowerID
		power  engine.PowerID
	}{{26, engine.Volcano}, {27, engine.Achilles}} {
		g := browserGame(t)
		g.SavePath = filepath.Join(t.TempDir(), "legacy.json")
		permissions := g.World.Level.Players[0].Powers
		world := g.World.Snapshot()
		world.Version = 2
		session := savedSession{Version: 1, World: world, Profile: g.Profile, Selected: tc.legacy}
		data, err := json.Marshal(session)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(g.SavePath, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := g.loadGame(); err != nil {
			t.Fatal(err)
		}
		if g.Selected != tc.power || g.Category != engine.Fire || g.World.Level.Players[0].Powers != permissions {
			t.Fatal("legacy session lost its spell meaning or moved original permission flags")
		}
		if err := g.saveGame(); err != nil {
			t.Fatal(err)
		}
		if err := g.loadGame(); err != nil || g.Selected != tc.power {
			t.Fatal("canonical session migrated its selection twice", err)
		}
	}
}

func TestSavedSessionRestoresProfileAndDetachedWorld(t *testing.T) {
	w := &engine.World{}
	for stage := 1; stage < engine.TownStages; stage++ {
		w.Landscape.WorkTicks[stage] = 1
		w.Landscape.EmigrationDivisor[stage] = 2
	}
	g := &Game{Assets: &Assets{Levels: make([]engine.Level, 1000)}, World: w, Profile: engine.NewDeity("PLAYER"), SavePath: filepath.Join(t.TempDir(), "game.json"), CameraX: 12, CameraY: 8, LevelIndex: 7, Selected: engine.Basalt, Category: engine.Fire, Direction: 2, LocalSide: 1}
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	g.Profile.Name = "CHANGED"
	g.CameraX = 0
	g.Category = engine.People
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.Profile.Name != "PLAYER" || g.CameraX != 12 || g.LevelIndex != 7 || g.Selected != engine.Basalt || g.Category != engine.Fire || g.World == w || g.playerSide() != 1 {
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

func TestLoadedCompletedGameReturnsToResultsWithoutAwarding(t *testing.T) {
	g := browserGame(t)
	g.World.Result = 1
	g.World.Players[1].Statistics.ScenarioOptions = 1
	profile := g.Profile
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.Screen != CampaignResult || g.resultApplied || g.Profile != profile {
		t.Fatal("loading a completed game resumed play or granted rewards")
	}
}

func TestLoadedCustomSessionRestoresSetupForTheNextGame(t *testing.T) {
	g := browserGame(t)
	g.CustomGame = true
	g.World.Level.Players[0].Population = 777
	g.World.Players[1].Computer = false
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	old := g.Assets.Levels[0]
	g.CustomLevel = &old
	g.CustomComputer = [2]bool{false, true}
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.CustomLevel == nil || g.CustomLevel.Players[0].Population != 777 || g.CustomComputer[1] {
		t.Fatal("loaded custom game retained stale menu setup")
	}
}

func TestPlayerAssistSelectionSurvivesSessionSave(t *testing.T) {
	g := browserGame(t)
	g.controlMode = 18
	g.World.Players[0].Assisted, g.World.Players[1].Computer = true, true
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	g.controlMode = 4
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.controlMode != 18 || !g.World.Players[0].Assisted || g.World.Players[0].Computer || !g.World.Players[1].Computer {
		t.Fatal("saved assist state was confused with computer-versus-computer mode")
	}
}
