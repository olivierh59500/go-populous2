package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/populous2"
)

func (g *Game) OpenScenarioRules() {
	g.ScenarioScreen = true
	g.scenarioSide = g.localPlayer()
	g.scenarioCodeEditing = false
	if err := g.refreshNativeOptions(); err != nil {
		g.failure = err
	}
}

func (g *Game) displayedScenarioRule(side int) populous2.ScenarioRules {
	if !g.Playing && !g.World.Custom {
		return g.Bundle.Levels[g.LevelIndex].Players[side].ScenarioRules()
	}
	return g.World.Rules[side]
}

func (g *Game) refreshNativeOptions() error {
	god, _ := populous2.NativeDeityAddress(uint8(g.scenarioSide + 1))
	speed, _ := (populous2.NativeRuntimeMemory{Records: &g.World.RecordImage, Globals: &g.World.NativeGlobals}).Read16(god + 0x68)
	plan, err := g.nativePresentation.Options(uint16(g.scenarioSide+1), g.displayedScenarioRule(g.scenarioSide).Raw, speed, []byte(g.scenarioSpecialCode))
	if err != nil {
		return err
	}
	img, err := g.nativePresentation.Compose(make([]byte, 320*200), plan.Requester, g.World.Landscape.Palettes[0])
	if err != nil {
		return err
	}
	g.scenarioPresentation = plan
	g.scenarioImage = ebiten.NewImageFromImage(img)
	return nil
}

func (g *Game) updateScenarioRules() error {
	if g.scenarioCodeEditing {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= ' ' && r <= '~' && len(g.scenarioSpecialCode) < 17 {
				g.scenarioSpecialCode += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.scenarioSpecialCode) > 0 {
			g.scenarioSpecialCode = g.scenarioSpecialCode[:len(g.scenarioSpecialCode)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.scenarioCodeEditing = false
			if len(g.scenarioSpecialCode) >= 4 && g.scenarioSpecialCode[:4] == "MUSI" {
				g.specialMusicDisabled = !g.specialMusicDisabled
				if g.audioReplay != nil {
					g.audioReplay.SetMusic(!g.specialMusicDisabled)
				}
			}
		}
		if err := g.refreshNativeOptions(); err != nil {
			return err
		}
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if y < 40 {
		return nil
	}
	action := g.scenarioPresentation.Rules.Click(g.scenarioPresentation.Requester, x/2, (y-40)/2)
	if g.World.NativeGameMode == 2 && action > 2 && action <= 26 {
		return nil
	}
	switch {
	case action == 2:
		g.scenarioSide ^= 1
	case action >= 4 && action <= 22 && action&1 == 0:
		raw := g.World.Rules[g.scenarioSide].Raw ^ (1 << uint((action-4)/2))
		g.World.Rules[g.scenarioSide] = populous2.DecodeScenarioRules(raw)
		g.World.Level.Players[g.scenarioSide].Parameters[6] = raw
		g.hasCustomScenarioOptions = true
		for side, rule := range g.World.Rules {
			g.customScenarioOptions[side] = rule.Raw
		}
	case action == 24 || action == 26:
		god, _ := populous2.NativeDeityAddress(uint8(g.scenarioSide + 1))
		offset := god + 0x68
		speed, _ := (populous2.NativeRuntimeMemory{Records: &g.World.RecordImage, Globals: &g.World.NativeGlobals}).Read16(offset)
		delta := int16(-1)
		if action == 26 {
			delta = 1
		}
		next := uint16(int16(speed) + delta)
		if int16(next) >= 0 && next != 16 {
			if _, err := (populous2.NativeRuntimeMemory{Records: &g.World.RecordImage, Globals: &g.World.NativeGlobals}).Write16(offset, next); err != nil {
				return err
			}
			g.World.Level.Players[g.scenarioSide].Parameters[7] = next
		}
	case action == 28:
		g.scenarioCodeEditing = true
		g.scenarioSpecialCode = ""
	case action == 30:
		g.ScenarioScreen = false
		return nil
	}
	return g.refreshNativeOptions()
}

func (g *Game) drawScenarioRules(screen *ebiten.Image) {
	if g.scenarioImage == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(0, 40)
	screen.DrawImage(g.scenarioImage, op)
}
