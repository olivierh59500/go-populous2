package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	var nameField, codeField []byte
	if g.deityNameEditing {
		nameField = []byte(g.Profile.Name + "_")
	}
	if g.deityEditing {
		codeField = []byte(g.deityPassword + "_")
	}
	frame, err := g.nativePresentation.DeityScreen(g.Bundle, g.Profile, nameField, codeField)
	if err != nil {
		g.notify(err.Error())
		return
	}
	g.deityRequester = frame.Requester
	g.deityPortrait = ebiten.NewImageFromImage(frame.Image)
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
			g.refreshDeityPortrait()
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
	if g.deityEditing || g.deityNameEditing {
		g.refreshDeityPortrait()
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if y < 40 {
		return nil
	}
	command := g.nativePresentation.Requesters.Click(g.deityRequester, x/2, (y-40)/2)
	action, err := g.nativePresentation.DeityAction(command, g.Bundle.Executable.Hunks[0].Data)
	if err != nil {
		return err
	}
	switch action.Kind {
	case "name":
		g.deityNameEditing, g.deityEditing = true, false
		g.Profile.Name = ""
	case "password":
		g.deityEditing, g.deityNameEditing = true, false
		g.deityPassword = ""
	case "allocate":
		g.Profile.AllocateBolt(action.Element)
		g.applyDeityProfile()
	case "face":
		g.Profile.CycleFace(action.Part, action.Direction)
		g.applyDeityProfile()
	case "continue":
		g.applyDeityProfile()
		g.DeityScreen = false
		return nil
	}
	g.refreshDeityPortrait()
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
	if g.deityPortrait != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(0, 40)
		screen.DrawImage(g.deityPortrait, op)
	}
	if g.messageTicks > 0 {
		label(screen, truncate(g.Message, 75), 12, 450, ink)
	}
}
