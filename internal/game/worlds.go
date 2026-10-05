package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous2/internal/populous2"
)

func (g *Game) openWorldRequester() error {
	// Conquest initialization precedes the requester; the world itself starts
	// running only after the original Proceed action.
	w, err := populous2.NewWorld(g.Bundle, g.LevelIndex, false)
	if err != nil {
		return err
	}
	g.World = w
	g.applyDeityProfile()
	if err := w.SwitchNativeProfile(1, &g.requesterContext); err != nil {
		return err
	}
	if err := w.LoadNativeWorldMenu(g.Bundle, uint16(g.LevelIndex), &g.requesterContext); err != nil {
		return err
	}
	g.Profile = w.Deity
	g.Playing = false
	g.worldMenu = true
	g.worldMenuChild = false
	g.worldMenuEditing = false
	g.resetNativeResultPresentation()
	return g.refreshWorldRequester()
}

func (g *Game) refreshWorldRequester() error {
	state, err := g.World.NativeWorldMenuState()
	if err != nil {
		return err
	}
	var field []byte
	if g.worldMenuEditing {
		field = append(append([]byte(nil), g.worldMenuCode...), '_')
	}
	frame, err := g.worldRequesters.WorldScreen(g.Bundle, state, make([]byte, 320*200), field)
	if err != nil {
		return err
	}
	g.worldMenuState = state
	g.worldMenuRequester = frame.Requester
	if g.worldMenuImage == nil {
		g.worldMenuImage = ebiten.NewImageFromImage(frame.Image)
	} else {
		g.worldMenuImage.WritePixels(frame.Image.Pix)
	}
	return nil
}

func (g *Game) updateWorldRequester() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.worldMenuChild || g.worldMenuEditing {
			g.worldMenuChild, g.worldMenuEditing = false, false
			return g.refreshWorldRequester()
		}
		g.worldMenu = false
		return nil
	}
	if g.worldMenuEditing {
		for _, ch := range ebiten.AppendInputChars(nil) {
			// Both original raw-key banks encode letters in uppercase.
			if ch >= 'a' && ch <= 'z' {
				ch -= 'a' - 'A'
			}
			if ch >= ' ' && ch <= '~' && len(g.worldMenuCode) < 38 {
				g.worldMenuCode = append(g.worldMenuCode, byte(ch))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.worldMenuCode) > 0 {
			g.worldMenuCode = g.worldMenuCode[:len(g.worldMenuCode)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.worldMenuEditing = false
			step := g.worldRequesters.FinishWorldCode(g.worldMenuCode)
			if step.InvalidCode {
				plan, err := g.worldRequesters.InvalidWorldCodePlan()
				if err != nil {
					return err
				}
				palette, err := populous2.NativeWorldPalette(g.Bundle)
				if err != nil {
					return err
				}
				img, err := g.nativePresentation.Overlay(plan, palette)
				if err != nil {
					return err
				}
				g.worldMenuImage.DrawImage(ebiten.NewImageFromImage(img), nil)
				g.worldMenuRequester = plan
				g.worldMenuChild = true
				return nil
			}
			if err := g.World.LoadNativeWorldMenu(g.Bundle, step.World, &g.requesterContext); err != nil {
				return err
			}
			g.LevelIndex = int(step.World)
			g.Profile = g.World.Deity
		}
		return g.refreshWorldRequester()
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if y < 40 {
		return nil
	}
	x, y = x/2, (y-40)/2
	if g.worldMenuChild {
		if g.nativePresentation.Requesters.Click(g.worldMenuRequester, x, y) == 2 {
			g.worldMenuChild = false
			return g.refreshWorldRequester()
		}
		return nil
	}
	step, err := g.worldRequesters.WorldClick(g.worldMenuState, uint16(x), uint16(y))
	if err != nil {
		return err
	}
	switch {
	case step.EditCode:
		g.worldMenuEditing = true
		g.worldMenuCode = nil
		return g.refreshWorldRequester()
	case step.ShowOpponent:
		world, reaction, aggression, err := g.World.NativeOpponentMenuState()
		if err != nil {
			return err
		}
		frame, err := g.worldRequesters.OpponentScreen(g.Bundle, world, reaction, aggression, make([]byte, 320*200))
		if err != nil {
			return err
		}
		g.worldMenuRequester, g.worldMenuChild = frame.Requester, true
		g.worldMenuImage.WritePixels(frame.Image.Pix)
	case step.ShowSpellHelp:
		frame, visible, err := g.worldRequesters.SpellHelpBase(g.Bundle, g.worldMenuState, step.SpellTableOffset, make([]byte, 320*200))
		if err != nil {
			return err
		}
		if visible {
			g.worldMenuRequester, g.worldMenuChild = frame.Requester, true
			g.worldMenuImage.WritePixels(frame.Image.Pix)
		}
	case step.Proceed:
		g.worldMenu = false
		return g.start(g.LevelIndex, false, false)
	case step.Cancel:
		g.worldMenu = false
	}
	return nil
}

func (g *Game) drawWorldRequester(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(0, 40)
	screen.DrawImage(g.worldMenuImage, op)
}
