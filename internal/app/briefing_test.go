package app

import (
	"reflect"
	"testing"

	"go-populous2/internal/engine"
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
