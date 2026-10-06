package app

import (
	"reflect"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func TestBriefingShowsNamedEnglishOpponentDataInsteadOfAuthorNotes(t *testing.T) {
	level := engine.Level{Code: "ALPHA", OpponentText: "\x82 CREER VOTRE DIEU", OpponentExperience: [6]uint8{1, 2, 3, 4, 5, 6}}
	level.Players[1] = engine.PlayerOptions{Groups: 8, Population: 123, MovementSpeed: 24, Weapons: 7}
	g := &Game{Assets: &Assets{Levels: []engine.Level{level}}}
	want := []string{"WORLD 0", "ALPHA", "XP PEOPLE 1 PLANTS 2", "XP EARTH 3 AIR 4", "XP FIRE 5 WATER 6", "ENEMY GROUPS 8 PEOPLE 123", "SPEED 24 WEAPONS 7"}
	if got := g.briefingLines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("briefing does not describe its actual opponent in English: %q", got)
	}
	for _, line := range g.briefingLines() {
		if len(line) > 32 {
			t.Fatal("briefing exceeds its original text area", line)
		}
		for _, glyph := range []byte(line) {
			if glyph < 32 || glyph > 126 {
				t.Fatal("campaign author bytes leaked into interface text")
			}
		}
	}
	g.editingWorldCode, g.worldCodeInput = true, "OTHER"
	if got := g.briefingLines()[1]; got != "OTHER_" {
		t.Fatal("English opponent display changed world-code editing", got)
	}
}

func TestConquestRequesterActionsPreserveLiveWorldUntilProceed(t *testing.T) {
	g := menuTestGame(t)
	g.Screen = ConquestBriefing
	g.Assets.Conquest = &visualassets.ConquestArt{Descriptor: visualassets.ConquestDescriptor{Layout: visualassets.RequesterLayout{Actions: []visualassets.RequesterAction{{Name: "world-code", X: 96, Y: 0, Width: 88, Height: 8}, {Name: "opponent", X: 16, Y: 168, Width: 304, Height: 8}, {Name: "proceed", X: 16, Y: 192, Width: 80, Height: 8}, {Name: "cancel", X: 224, Y: 192, Width: 72, Height: 8}}}}}
	before, profile := g.World.Snapshot(), g.Profile
	if err := g.updateBriefing(110, 4, true); err != nil || !g.editingWorldCode {
		t.Fatal("world-code field did not start editing", err)
	}
	g.worldCodeInput = "INVALID"
	if g.acceptBriefingCode() || !g.editingWorldCode || g.LevelIndex != 0 {
		t.Fatal("invalid code was accepted")
	}
	g.worldCodeInput = engine.CodeForLevel(0)
	if !g.acceptBriefingCode() || g.editingWorldCode {
		t.Fatal("valid code was not accepted")
	}
	if err := g.updateBriefing(90, 172, true); err != nil || !g.briefingOpponent {
		t.Fatal("opponent biography did not open", err)
	}
	if err := g.updateBriefing(0, 0, true); err != nil || g.briefingOpponent {
		t.Fatal("opponent page did not return", err)
	}
	if err := g.updateBriefing(240, 196, true); err != nil || g.Screen != MainMenu {
		t.Fatal("cancel did not restore main menu", err)
	}
	if g.World.Snapshot() != before || g.Profile != profile {
		t.Fatal("briefing navigation changed live gameplay or the deity")
	}
	g.Screen = ConquestBriefing
	if err := g.updateBriefing(40, 196, true); err != nil || g.Screen != Playing || g.World == nil {
		t.Fatal("proceed did not start conquest", err)
	}
}

func TestConquestPowerHelpUsesOnlyEnabledOriginalIconSlots(t *testing.T) {
	g := menuTestGame(t)
	g.Screen = ConquestBriefing
	g.Assets.Conquest = &visualassets.ConquestArt{}
	before := g.World.Snapshot()
	if err := g.updateBriefing(174, 38, true); err != nil || g.Screen != ConquestBriefing || g.PowerPreview != nil {
		t.Fatal("disabled fire column icon opened help", err)
	}
	g.Assets.Levels[0].Players[0].Powers[engine.FireColumn] = true
	if err := g.updateBriefing(174, 38, true); err != nil || g.Screen != PowerHelpScreen || g.PowerPreview == nil || g.PowerPreview.Power != engine.FireColumn || g.PowerPreview.Return != ConquestBriefing {
		t.Fatal("enabled icon did not open its actual power preview", err)
	}
	if g.World.Snapshot() != before {
		t.Fatal("briefing power preview changed the live world")
	}
}

func TestConquestValuesDescribeCustomRulesAndSelectedOpponent(t *testing.T) {
	g := menuTestGame(t)
	g.Assets.Conquest = &visualassets.ConquestArt{Descriptor: visualassets.ConquestDescriptor{LandscapeNames: [4]string{"FERTILE", "WINTER", "BARREN", "SLUDGE"}}}
	g.Assets.Conquest.Descriptor.Opponents[0].Name = "EPIMETHEUS"
	custom := g.Assets.Levels[0]
	custom.Landscape = 3
	custom.Players[0].Scenario = engine.ScenarioOptions{FatalWater: true, HideDisasters: true, ForbidRaise: true}
	g.CustomGame, g.CustomLevel = true, &custom
	values, flags := g.briefingValues()
	if values["landscape"] != "SLUDGE" || values["opponent"] != "EPIMETHEUS" || !flags["fatal-water"] || !flags["hide-disasters"] || !flags["forbid-raise"] || flags["build-anywhere"] {
		t.Fatal("custom briefing used campaign defaults", values, flags)
	}
	g.editingWorldCode, g.worldCodeInput = true, "EDIT"
	values, _ = g.briefingValues()
	if values["world-code"] != "EDIT_" {
		t.Fatal("world-code editing is not visible")
	}
}
