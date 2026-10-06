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
}

func TestFinalCampaignResultReturnsToMainMenu(t *testing.T) {
	g := &Game{World: &engine.World{Result: 1}, Profile: engine.NewDeity("PLAYER"), ResultScore: engine.CampaignScore{Value: 65035}, LevelIndex: 999}
	if err := g.applyCampaignResult(); err != nil {
		t.Fatal(err)
	}
	if !g.ResultProgress.Complete || g.LevelIndex != 0 || g.Screen != MainMenu {
		t.Fatal("final campaign transition differs", g.ResultProgress, g.Screen)
	}
}
