package app

import (
	"testing"

	"go-populous2/internal/engine"
)

func TestOriginalProfileActionsChangeOnlyNamedProfileFields(t *testing.T) {
	g := &Game{Profile: engine.NewDeity("PLAYER"), Screen: DeityProfile}
	g.applyOriginalProfileAction("experience-2")
	if g.Profile.Experience[engine.Earth] != 1 || g.Profile.Bolts != 4 {
		t.Fatal("original experience strip did not allocate one bolt")
	}
	g.applyOriginalProfileAction("face-1-prev")
	if g.Profile.FaceParts[1] != 7 {
		t.Fatal("original previous-face arrow did not wrap")
	}
	g.applyOriginalProfileAction("password")
	if !g.editingProfileCode || g.editingProfileName {
		t.Fatal("original password field did not open editing")
	}
	g.applyOriginalProfileAction("name")
	// The password modal consumes its terminating click before another
	// action can begin editing a different field.
	g.applyOriginalProfileAction("name")
	if !g.editingProfileName || g.editingProfileCode {
		t.Fatal("original name field did not open editing")
	}
	g.applyOriginalProfileAction("proceed")
	if g.Screen != MainMenu || g.editingProfileName || g.editingProfileCode {
		t.Fatal("original Proceed did not close the profile")
	}
}

func TestProfileNameInputKeepsPrintableCharactersAndCursorEditing(t *testing.T) {
	g := &Game{Profile: engine.NewDeity("ZEUS"), Screen: DeityProfile}
	g.beginProfileName()
	g.applyProfileInput(profileInput{Characters: []rune(" 2!é")})
	if g.Profile.Name != "ZEUS 2!" {
		t.Fatal("name lost printable ASCII or admitted an unsupported glyph", g.Profile.Name)
	}
	g.applyProfileInput(profileInput{Left: true})
	g.applyProfileInput(profileInput{Characters: []rune("a")})
	if g.Profile.Name != "ZEUS 2A!" {
		t.Fatal("cursor insertion or uppercase normalization changed", g.Profile.Name)
	}
	g.applyProfileInput(profileInput{Backspace: true})
	if g.Profile.Name != "ZEUS 2!" {
		t.Fatal("backspace did not remove the character before the caret", g.Profile.Name)
	}
	g.applyProfileInput(profileInput{Characters: []rune("LONGER THAN THE FIELD")})
	if len(g.Profile.Name) != 15 {
		t.Fatal("name exceeded the original terminated buffer", g.Profile.Name)
	}
	g.applyProfileInput(profileInput{Enter: true})
	if g.Screen != DeityProfile || g.editingProfileName {
		t.Fatal("name confirmation closed its profile requester")
	}
}

func TestProfilePasswordValidationConsumesItsClickAndPreservesInvalidProfile(t *testing.T) {
	g := &Game{Profile: engine.NewDeity("KEEP"), Screen: DeityProfile}
	before := g.Profile
	g.beginProfileCode()
	g.applyProfileInput(profileInput{Characters: []rune("12!INVALID")})
	g.applyOriginalProfileAction("proceed")
	if g.Profile != before || g.Screen != DeityProfile || g.editingProfileCode {
		t.Fatal("invalid password changed the profile or dispatched its terminating click")
	}
	want := engine.NewDeity("UNCHANGED NAME")
	want.Bolts = 3
	want.Experience[engine.Earth] = 7
	code, err := want.Password()
	if err != nil {
		t.Fatal(err)
	}
	g.beginProfileCode()
	g.applyProfileInput(profileInput{Characters: []rune(code), Enter: true})
	if g.Profile.Name != "KEEP" || g.Profile.Bolts != 3 || g.Profile.Experience[engine.Earth] != 7 || g.Screen != DeityProfile {
		t.Fatal("valid password did not update its named properties atomically")
	}
	g.applyOriginalProfileAction("proceed")
	if g.Screen != MainMenu {
		t.Fatal("subsequent profile OK did not return normally")
	}
}

func TestProfileNameModalDispatchesTheActionThatFinishesIt(t *testing.T) {
	g := &Game{Profile: engine.NewDeity("KEEP"), Screen: DeityProfile, profileReturn: ConquestBriefing}
	g.beginProfileName()
	g.applyProfileInput(profileInput{Characters: []rune(" 2")})
	g.applyOriginalProfileAction("proceed")
	if g.Profile.Name != "KEEP 2" || g.Screen != ConquestBriefing || g.editingProfileName {
		t.Fatal("name modal failed to preserve direct edits and dispatch OK")
	}
}
