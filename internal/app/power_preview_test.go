package app

import (
	"image"
	"image/color"
	"image/draw"
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

func TestOriginalSpellHelpRunsAtTenFramesWithoutAdvancingLiveGame(t *testing.T) {
	g := menuTestGame(t)
	background := image.NewRGBA(image.Rect(0, 0, 320, 200))
	draw.Draw(background, background.Bounds(), image.NewUniform(color.RGBA{10, 20, 30, 255}), image.Point{}, draw.Src)
	font := &visualassets.Font{FirstCode: 32, Width: 8, Height: 8, Glyphs: make([][]uint8, 96)}
	for i := range font.Glyphs {
		font.Glyphs[i] = make([]uint8, 64)
	}
	g.Assets.Visual = &visualassets.Bundle{Background: background, Font: font}
	g.framebuffer = image.NewRGBA(image.Rect(0, 0, 320, 200))
	art := &visualassets.SpellHelpArt{}
	art.Descriptor.Layout = visualassets.RequesterLayout{Version: 1, Name: "spell-help", Columns: 40, Rows: 22, Actions: []visualassets.RequesterAction{{Name: "return", X: 128, Y: 152, Width: 56, Height: 8}}}
	art.Descriptor.Layout.Palette[0] = color.RGBA{0, 0, 0, 255}
	for row := 0; row < 22; row++ {
		for column := 0; column < 40; column++ {
			art.Descriptor.Layout.Cells = append(art.Descriptor.Layout.Cells, visualassets.GlyphCell{Column: column, Row: row, Glyph: ' '})
		}
	}
	for _, c := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
		frame := image.NewRGBA(image.Rect(0, 0, 1, 1))
		frame.SetRGBA(0, 0, c)
		art.Sequences[0][int(engine.FireColumn)].Frames = append(art.Sequences[0][int(engine.FireColumn)].Frames, visualassets.HelpFrame{Image: frame, Position: image.Pt(160, 32)})
	}
	g.Assets.SpellHelp = art
	before, profile, sounds := g.World.Snapshot(), g.Profile, g.AnimationSounds
	if err := g.openPowerHelp(engine.FireColumn); err != nil {
		t.Fatal(err)
	}
	if g.PowerPreview.World != nil || g.PowerPreview.Sheet == nil {
		t.Fatal("original sheet created an unnecessary simulation")
	}
	g.drawPowerHelp()
	if got := g.framebuffer.RGBAAt(160, 32); got != (color.RGBA{255, 0, 0, 255}) {
		t.Fatal("first original graphic frame was not displayed", got)
	}
	if got := g.framebuffer.RGBAAt(1, 190); got != (color.RGBA{10, 20, 30, 255}) {
		t.Fatal("help covered pixels outside its original requester", got)
	}
	for range 50 {
		if err := g.updatePowerHelp(0, 0, false); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			g.drawPowerHelp()
		}
	}
	if g.PowerPreview.Sheet.Age != 10 || g.PowerPreview.Sheet.LastSoundAge != 10 {
		t.Fatal("original graphic sequence did not retain the ten-frame clock")
	}
	if g.World.Snapshot() != before || g.Profile != profile || g.AnimationSounds != sounds {
		t.Fatal("help drawing or playback changed the paused gameplay continuation")
	}
	if err := g.updatePowerHelp(160, 185, true); err != nil || g.Screen != PowerHelpScreen {
		t.Fatal("click outside the original OK region closed help", err)
	}
	if err := g.updatePowerHelp(140, 156, true); err != nil || g.Screen != Playing || g.PowerPreview != nil {
		t.Fatal("original OK region did not restore its caller", err)
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
