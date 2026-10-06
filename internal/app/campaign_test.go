package app

import (
	"go-populous2/internal/engine"
	"testing"
)

func TestCampaignResultAwardsOnceAndRoutesExperienceAllocation(t *testing.T) {
	g := &Game{World: &engine.World{Result: 1}, Profile: engine.NewDeity("PLAYER"), ResultScore: engine.CampaignScore{Value: 26014}, LevelIndex: 12}
	if err := g.applyCampaignResult(); err != nil {
		t.Fatal(err)
	}
	if g.Profile.Bolts != 7 || g.LevelIndex != 17 || g.Screen != DeityProfile || !g.resultApplied {
		t.Fatal("winning result did not award and advance", g.Profile, g.LevelIndex, g.Screen)
	}
	before := g.Profile
	if err := g.applyCampaignResult(); err != nil || g.Profile != before || g.LevelIndex != 17 {
		t.Fatal("result was applied twice", err)
	}
	g.applyOriginalProfileAction("experience-2")
	allocated := g.Profile
	g.applyOriginalProfileAction("proceed")
	if g.Screen != ConquestBriefing || g.LevelIndex != 17 || g.Profile != allocated {
		t.Fatal("experience allocation did not continue directly to the next conquest")
	}
}

func TestCampaignVictoryWithNoRemainingBoltsSkipsEmptyProfile(t *testing.T) {
	g := &Game{World: &engine.World{Result: 1}, Profile: engine.Deity{Name: "PLAYER"}, LevelIndex: 12}
	if err := g.applyCampaignResult(); err != nil {
		t.Fatal(err)
	}
	if g.Screen != ConquestBriefing || g.Profile.Bolts != 0 || g.LevelIndex != 13 {
		t.Fatal("empty experience allocation interrupted conquest", g.Screen, g.LevelIndex)
	}
}

func TestFinalCampaignResultReturnsToMainMenu(t *testing.T) {
	g := &Game{World: &engine.World{Result: 1}, Profile: engine.NewDeity("PLAYER"), ResultScore: engine.CampaignScore{Value: 65035}, LevelIndex: 999}
	if err := g.applyCampaignResult(); err == nil {
		t.Fatal("final campaign accepted missing ending artwork")
	}
	if !g.ResultProgress.Complete || g.LevelIndex != 0 {
		t.Fatal("final campaign transition differs", g.ResultProgress, g.Screen)
	}
}

func TestNetworkVictoryUsesLocalSideAndLeavesConquestUntouched(t *testing.T) {
	host, join, _, world := controllerPair(t)
	_ = host
	world.Result, world.Tick = 2, 100
	world.Players[0].Statistics.PeakPopulation = 200
	world.Players[1].Statistics.PeakPopulation = 900
	world.Players[0].Statistics.ScenarioOptions = 1
	world.Players[1].Statistics.ScenarioOptions = 1
	g := &Game{World: world, Network: join, Screen: Playing, Updates: 80, Profile: engine.NewDeity("PLAYER"), LevelIndex: 27}
	want, err := engine.ScoreCampaign(uint32(world.Tick), world.Players[1].Statistics, world.Players[0].Statistics)
	if err != nil {
		t.Fatal(err)
	}
	g.finishWorld()
	if g.Screen != CampaignResult || g.ResultScore != want || g.resultAt != 80 {
		t.Fatal("joining player's result used the wrong camp", g.ResultScore, want)
	}
	g.Updates++
	g.finishWorld()
	if g.resultAt != 80 {
		t.Fatal("result presentation timer restarted")
	}
	profile := g.Profile
	if err := g.applyCampaignResult(); err != nil {
		t.Fatal(err)
	}
	if g.Network != nil || g.Screen != MainMenu || g.Profile != profile || g.LevelIndex != 27 {
		t.Fatal("two-player result altered conquest or retained the connection")
	}
	if !join.closed {
		t.Fatal("finished two-player connection was not closed")
	}
}
