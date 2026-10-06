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
				g.closeDeityProfile()
				return
			}
			if x >= 194 && x < 240 && y >= 36 && y < 84 {
				g.Profile.CycleFace(min(2, (y-36)/16), 1)
			}
			if x >= 30 && x < 178 && y >= 49 && y < 65 {
				g.beginProfileName()
			}
			if x >= 20 && x < 300 && y >= 143 && y < 164 {
				g.beginProfileCode()
			}
			if y >= 98 && y < 132 && x >= 20 && x < 296 {
				element := engine.Element(min(5, (x-20)/46))
				g.Profile.AllocateBolt(element)
			}
		}
	}
	g.applyProfileInput(profileInput{Characters: ebiten.AppendInputChars(nil), Backspace: inpututil.IsKeyJustPressed(ebiten.KeyBackspace) || inpututil.IsKeyJustPressed(ebiten.KeyDelete), Left: inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft), Right: inpututil.IsKeyJustPressed(ebiten.KeyArrowRight), Enter: inpututil.IsKeyJustPressed(ebiten.KeyEnter)})
}

type profileInput struct {
	Characters                    []rune
	Backspace, Left, Right, Enter bool
}

func (g *Game) beginProfileName() {
	g.editingProfileName, g.editingProfileCode = true, false
	g.profileCaret = len(g.Profile.Name)
}
func (g *Game) beginProfileCode() {
	g.editingProfileCode, g.editingProfileName = true, false
	g.profileCodeInput = ""
	g.profileCaret = 0
}

func (g *Game) finishProfileInput() {
	if g.editingProfileCode {
		if err := g.Profile.SetPassword(g.profileCodeInput); err != nil {
			g.Message, g.messageUntil = "INVALID PROFILE CODE", g.Updates+150
		}
	}
	g.editingProfileCode, g.editingProfileName = false, false
}

func (g *Game) applyProfileInput(input profileInput) {
	if !g.editingProfileName && !g.editingProfileCode {
		return
	}
	value, limit := &g.Profile.Name, 15
	if g.editingProfileCode {
		value, limit = &g.profileCodeInput, 19
	}
	g.profileCaret = min(max(g.profileCaret, 0), len(*value))
	if input.Left {
		g.profileCaret = max(0, g.profileCaret-1)
	}
	if input.Right {
		g.profileCaret = min(len(*value), g.profileCaret+1)
	}
	if input.Backspace && g.profileCaret > 0 {
		*value = (*value)[:g.profileCaret-1] + (*value)[g.profileCaret:]
		g.profileCaret--
	}
	for _, r := range input.Characters {
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		if r < 32 || r > 126 || len(*value) >= limit {
			continue
		}
		*value = (*value)[:g.profileCaret] + string(r) + (*value)[g.profileCaret:]
		g.profileCaret++
	}
	if input.Enter {
		g.finishProfileInput()
	}
}

func (g *Game) profileFieldText(value string, width int) string {
	caret := min(max(g.profileCaret, 0), len(value))
	start := max(0, caret-width+1)
	if start > 0 {
		value = value[start:]
		caret -= start
	}
	if g.Updates&16 != 0 {
		if caret == len(value) {
			value += "m"
		} else {
			value = value[:caret] + "m" + value[caret+1:]
		}
	}
	return value
}

func (g *Game) openDeityProfile(returnScreen Screen) {
	g.profileReturn = returnScreen
	g.editingProfileName, g.editingProfileCode = false, false
	g.Screen = DeityProfile
}

func (g *Game) closeDeityProfile() {
	g.Screen = g.profileReturn
	g.editingProfileName, g.editingProfileCode = false, false
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
	if action == "" {
		return
	}
	if g.editingProfileCode {
		// A click finishes and validates this modal; its action is not dispatched
		// until a new click after the password requester has been rebuilt.
		g.finishProfileInput()
		return
	}
	if g.editingProfileName {
		g.finishProfileInput()
	}
	switch {
	case action == "proceed":
		g.closeDeityProfile()
	case action == "name":
		g.beginProfileName()
	case action == "password":
		g.beginProfileCode()
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
		password = g.profileFieldText(g.profileCodeInput, 19)
	}
	bolts := "NOTHING"
	if g.Profile.Bolts != 0 {
		bolts = strings.TrimSuffix(strings.Repeat("} ", min(8, int(g.Profile.Bolts))), " ")
	}
	name := g.Profile.Name
	if g.editingProfileName {
		name = g.profileFieldText(name, 16)
	}
	layout.Draw(g.framebuffer, g.Assets.Visual.Font, map[string]string{"name": name, "bolts": bolts, "password": password}, nil)
	widgets.Draw(g.framebuffer, g.Profile.Experience, g.Profile.FaceParts, g.Assets.Visual.PortraitParts)
	if g.Updates < g.messageUntil {
		g.drawMessage(152)
	}
	return true
}
