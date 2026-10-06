package app

import (
	"image"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func TestEveryPowerPreviewUsesARealIndependentCast(t *testing.T) {
	assets := menuTestGame(t).Assets
	for _, power := range engine.Powers {
		t.Run(power.Name, func(t *testing.T) {
			preview, err := newPowerPreview(assets, power.ID)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Power != power.ID || preview.World.Players[0].Mana >= 1000000 && power.ID != engine.Lightning {
				t.Fatal("preview did not execute its real casting path")
			}
			for range 20 {
				preview.World.Step()
			}
			if _, err := preview.World.Snapshot().Restore(); err != nil {
				t.Fatalf("real preview produced invalid continuation: %v", err)
			}
		})
	}
}

func TestOpeningPowerPreviewDoesNotChangeLiveContinuationOrProfile(t *testing.T) {
	g := menuTestGame(t)
	g.Profile = engine.NewDeity("KEEP")
	g.Assets.Visual = &visualassets.Bundle{Background: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	g.framebuffer = image.NewRGBA(image.Rect(0, 0, 320, 200))
	before := g.World.Snapshot()
	profile := g.Profile
	sounds := g.AnimationSounds
	if err := g.openPowerHelp(engine.FireColumn); err != nil {
		t.Fatal(err)
	}
	for range 40 {
		if err := g.updatePowerHelp(0, 0, false); err != nil {
			t.Fatal(err)
		}
	}
	for range 25 {
		g.drawPowerHelp()
	}
	if g.World.Snapshot() != before || g.Profile != profile {
		t.Fatal("power help modified live gameplay or profile")
	}
	if g.AnimationSounds != sounds {
		t.Fatal("preview rendering consumed live animation sound transitions")
	}
	if err := g.updatePowerHelp(120, 185, true); err != nil {
		t.Fatal(err)
	}
	if g.Screen != Playing || g.PowerPreview != nil {
		t.Fatal("preview Back did not restore caller")
	}
}
