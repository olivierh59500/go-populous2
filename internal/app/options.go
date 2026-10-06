package app

import (
	"fmt"
	"image"
	"image/draw"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous2/internal/engine"
)

type OptionsState struct {
	Draft              engine.Level
	Computer           [2]bool
	Owner, Page        int
	Return             Screen
	Music, Sound       bool
	SelectedPower      engine.PowerID
	Live               bool
	RulesLocked        bool
	SpecialCode        string
	EditingSpecialCode bool
}

func (g *Game) openOptions() error {
	if g.Network != nil {
		return fmt.Errorf("options cannot be changed during multiplayer")
	}
	level := g.Assets.Levels[g.LevelIndex]
	computer := [2]bool{false, true}
	live := g.World != nil && (g.Screen == Playing || g.Screen == InGameMenuScreen)
	if !live && g.CustomLevel != nil {
		level = *g.CustomLevel
		computer = g.CustomComputer
	}
	if live {
		level = g.World.Level
		for owner, p := range g.World.Players {
			computer[owner] = p.Computer
		}
	}
	musicEnabled, soundEnabled := true, true
	if g.music != nil {
		musicEnabled = g.music.IsMusicEnabled()
		soundEnabled = g.music.IsSoundEnabled()
	}
	g.Options = &OptionsState{Draft: level, Computer: computer, Return: g.Screen, Music: musicEnabled, Sound: soundEnabled, Live: live, Owner: g.playerSide(), RulesLocked: live && !g.CustomGame}
	g.Screen = OptionsScreen
	return nil
}

func (s *OptionsState) toggleRule(index int) bool {
	if s == nil || s.Owner < 0 || s.Owner > 1 {
		return false
	}
	p := &s.Draft.Players[s.Owner].Scenario
	var setting *bool
	switch index {
	case 0:
		setting = &p.BuildAnywhere
	case 1:
		setting = &p.SeaLevelOnly
	case 2:
		setting = &p.ForbidEnemyTerrain
	case 3:
		setting = &p.ForbidRaise
	case 4:
		setting = &p.ForbidLower
	case 5:
		setting = &p.FatalWater
	case 6:
		setting = &p.HideEnemy
	case 7:
		setting = &p.DisableEmigration
	case 8:
		setting = &p.HideDisasters
	case 9:
		setting = &p.ShallowSwamps
	default:
		return false
	}
	*setting = !*setting
	return true
}

func (s *OptionsState) rule(index int) bool {
	if s == nil || s.Owner < 0 || s.Owner > 1 {
		return false
	}
	p := s.Draft.Players[s.Owner].Scenario
	return [10]bool{p.BuildAnywhere, p.SeaLevelOnly, p.ForbidEnemyTerrain, p.ForbidRaise, p.ForbidLower, p.FatalWater, p.HideEnemy, p.DisableEmigration, p.HideDisasters, p.ShallowSwamps}[index]
}

func scenarioWord(s engine.ScenarioOptions) uint16 {
	values := [10]bool{s.BuildAnywhere, s.SeaLevelOnly, s.ForbidEnemyTerrain, s.ForbidRaise, s.ForbidLower, s.FatalWater, s.HideEnemy, s.DisableEmigration, s.HideDisasters, s.ShallowSwamps}
	var word uint16
	for bit, value := range values {
		if value {
			word |= 1 << uint(bit)
		}
	}
	return word
}

func (g *Game) applyOptions() error {
	if g.Options == nil {
		return fmt.Errorf("options draft missing")
	}
	if g.Network != nil {
		return fmt.Errorf("options cannot be changed during multiplayer")
	}
	d := g.Options.Draft
	if d.Landscape < 0 || d.Landscape >= len(g.Assets.Landscapes) {
		return fmt.Errorf("invalid landscape")
	}
	for owner, p := range d.Players {
		if p.Groups < 0 || p.Groups > engine.FollowerCapacity/2 || p.Population < 1 || p.Population > 1000000 || p.MovementSpeed == 0 || p.ReactionDelay < 0 || p.ReactionDelay > 1000 {
			return fmt.Errorf("invalid faction setup")
		}
		d.Players[owner].Extra[0] = scenarioWord(p.Scenario)
	}
	if g.Options.Live && g.World != nil {
		// Changing the landscape changes its real economy and artwork together.
		candidate, err := g.World.Snapshot().Restore()
		if err != nil {
			return err
		}
		candidate.Level = d
		candidate.Landscape = g.Assets.Landscapes[d.Landscape]
		for owner := range candidate.Players {
			candidate.Players[owner].Computer = g.Options.Computer[owner]
		}
		candidate.RefreshAIChoices()
		if _, err := candidate.Snapshot().Restore(); err != nil {
			return fmt.Errorf("changed game rules: %w", err)
		}
		g.World = candidate
	}
	g.CustomLevel = &d
	g.CustomComputer = g.Options.Computer
	if g.music != nil {
		g.music.SetMusic(g.Options.Music)
		g.music.SetSoundEnabled(g.Options.Sound)
	}
	g.Screen = g.Options.Return
	g.Options = nil
	return nil
}

