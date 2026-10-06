package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
)

func (g *Game) updateProfile(x, y int, clicked bool) {
	if clicked {
		if g.Assets.DeityLayout != nil {
			g.applyOriginalProfileAction(g.Assets.DeityLayout.ActionAt(x, y))
		} else {
			if y >= 169 && y < 190 {
				g.Screen = MainMenu
				g.editingProfileCode = false
				g.editingProfileName = false
				return
			}
			if x >= 194 && x < 240 && y >= 36 && y < 84 {
				g.Profile.CycleFace(min(2, (y-36)/16), 1)
			}
			if x >= 30 && x < 178 && y >= 49 && y < 65 {
				g.editingProfileName = true
				g.editingProfileCode = false
			}
			if x >= 20 && x < 300 && y >= 143 && y < 164 {
				g.editingProfileCode = true
				g.editingProfileName = false
				g.profileCodeInput = ""
			}
			if y >= 98 && y < 132 && x >= 20 && x < 296 {
				element := engine.Element(min(5, (x-20)/46))
				g.Profile.AllocateBolt(element)
			}
		}
	}
	if !g.editingProfileName && !g.editingProfileCode {
		return
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		if r < 'A' || r > 'Z' {
			if g.editingProfileName && r == ' ' && len(g.Profile.Name) < 16 {
				g.Profile.Name += " "
			}
			continue
		}
		if g.editingProfileName && len(g.Profile.Name) < 16 {
			g.Profile.Name += string(r)
		}
		if g.editingProfileCode && len(g.profileCodeInput) < 16 {
			g.profileCodeInput += string(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if g.editingProfileName && len(g.Profile.Name) > 0 {
			g.Profile.Name = g.Profile.Name[:len(g.Profile.Name)-1]
		}
		if g.editingProfileCode && len(g.profileCodeInput) > 0 {
			g.profileCodeInput = g.profileCodeInput[:len(g.profileCodeInput)-1]
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if g.editingProfileCode {
			if err := g.Profile.SetPassword(g.profileCodeInput); err != nil {
				g.Message = "INVALID PROFILE CODE"
				g.messageUntil = g.Updates + 150
			}
		}
		g.editingProfileCode = false
		g.editingProfileName = false
	}
}

func (g *Game) drawProfile() {
	if g.drawOriginalProfile() {
		return
	}
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	g.text("YOUR DEITY", 112, 15)
	name := g.Profile.Name
	if g.editingProfileName {
		name += "_"
	}
	g.text(name, 30, 52)
	portrait, err := g.Assets.Visual.Portrait(g.Profile.FaceParts)
	if err == nil {
		draw.Draw(g.framebuffer, image.Rect(202, 36, 234, 132), portrait, image.Point{}, draw.Over)
	}
	g.text(fmt.Sprintf("BOLTS %d", g.Profile.Bolts), 30, 78)
	for index, label := range [6]string{"PEOP", "PLNT", "EARTH", "AIR", "FIRE", "WATER"} {
		x := 20 + index*46
		g.button(label, x, 98, 44)
		g.text(fmt.Sprintf("%3d", g.Profile.Experience[index]), x+7, 121)
	}
	code, err := g.Profile.Password()
	if err != nil {
		code = "UNSPENT BOLTS TOO HIGH"
	}
	if g.editingProfileCode {
		code = g.profileCodeInput + "_"
	}
	g.text(strings.ToUpper(code), 20, 148)
	if g.Updates < g.messageUntil {
		g.text(g.Message, 20, 158)
	}
	g.button("BACK", 120, 171, 80)
}

func (g *Game) applyOriginalProfileAction(action string) {
	switch {
	case action == "proceed":
		g.Screen = MainMenu
		g.editingProfileCode = false
		g.editingProfileName = false
	case action == "name":
		g.editingProfileName = true
		g.editingProfileCode = false
	case action == "password":
		g.editingProfileCode = true
		g.editingProfileName = false
		g.profileCodeInput = ""
	case strings.HasPrefix(action, "experience-"):
		index, err := strconv.Atoi(strings.TrimPrefix(action, "experience-"))
		if err == nil && index >= 0 && index < 6 {
			g.Profile.AllocateBolt(engine.Element(index))
		}
	case strings.HasPrefix(action, "face-"):
		parts := strings.Split(action, "-")
		if len(parts) != 3 {
			return
		}
		index, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}
		direction := 1
		if parts[2] == "prev" {
			direction = -1
		}
		g.Profile.CycleFace(index, direction)
	}
}

func (g *Game) drawOriginalProfile() bool {
	layout, widgets := g.Assets.DeityLayout, g.Assets.DeityWidgets
	if layout == nil || widgets == nil {
		return false
	}
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(layout.Palette[0]), image.Point{}, draw.Src)
	password, err := g.Profile.Password()
	if err != nil {
		password = "INVALID BOLT COUNT"
	}
	if g.editingProfileCode {
		password = g.profileCodeInput
	}
	bolts := "NOTHING"
	if g.Profile.Bolts != 0 {
		bolts = strings.TrimSuffix(strings.Repeat("} ", min(8, int(g.Profile.Bolts))), " ")
	}
	layout.Draw(g.framebuffer, g.Assets.Visual.Font, map[string]string{"name": g.Profile.Name, "bolts": bolts, "password": password}, nil)
	widgets.Draw(g.framebuffer, g.Profile.Experience, g.Profile.FaceParts, g.Assets.Visual.PortraitParts)
	if g.Updates < g.messageUntil {
		g.drawMessage(152)
	}
	return true
}
