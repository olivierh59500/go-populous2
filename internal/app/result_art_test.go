package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestResultPresentationUsesLocalCampAndLeavesScoreProgressionUntouched(t *testing.T) {
	world := controllerWorld(t)
	world.Result = 2
	world.Tick = 1200
	world.Players[0].Statistics = engine.CampaignStatistics{PeakPopulation: 20, PeakMana: 100, BattleWins: 3, LeaderLosses: 4}
	world.Players[1].Statistics = engine.CampaignStatistics{PeakPopulation: 50, PeakMana: 900, BattleWins: 7, LeaderLosses: 1}
	g := &Game{World: world, LocalSide: 1, ResultScore: engine.CampaignScore{Value: 65535}}
	before := world.Snapshot()
	state := g.resultPresentationState()
	if state.Winner != 1 || state.Local.PeakPopulation != 50 || state.Opponent.PeakPopulation != 20 || state.Score != 65535 || state.Ticks != 1200 {
		t.Fatal("original result fields selected the wrong camp", state)
	}
	if world.Snapshot() != before {
		t.Fatal("result presentation changed simulation or score inputs")
	}
	g.Assets = &Assets{Result: &visualassets.ResultDescriptor{Layout: visualassets.RequesterLayout{Actions: []visualassets.RequesterAction{{Name: "continue", X: 100, Y: 120, Width: 64, Height: 8}}}}}
	if !g.resultContinueHit(110, 124) || g.resultContinueHit(12, 12) {
		t.Fatal("result continuation ignored original OK hit region")
	}
}

func TestPrivateResultApplicationFrameWithPortableArtwork(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("set portable original artwork directory")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	world, err := engine.NewWorld(assets.Levels[0], assets.Landscapes[assets.Levels[0].Landscape])
	if err != nil {
		t.Fatal(err)
	}
	world.Result = 1
	// The presentation fixture supplies the scoring precondition explicitly.
	world.Players[1].Statistics.ScenarioOptions = 1
	g := &Game{Assets: assets, World: world, CameraX: 28, CameraY: 28, Profile: engine.NewDeity("PLAYER"), framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.finishWorld()
	g.drawCampaignResult()
	if g.Screen != CampaignResult || g.ResultScoreError != "" {
		t.Fatal("original result presentation lacks its computed score", g.ResultScoreError)
	}
	if capture := os.Getenv("POPULOUS2_RESULT_CAPTURE"); capture != "" {
		file, err := os.Create(capture)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, g.framebuffer)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
	}
}
