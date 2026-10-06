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
	if !g.editingProfileName || g.editingProfileCode {
		t.Fatal("original name field did not open editing")
	}
	g.applyOriginalProfileAction("proceed")
	if g.Screen != MainMenu || g.editingProfileName || g.editingProfileCode {
		t.Fatal("original Proceed did not close the profile")
	}
}
