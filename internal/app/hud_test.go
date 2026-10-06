package app

import (
	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
	"testing"
)

func TestHUDCommandsUseOriginalSlantedGridsAndLocalMode(t *testing.T) {
	g := browserGame(t)
	g.Assets.HUD = &visualassets.HUDArt{}
	g.Assets.HUD.Controls[0] = "inspect"
	g.Assets.HUD.Controls[4] = "fight"
	if handled, err := g.handleHUDClick(302, 160); !handled || err != nil || g.World.Players[0].Mode != engine.Fight {
		t.Fatal("original mode grid did not dispatch local mode", err)
	}
	if handled, err := g.handleHUDClick(319, 149); !handled || err != nil || !g.Inspecting {
		t.Fatal("original inspect control did not select groups", err)
	}
	if handled, err := g.handleHUDClick(67, 141); !handled || err != nil || g.Category != engine.People {
		t.Fatal("category diamond did not select its panel", err)
	}
	g.World.Level.Players[0].Powers[engine.Trees] = true
	if handled, err := g.handleHUDClick(83, 149); !handled || err != nil || g.Category != engine.Plants {
		t.Fatal("second category diamond differs", err)
	}
	if handled, err := g.handleHUDClick(50, 155); !handled || err != nil || g.Selected != engine.Trees || g.Inspecting {
		t.Fatal("power diamond did not select available original power", err)
	}
	if handled, _ := g.handleHUDClick(192, 100); handled {
		t.Fatal("HUD consumed terrain click")
	}
}

func TestHUDDisabledPowerDoesNotChangeSelectionOrSpendMana(t *testing.T) {
	g := browserGame(t)
	g.Assets.HUD = &visualassets.HUDArt{}
	g.Category = engine.Plants
	before := g.World.Snapshot()
	selected := g.Selected
	if handled, err := g.handleHUDClick(50, 155); !handled || err == nil {
		t.Fatal("disabled power was silently selected")
	}
	if g.World.Snapshot() != before || g.Selected != selected {
		t.Fatal("disabled HUD power changed state")
	}
}

func TestHUDOriginalMenuIconOpensWindowWithoutChangingGame(t *testing.T) {
	g := browserGame(t)
	g.Assets.HUD = &visualassets.HUDArt{}
	g.Assets.HUD.Controls[0] = "menu"
	g.Assets.InGameLayout = &visualassets.RequesterLayout{}
	before := g.World.Snapshot()
	if handled, err := g.handleHUDClick(319, 149); !handled || err != nil || g.Screen != InGameMenuScreen {
		t.Fatal("original menu icon did not open the independent requester", err)
	}
	if g.World.Snapshot() != before {
		t.Fatal("opening HUD menu changed simulation")
	}
}
