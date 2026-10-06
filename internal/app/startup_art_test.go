package app

import (
	"go-populous2/internal/visualassets"
	"testing"
)

func TestOriginalStartupActionsBindIndependentScreens(t *testing.T) {
	for _, test := range []struct {
		action visualassets.StartupAction
		screen Screen
		quit   bool
	}{
		{visualassets.StartupProfile, DeityProfile, false}, {visualassets.StartupConquest, ConquestBriefing, false}, {visualassets.StartupCustom, OptionsScreen, false}, {visualassets.StartupQuit, MainMenu, true},
	} {
		g := menuTestGame(t)
		g.Screen = MainMenu
		g.Assets.StartupMenu = &visualassets.StartupMenu{Regions: []visualassets.StartupHitRegion{{Action: test.action, X: 80, Y: 88, Width: 152, Height: 8}}}
		handled, quit, err := g.handleOriginalStartup(88, 92)
		if err != nil || !handled || quit != test.quit || g.Screen != test.screen {
			t.Fatal("startup semantic binding differs", test.action, g.Screen, quit, err)
		}
		if test.action == visualassets.StartupCustom && (g.Options == nil || g.Options.Return != ConquestBriefing || !g.CustomGame) {
			t.Fatal("custom startup did not prepare independent setup")
		}
	}
}
