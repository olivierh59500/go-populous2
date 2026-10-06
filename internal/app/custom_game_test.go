package app

import (
	"testing"

	"go-populous2/internal/engine"
)

func TestCustomStartupUsesItsOwnRulesAndRetainsPreparedEdits(t *testing.T) {
	g := menuTestGame(t)
	g.Screen = MainMenu
	g.Assets.Levels[0].Players[0].Scenario.FatalWater = true
	g.Assets.Levels[0].Players[0].Powers[engine.FireRain] = true
	if err := g.startCustomGame(); err != nil {
		t.Fatal(err)
	}
	if g.Screen != Playing || !g.CustomGame || g.Options != nil || g.World.Level.Seed != 0x058028af {
		t.Fatal("custom game did not begin directly with its original seed")
	}
	for side, p := range g.World.Level.Players {
		mana := 800
		if side == 1 {
			mana = 200
		}
		if p.Groups != 1 || p.Population != 50 || p.MovementSpeed != 20 || p.Weapons != 1 || p.Mana != mana || p.Attrition != 2 || p.ReactionDelay != 1 || p.Scenario.FatalWater || p.Powers[engine.FireRain] {
			t.Fatal("campaign setup leaked into custom defaults", side, p)
		}
	}
	draft := *g.CustomLevel
	draft.Landscape = 2
	draft.Players[0].Population = 321
	draft.Players[0].Powers[engine.FireRain] = true
	g.CustomLevel = &draft
	g.CustomComputer = [2]bool{true, false}
	g.Screen = MainMenu
	if err := g.startCustomGame(); err != nil {
		t.Fatal(err)
	}
	if g.World.Level.Landscape != 2 || g.World.Level.Players[0].Population != 321 || !g.World.Level.Players[0].Powers[engine.FireRain] || !g.World.Players[0].Computer || g.World.Players[1].Computer {
		t.Fatal("restarting custom play discarded its committed settings")
	}
}

func TestRejectedCustomStartLeavesLiveSessionAndSetupUnchanged(t *testing.T) {
	g := menuTestGame(t)
	host, _, _, _ := controllerPair(t)
	g.Network = host
	before := g.World.Snapshot()
	if err := g.startCustomGame(); err == nil {
		t.Fatal("custom game replaced an active network session")
	}
	if g.World.Snapshot() != before || g.CustomLevel != nil || g.CustomGame {
		t.Fatal("rejected custom action partially changed live setup")
	}
}

func TestProfileReturnIsChosenByItsCaller(t *testing.T) {
	g := &Game{Profile: engine.NewDeity("PLAYER")}
	g.openDeityProfile(ConquestBriefing)
	g.applyOriginalProfileAction("proceed")
	if g.Screen != ConquestBriefing {
		t.Fatal("campaign profile returned through the main menu")
	}
	g.openDeityProfile(MainMenu)
	g.applyOriginalProfileAction("proceed")
	if g.Screen != MainMenu {
		t.Fatal("manual deity creation retained a stale campaign return")
	}
}
