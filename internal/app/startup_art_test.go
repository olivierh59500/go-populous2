package app

import (
	"path/filepath"
	"testing"

	"go-populous2/internal/visualassets"
)

func TestOriginalStartupActionsBindIndependentScreens(t *testing.T) {
	for _, test := range []struct {
		action visualassets.StartupAction
		screen Screen
		quit   bool
	}{
		{visualassets.StartupProfile, DeityProfile, false}, {visualassets.StartupConquest, ConquestBriefing, false}, {visualassets.StartupCustom, Playing, false}, {visualassets.StartupLoad, SaveBrowserScreen, false}, {visualassets.StartupQuit, MainMenu, true},
	} {
		g := menuTestGame(t)
		g.Screen = MainMenu
		g.SavePath = filepath.Join(t.TempDir(), "game.json")
		g.Assets.StartupMenu = &visualassets.StartupMenu{Regions: []visualassets.StartupHitRegion{{Action: test.action, X: 80, Y: 88, Width: 152, Height: 8}}}
		handled, quit, err := g.handleOriginalStartup(88, 92)
		if err != nil || !handled || quit != test.quit || g.Screen != test.screen {
			t.Fatal("startup semantic binding differs", test.action, g.Screen, quit, err)
		}
		if test.action == visualassets.StartupCustom && (g.Options != nil || !g.CustomGame || g.CustomLevel == nil || g.World.Level.Players[0].Groups != 1 || g.World.Level.Players[0].Population != 50) {
			t.Fatal("custom startup did not enter its original starting game directly")
		}
		if test.action == visualassets.StartupLoad && (g.SaveBrowser == nil || g.SaveBrowser.Saving || g.SaveBrowser.Return != MainMenu) {
			t.Fatal("load startup did not open the independent load browser")
		}
	}
}
