package app

import (
	"testing"

	"go-populous2/internal/engine"
)

func menuTestGame(t *testing.T) *Game {
	t.Helper()
	var land engine.Landscape
	for i := 1; i < engine.TownStages; i++ {
		land.WorkTicks[i] = 8
		land.EmigrationDivisor[i] = 3
		land.PopulationLimit[i] = 100 + i*100
	}
	level := engine.Level{Seed: 4311, Landscape: 0}
	for owner := range level.Players {
		level.Players[owner] = engine.PlayerOptions{Groups: 2, Population: 100, Mana: 1000, MovementSpeed: 20, ReactionDelay: 10, Scenario: engine.ScenarioOptions{BuildAnywhere: true}}
	}
	w, err := engine.NewWorld(level, land)
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{Screen: Playing, World: w, Assets: &Assets{Levels: []engine.Level{level}, Landscapes: [4]engine.Landscape{land, land, land, land}}}
	return g
}

func TestOptionsDraftAndCancelLeaveLiveRulesUntouched(t *testing.T) {
	g := menuTestGame(t)
	before := *g.World
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	g.Options.Owner = 1
	g.Options.toggleRule(3)
	g.Options.Draft.Players[1].Powers[engine.Perseus] = true
	if *g.World != before {
		t.Fatal("editing options mutated the running game")
	}
	g.cancelOptions()
	if *g.World != before || g.Screen != Playing || g.Options != nil {
		t.Fatal("cancel did not preserve live rules")
	}
}

func TestOptionsApplyIsAtomicAndKeepsFactionSettingsSeparate(t *testing.T) {
	g := menuTestGame(t)
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	g.Options.Owner = 1
	g.Options.toggleRule(3)
	g.Options.Computer[1] = false
	g.Options.Draft.Players[1].Powers[engine.FireColumn] = true
	g.Options.Draft.Landscape = 2
	if err := g.applyOptions(); err != nil {
		t.Fatal(err)
	}
	if !g.World.Level.Players[1].Scenario.ForbidRaise || g.World.Level.Players[0].Scenario.ForbidRaise || g.World.Players[1].Computer || g.World.Level.Landscape != 2 {
		t.Fatal("per-side options or landscape did not apply")
	}
	if g.World.Level.Players[1].Extra[0]&(1<<3) == 0 || g.CustomLevel == nil || g.CustomComputer[1] {
		t.Fatal("options scoring word or next-game setup did not follow semantic choices")
	}
	found := false
	for _, choice := range g.World.AI[1].Choices[:g.World.AI[1].ChoiceCount] {
		found = found || choice.Power == engine.FireColumn
	}
	if !found {
		t.Fatal("live power change did not reach the AI choice catalogue")
	}
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	before := *g.World
	g.Options.Draft.Players[0].MovementSpeed = 0
	if err := g.applyOptions(); err == nil || *g.World != before {
		t.Fatal("invalid setup partially changed a live world")
	}
}

func TestOptionsKeyboardBoundaryRowsDoNotMutateSetup(t *testing.T) {
	g := menuTestGame(t)
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	g.Options.Page = 2
	before := *g.Options
	if err := g.updateOptions(0, 20, true); err != nil {
		t.Fatal(err)
	}
	if *g.Options != before {
		t.Fatal("click above setup rows altered faction configuration")
	}
}

func TestInvalidLandscapeEconomyDoesNotPartiallyApplyOptions(t *testing.T) {
	g := menuTestGame(t)
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	before := g.World.Snapshot()
	g.Options.Draft.Landscape = 2
	g.Options.Computer[1] = false
	g.Assets.Landscapes[2].WorkTicks[4] = 0
	if err := g.applyOptions(); err == nil || g.World.Snapshot() != before || g.Screen != OptionsScreen {
		t.Fatal("invalid landscape partially changed live rules")
	}
}

func TestMenuOptionsEditCustomSetupWithoutMutatingPausedWorld(t *testing.T) {
	g := menuTestGame(t)
	g.Screen = MainMenu
	draft := g.Assets.Levels[0]
	draft.Players[0].Population = 999
	g.CustomLevel = &draft
	before := *g.World
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	if g.Options.Live || g.Options.Draft.Players[0].Population != 999 {
		t.Fatal("menu options selected an old paused world instead of custom setup")
	}
	g.Options.Draft.Players[0].Population = 1234
	if err := g.applyOptions(); err != nil {
		t.Fatal(err)
	}
	if *g.World != before || g.CustomLevel.Players[0].Population != 1234 {
		t.Fatal("menu setup mutated paused gameplay or lost next-game settings")
	}
}

func TestOptionsAndEditorRejectMultiplayerMutation(t *testing.T) {
	g := menuTestGame(t)
	g.Network = &NetworkController{}
	before := *g.World
	if err := g.openOptions(); err == nil {
		t.Fatal("network game opened mutable options")
	}
	if err := g.openEditor(); err == nil {
		t.Fatal("network game opened the map editor")
	}
	if *g.World != before {
		t.Fatal("network rejection mutated simulation")
	}
}
