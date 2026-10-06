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
	g.World.Players[0].Mana = 10000
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
	if handled, err := g.handleHUDClick(50, 155); !handled || err != nil {
		t.Fatal("disabled power did not retain the original silent admission", err)
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

func TestFireIconAdmissionAndBarsUseOriginalPanelPriceWithoutCasting(t *testing.T) {
	g := browserGame(t)
	g.Assets.HUD = &visualassets.HUDArt{}
	g.Category = engine.Fire
	g.World.Level.Players[0].Powers[engine.Achilles] = true
	g.World.Level.Players[0].Powers[engine.Volcano] = true
	g.World.Players[0].Experience = [6]uint8{}
	g.World.Players[0].Mana = 39999
	before, selected := g.World.Snapshot(), g.Selected
	if handled, err := g.handleHUDClick(82, 171); !handled || err != nil || g.Selected != selected {
		t.Fatal("original physical hero icon bypassed its panel-price gate", handled, err)
	}
	if g.World.Snapshot() != before {
		t.Fatal("rejected icon selection changed the simulation")
	}
	g.World.Players[0].Mana = 40000
	before = g.World.Snapshot()
	if handled, err := g.handleHUDClick(82, 171); !handled || err != nil || g.Selected != engine.Achilles {
		t.Fatal("Achilles icon used the higher creator price for panel admission", handled, err, g.Selected)
	}
	state := g.hudState(0, 0)
	if state.Costs[engine.Achilles] != 10000 || state.Costs[engine.Volcano] != 20000 || g.World.Snapshot() != before {
		t.Fatal("original physical bar costs were crossed or selecting cast a spell")
	}
}
