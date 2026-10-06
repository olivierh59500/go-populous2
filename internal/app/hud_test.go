package app

import (
	"testing"

	"go-populous2/internal/engine"
)

func TestHUDCommandsUseLocalModeAndOpenDetachedScreens(t *testing.T) {
	g := browserGame(t)
	if handled, err := g.handleHUDClick(64, 122); !handled || err != nil || g.World.Players[0].Mode != engine.Fight {
		t.Fatal("fight control did not dispatch the local mode", err)
	}
	if handled, err := g.handleHUDClick(25, 85); !handled || err != nil || !g.PickingPower {
		t.Fatal("power control did not open the chooser", err)
	}
	g.PickingPower = false
	if handled, err := g.handleHUDClick(75, 146); !handled || err != nil || g.Screen != HelpScreen || g.helpReturn != Playing {
		t.Fatal("help control lost its return screen", err)
	}
	g.Screen = Playing
	before := g.World.Snapshot()
	if handled, err := g.handleHUDClick(74, 165); !handled || err != nil || g.Screen != SaveBrowserScreen || g.World.Snapshot() != before {
		t.Fatal("save control mutated the world or did not open its browser", err)
	}
	if handled, _ := g.handleHUDClick(192, 100); handled {
		t.Fatal("HUD consumed a terrain click")
	}
}
