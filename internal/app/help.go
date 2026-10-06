package app

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"go-populous2/internal/engine"
)

var powerHelp = map[engine.PowerID]string{
	engine.RaiseLower:  "LEFT CLICK RAISES THE LAND.\nRIGHT CLICK RELEASES TOWNS\nOR LOWERS A TERRAIN CORNER.",
	engine.PapalMagnet: "PLACE THE RALLY DESTINATION.\nSELECT RALLY MODE TO GATHER\nFOLLOWERS AROUND THE LEADER.",
	engine.Lightning:   "LEFT CLICK PLACES THE MARKER.\nENTER ACTIVATES LIGHTNING.\nRIGHT CLICK DISMISSES IT.",
	engine.Basalt:      "MAKE A PASSAGE ACROSS WATER.\nQ AND E SELECT ITS DIRECTION.",
	engine.Plague:      "INFECT AN OPPOSING GROUP.\nINFECTION FOLLOWS MERGING\nAND NEW FOLLOWER GROUPS.",
	engine.Armageddon:  "COMMIT THE TWO POPULATIONS\nTO THEIR FINAL HERO BATTLE.",
	engine.Baptism:     "PLACE CONVERSION FONTS.\nGROUPS CROSSING THE FONTS\nCHANGE TO THE OPPOSING SIDE.",
	engine.Whirlwind:   "LIFT GROUPS AND CARRY THEM\nACROSS THE LANDSCAPE.",
	engine.Tsunami:     "START WAVES FROM THE WATER\nBESIDE THE SELECTED PARCEL.",
}

func (g *Game) drawHelp() {
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	g.text("HOW TO PLAY", 116, 14)
	for row, text := range []string{
		"ARROWS: MOVE THE VIEW",
		"CLICK PANEL POWER ICONS",
		"1-4: SETTLE RALLY JOIN FIGHT",
		"O: OPTIONS     P: MAP EDITOR",
		"F9: LOAD       F10: SAVE",
		"SPACE: PAUSE / RESUME",
		"I: INSPECT GROUPS / TOWNS",
		"F11 OVER AN ICON: SPELL HELP",
	} {
		g.text(text, 32, 32+row*10)
	}
	if power, ok := engine.PowerByID(g.Selected); ok {
		g.text(strings.ToUpper(power.Name), 32, 115)
		text := powerHelp[g.Selected]
		if g.Assets.SpellHelp != nil {
			lines := strings.Split(g.Assets.SpellHelp.Descriptor.Descriptions[int(g.Selected)], "\n")
			for i := range lines {
				if len(lines[i]) > 32 {
					lines[i] = lines[i][:32]
				}
			}
			text = strings.Join(lines[:min(2, len(lines))], "\n")
		}
		if text == "" {
			if _, hero := engine.HeroKindForPower(g.Selected); hero {
				text = "TRANSFORM YOUR LEADER INTO\nA HERO USING EARNED MANA."
			} else {
				text = "SELECT A TARGET PARCEL.\nTHE POWER USES YOUR MANA."
			}
		}
		g.text(text, 32, 129)
	}
	g.button("PREVIEW POWER", 96, 155, 128)
	g.button("RETURN", 120, 174, 80)
}
