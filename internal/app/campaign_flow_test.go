package app

import (
	"os"
	"testing"

	"go-populous2/internal/engine"
)

func TestPrivateCompletedFinalWorldRoutesThroughAwardAndOriginalEnding(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("provide private portable ending artwork")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	world, err := engine.NewWorld(assets.Levels[999], assets.Landscapes[assets.Levels[999].Landscape])
	if err != nil {
		t.Fatal(err)
	}
	world.Result, world.Tick = 1, 5000
	world.Players[1].Statistics.ScenarioOptions = 1
	g := &Game{Assets: assets, World: world, Profile: engine.NewDeity("PLAYER"), LevelIndex: 999, Screen: Playing}
	g.finishWorld()
	if g.Screen != CampaignResult {
		t.Fatal("finished world did not open its result")
	}
	if err := g.applyCampaignResult(); err != nil {
		t.Fatal(err)
	}
	if g.Screen != EndingScreen || g.Ending == nil || !g.ResultProgress.Complete || g.LevelIndex != 0 {
		t.Fatal("final campaign did not reach the original ending")
	}
	profile := g.Profile
	if err := g.applyCampaignResult(); err != nil || g.Profile != profile {
		t.Fatal("final result awarded twice", err)
	}
	for range 150 {
		g.Ending.Update()
	}
	if g.Ending.Frame < g.Ending.Sequence.LoopStart || g.World.Tick != 5000 {
		t.Fatal("ending clock did not loop independently of the completed world")
	}
}
