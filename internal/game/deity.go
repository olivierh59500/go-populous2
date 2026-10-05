package game

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/populous2"
	"strings"
)

func (g *Game) OpenDeity() {
	g.DeityScreen = true
	g.Playing = false
	g.deityPassword = ""
	g.deityEditing = false
	g.deityNameEditing = false
	g.refreshDeityPortrait()
}

func (g *Game) refreshDeityPortrait() {
	image, err := g.Bundle.DeityArt.Portrait(g.Profile.FaceParts)
	if err != nil {
		g.notify(err.Error())
		return
	}
	g.deityPortrait = ebiten.NewImageFromImage(image)
}

func (g *Game) updateDeity() error {
	if g.deityNameEditing {
		for _, r := range ebiten.AppendInputChars(nil) {
			if len(g.Profile.Name) >= 15 {
				break
			}
			if r >= ' ' && r <= '~' {
				g.Profile.Name += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.Profile.Name) > 0 {
			g.Profile.Name = g.Profile.Name[:len(g.Profile.Name)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.deityNameEditing = false
			g.applyDeityProfile()
		}
	}
	if g.deityEditing {
		for _, r := range ebiten.AppendInputChars(nil) {
			if len(g.deityPassword) >= 16 {
				break
			}
			s := strings.ToUpper(string(r))
			if len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' {
				g.deityPassword += s
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.deityPassword) > 0 {
			g.deityPassword = g.deityPassword[:len(g.deityPassword)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.importDeityPassword()
		}
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if hit(x, y, 70, 276, 210, 24) {
		g.deityNameEditing = true
		g.deityEditing = false
		g.Profile.Name = ""
		return nil
	}
	for part := 0; part < 3; part++ {
		if hit(x, y, 70, 120+part*54, 32, 28) {
			g.Profile.CycleFace(part, -1)
			g.applyDeityProfile()
			g.refreshDeityPortrait()
			return nil
		}
		if hit(x, y, 252, 120+part*54, 32, 28) {
			g.Profile.CycleFace(part, 1)
			g.applyDeityProfile()
			g.refreshDeityPortrait()
			return nil
		}
	}
	for element := 0; element < 6; element++ {
		if hit(x, y, 328, 96+element*40, 228, 34) {
			g.Profile.AllocateBolt(populous2.Element(element))
			g.applyDeityProfile()
			return nil
		}
	}
	if hit(x, y, 80, 330, 480, 30) {
		g.deityEditing = true
		g.deityNameEditing = false
		g.deityPassword = ""
		return nil
	}
	if hit(x, y, 80, 373, 170, 32) {
		g.importDeityPassword()
		return nil
	}
	if hit(x, y, 390, 373, 170, 32) {
		g.applyDeityProfile()
		g.DeityScreen = false
		return nil
	}
	return nil
}

func (g *Game) importDeityPassword() {
	if err := g.Profile.SetPassword(g.deityPassword); err != nil {
		g.notify(err.Error())
		return
	}
	g.deityEditing = false
	g.applyDeityProfile()
	g.refreshDeityPortrait()
	g.notify("Profil importe.")
}

func (g *Game) applyDeityProfile() {
	g.World.Deity = g.Profile
	g.World.Experience[g.localPlayer()] = g.Profile.Experience
}

func (g *Game) drawDeity(screen *ebiten.Image) {
	panel(screen, 48, 54, 544, 374)
	label(screen, "CREER VOTRE DIEU", 98, 68, ink)
	if g.deityPortrait != nil {
		drawImage(screen, g.deityPortrait, 128, 96, 3)
	}
	for part, name := range []string{"COIFFE", "YEUX", "BOUCHE"} {
		button(screen, 70, 120+part*54, 32, 28, "<", true, false)
		button(screen, 252, 120+part*54, 32, 28, ">", true, false)
		label(screen, name, 103, 122+part*54+27, muted)
	}
	for element, name := range populous2.ElementNames {
		button(screen, 328, 96+element*40, 228, 34, fmt.Sprintf("%s  %3d", name, g.Profile.Experience[element]), g.Profile.Bolts > 0 && g.Profile.Experience[element] < 255, false)
	}
	label(screen, fmt.Sprintf("POINTS DISPONIBLES : %d", g.Profile.Bolts), 327, 68, gold)
	name := g.Profile.Name
	if g.deityNameEditing {
		name += "_"
	}
	button(screen, 70, 276, 210, 24, name, true, g.deityNameEditing)
	password, err := g.Profile.Password()
	if err != nil {
		password = "DISTRIBUEZ LES POINTS RESTANTS"
	}
	if g.deityEditing {
		password = g.deityPassword + "_"
	}
	button(screen, 80, 330, 480, 30, password, true, g.deityEditing)
	button(screen, 80, 373, 170, 32, "IMPORTER LE CODE", len(g.deityPassword) == 16, false)
	button(screen, 390, 373, 170, 32, "CONTINUER", true, false)
	if g.Message != "" {
		label(screen, truncate(g.Message, 75), 60, 434, ink)
	}
}
