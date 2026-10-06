package app

import (
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func hudInspectionGame(t *testing.T) *Game {
	g := browserGame(t)
	g.Assets.HUD = &visualassets.HUDArt{}
	g.World.Level.Players[0].Powers[engine.Perseus] = true
	g.World.Followers[10] = engine.Follower{State: engine.Walking, Owner: 0, X: 20, Y: 21, Hero: engine.HeroState{Kind: engine.HeroPerseus}}
	g.World.Followers[20] = engine.Follower{State: engine.Walking, Owner: 0, X: 40, Y: 41, Hero: engine.HeroState{Kind: engine.HeroHelen}}
	g.World.Players[0].Mana = 0
	return g
}

func TestHeroInspectionNewPressBypassesManaAndCyclesWithoutSpending(t *testing.T) {
	g := hudInspectionGame(t)
	before := g.World.Snapshot()
	if handled, err := g.handleHUDSecondary(82, 171, true); !handled || err != nil || g.SelectedFollower != 10 || g.heroScanCursor != 10 || g.CameraX != 16 || g.CameraY != 17 {
		t.Fatal("first original secondary hero scan differs", handled, err, g.SelectedFollower)
	}
	if handled, err := g.handleHUDSecondary(82, 171, true); !handled || err != nil || g.SelectedFollower != 20 || g.heroScanCursor != 20 {
		t.Fatal("next source scan did not include other hero kinds", handled, err, g.SelectedFollower)
	}
	if g.World.Snapshot() != before {
		t.Fatal("inspection cast a hero or spent mana")
	}
}

func TestHeldHeroInspectionUsesManaGateAndKeepsExistingCursor(t *testing.T) {
	g := hudInspectionGame(t)
	g.heroScanCursor = 10
	selected := g.SelectedFollower
	if handled, err := g.handleHUDSecondary(82, 171, false); !handled || err != nil || g.heroScanCursor != 10 || g.SelectedFollower != selected {
		t.Fatal("held inspection bypassed its source cost gate")
	}
	g.World.Players[0].Mana = g.World.PowerCost(0, engine.Perseus)
	if handled, err := g.handleHUDSecondary(82, 171, false); !handled || err != nil || g.SelectedFollower != 10 || g.heroScanCursor != 10 {
		t.Fatal("held inspection advanced instead of repeating its current hero", err)
	}
	g.World.Followers[20] = engine.Follower{}
	g.SelectionReturn.FramesLeft = 17
	if _, err := g.handleHUDSecondary(82, 171, true); err != nil || g.SelectedFollower != 10 || g.SelectionReturn.FramesLeft != 17 || g.heroScanCursor != 10 {
		t.Fatal("new press reselected the sole hero and restarted its timeout", err)
	}
}

func TestEmptyHeroScanAndRallyInspectionKeepOriginalSideEffects(t *testing.T) {
	g := hudInspectionGame(t)
	g.World.Followers[10], g.World.Followers[20] = engine.Follower{}, engine.Follower{}
	selected := g.SelectedFollower
	if _, err := g.handleHUDSecondary(82, 171, true); err != nil || g.heroScanCursor != 0 || g.SelectedFollower != selected {
		t.Fatal("empty scan changed its saved cursor or selection", err)
	}
	g.Assets.HUD.Controls[0] = "rally"
	leader := g.World.Players[0].Leader
	mode := g.World.Players[0].Mode
	if handled, err := g.handleHUDSecondary(319, 149, true); !handled || err != nil || g.SelectedFollower != leader || g.World.Players[0].Mode != mode {
		t.Fatal("secondary rally did not inspect its leader without mode change", err)
	}
	g.World.Players[0].Leader = 0
	g.World.Players[0].RallyX, g.World.Players[0].RallyY = 62, 1
	selected = g.SelectedFollower
	if _, err := g.handleHUDSecondary(319, 149, true); err != nil || g.SelectedFollower != selected || g.CameraX != 56 || g.CameraY != 0 {
		t.Fatal("leaderless rally did not recenter the magnet without selecting", err)
	}
}

func TestHUDHelpUsesEnabledHoveredPowerWithoutManaOrInspection(t *testing.T) {
	g := hudInspectionGame(t)
	g.Screen = Playing
	g.Assets.SpellHelp = &visualassets.SpellHelpArt{}
	g.World.Players[0].Mana = 0
	before := g.World.Snapshot()
	if handled, err := g.handleHUDHelp(82, 171); !handled || err != nil || g.Screen != PowerHelpScreen || g.PowerPreview == nil || g.PowerPreview.Power != engine.Perseus || g.PowerPreview.Return != Playing {
		t.Fatal("dedicated Help gesture did not open its enabled hovered icon", handled, err)
	}
	if g.World.Snapshot() != before {
		t.Fatal("help consumed mana or changed simulation")
	}
	g.closePowerHelp()
	g.World.Level.Players[0].Powers[engine.Perseus] = false
	if handled, err := g.handleHUDHelp(82, 171); !handled || err != nil || g.Screen != Playing {
		t.Fatal("disabled icon opened source help", err)
	}
}
