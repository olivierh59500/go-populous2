package app

import (
	"image"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
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

func originalOptionsTestArt() *visualassets.OptionsArt {
	return &visualassets.OptionsArt{ReactionPosition: image.Pt(152, 128), Layout: visualassets.RequesterLayout{Actions: []visualassets.RequesterAction{
		{Name: "side", X: 16, Y: 24, Width: 304, Height: 8},
		{Name: "rule-3", X: 16, Y: 64, Width: 304, Height: 8},
		{Name: "special-codes", X: 152, Y: 144, Width: 136, Height: 8},
		{Name: "proceed", X: 120, Y: 168, Width: 56, Height: 8},
	}}}
}

func TestOptionsReactionButtonsRespectOriginalZeroToFifteenRange(t *testing.T) {
	g := menuTestGame(t)
	g.CustomGame = true
	g.Assets.OptionsArt = originalOptionsTestArt()
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	g.Options.Draft.Players[0].ReactionDelay = 15
	if err := g.updateOptions(272, 132, true); err != nil {
		t.Fatal(err)
	}
	if g.Options.Draft.Players[0].ReactionDelay != 15 {
		t.Fatal("reaction exceeded the original maximum")
	}
	g.Options.Draft.Players[0].ReactionDelay = 0
	if err := g.updateOptions(144, 132, true); err != nil {
		t.Fatal(err)
	}
	if g.Options.Draft.Players[0].ReactionDelay != 0 {
		t.Fatal("reaction went below original minimum")
	}
	g.Options.Draft.Players[0].ReactionDelay = 7
	if err := g.updateOptions(152, 132, true); err != nil {
		t.Fatal(err)
	}
	if g.Options.Draft.Players[0].ReactionDelay != 6 {
		t.Fatal("left of the reaction thumb did not decrease its value")
	}
	if err := g.updateOptions(200, 132, true); err != nil {
		t.Fatal(err)
	}
	if g.Options.Draft.Players[0].ReactionDelay != 7 {
		t.Fatal("right of the reaction thumb did not increase its value")
	}
}

func TestOriginalOptionsHasNoInventedEditorButton(t *testing.T) {
	g := menuTestGame(t)
	g.Assets.OptionsArt = originalOptionsTestArt()
	before := g.World.Snapshot()
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	if err := g.updateOptions(160, 34, true); err != nil {
		t.Fatal(err)
	}
	if g.Screen != OptionsScreen || g.Editor != nil || g.Options == nil || g.World.Snapshot() != before {
		t.Fatal("blank original requester row triggered an unrelated editor transition")
	}
}

func TestCampaignOptionsDisplayButDoNotChangeRulesOrReaction(t *testing.T) {
	g := menuTestGame(t)
	g.Assets.OptionsArt = originalOptionsTestArt()
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	if !g.Options.RulesLocked {
		t.Fatal("campaign rule controls were not protected")
	}
	before := g.Options.Draft
	if err := g.updateOptions(40, 68, true); err != nil {
		t.Fatal(err)
	}
	if err := g.updateOptions(144, 132, true); err != nil {
		t.Fatal(err)
	}
	if g.Options.Draft != before {
		t.Fatal("protected campaign controls changed the world settings")
	}
	if err := g.updateOptions(40, 28, true); err != nil || g.Options.Owner != 1 {
		t.Fatal("protected options did not allow viewing the other faction", err)
	}
}

func TestCustomOptionsRuleAndMusicEditsRemainAtomicUntilOK(t *testing.T) {
	g := menuTestGame(t)
	g.CustomGame = true
	g.Assets.OptionsArt = originalOptionsTestArt()
	before := g.World.Snapshot()
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	if err := g.updateOptions(40, 68, true); err != nil {
		t.Fatal(err)
	}
	if !g.Options.Draft.Players[0].Scenario.ForbidRaise {
		t.Fatal("original rule row did not toggle its named setting")
	}
	if err := g.updateOptions(170, 148, true); err != nil || !g.Options.EditingSpecialCode {
		t.Fatal("special codes field did not begin editing", err)
	}
	g.Options.SpecialCode = "MUSIC"
	g.Options.acceptSpecialCode()
	if g.Options.Music || g.Options.EditingSpecialCode {
		t.Fatal("MUSIC did not toggle the draft soundtrack option")
	}
	if g.World.Snapshot() != before {
		t.Fatal("editing options changed the live game before confirmation")
	}
	g.cancelOptions()
	if g.World.Snapshot() != before || g.Screen != Playing {
		t.Fatal("cancel changed the live continuation")
	}
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	if err := g.updateOptions(40, 68, true); err != nil {
		t.Fatal(err)
	}
	if err := g.updateOptions(136, 172, true); err != nil || g.Screen != Playing || g.Options != nil || !g.World.Level.Players[0].Scenario.ForbidRaise {
		t.Fatal("original OK row did not apply the draft", err)
	}
}