func (g *Game) cancelOptions() {
	if g.Options != nil {
		g.Screen = g.Options.Return
		g.Options = nil
	}
}

// The original campaign requester displays rules without changing them.
// Custom games expose the same controls as editable settings.
func (g *Game) handleOptionsAction(action string) error {
	if g.Options == nil {
		return fmt.Errorf("options screen has no draft")
	}
	s := g.Options
	switch action {
	case "side":
		s.Owner = 1 - s.Owner
	case "reaction-decrease":
		if !s.RulesLocked {
			p := &s.Draft.Players[s.Owner]
			p.ReactionDelay = max(0, p.ReactionDelay-1)
		}
	case "reaction-increase":
		if !s.RulesLocked {
			p := &s.Draft.Players[s.Owner]
			p.ReactionDelay = min(15, p.ReactionDelay+1)
		}
	case "special-codes":
		s.EditingSpecialCode = true
		s.SpecialCode = ""
	case "proceed":
		return g.applyOptions()
	default:
		if !s.RulesLocked {
			for i := 0; i < 10; i++ {
				if action == fmt.Sprintf("rule-%d", i) {
					s.toggleRule(i)
					break
				}
			}
		}
	}
	return nil
}

func (s *OptionsState) acceptSpecialCode() {
	// The original music code recognizes the four-letter prefix MUSI.
	if strings.HasPrefix(s.SpecialCode, "MUSI") {
		s.Music = !s.Music
	}
	s.EditingSpecialCode = false
}

func (g *Game) updateOptions(x, y int, clicked bool) error {
	if g.Options == nil {
		return fmt.Errorf("options screen has no draft")
	}
	s := g.Options
	if s.EditingSpecialCode {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 'a' && r <= 'z' {
				r -= 32
			}
			if r >= 32 && r <= 126 && len(s.SpecialCode) < 17 {
				s.SpecialCode += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(s.SpecialCode) > 0 {
			s.SpecialCode = s.SpecialCode[:len(s.SpecialCode)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			s.acceptSpecialCode()
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return g.handleOptionsAction("proceed")
	}
	if clicked && g.Assets.OptionsArt != nil {
		return g.handleOptionsAction(g.Assets.OptionsArt.ActionAt(x, y, s.Draft.Players[s.Owner].ReactionDelay))
	}
	return nil
}

func (g *Game) drawOptions() {
	if g.Options == nil {
		return
	}
	s := g.Options
	art := g.Assets.OptionsArt
	if art == nil {
		draw.Draw(g.framebuffer, g.framebuffer.Bounds(), g.Assets.Visual.Startup, image.Point{}, draw.Src)
		g.text("GAME OPTIONS", 112, 14)
		g.text("PRESS ENTER TO APPLY", 80, 166)
		return
	}
	palette := art.Layout.Palette
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(palette[0]), image.Point{}, draw.Src)
	code := s.SpecialCode
	if s.EditingSpecialCode {
		code += "_"
	}
	values := map[string]string{"side": art.SideNames[s.Owner], "special-codes": code}
	flags := map[string]bool{}
	for i := 0; i < 10; i++ {
		flags[fmt.Sprintf("rule-%d", i)] = s.rule(i)
	}
	art.Layout.Draw(g.framebuffer, g.Assets.Visual.Font, values, flags)
	art.DrawReaction(g.framebuffer, g.Assets.Visual.Font, s.Draft.Players[s.Owner].ReactionDelay)
}
