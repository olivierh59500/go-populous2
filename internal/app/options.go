package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"go-populous2/internal/engine"
)

type OptionsState struct {
	Draft         engine.Level
	Computer      [2]bool
	Owner, Page   int
	Return        Screen
	Music, Sound  bool
	SelectedPower engine.PowerID
	Live          bool
}

func (g *Game) openOptions() error {
	if g.Network != nil {
		return fmt.Errorf("options cannot be changed during multiplayer")
	}
	level := g.Assets.Levels[g.LevelIndex]
	computer := [2]bool{false, true}
	live := g.World != nil && g.Screen == Playing
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
	g.Options = &OptionsState{Draft: level, Computer: computer, Return: g.Screen, Music: musicEnabled, Sound: soundEnabled, Live: live}
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

var optionLabels = [10]string{"BUILD ANYWHERE", "SEA LEVEL BUILDING", "PROTECT ENEMY LAND", "FORBID RAISING", "FORBID LOWERING", "FATAL WATER", "HIDE ENEMY ON MAP", "DISABLE MANUAL SPROG", "HIDE DISASTERS", "SHALLOW SWAMPS"}

func (g *Game) updateOptions(x, y int, clicked bool) error {
	if g.Options == nil {
		return fmt.Errorf("options screen has no draft")
	}
	if !clicked {
		return nil
	}
	s := g.Options
	if y >= 171 && y < 190 {
		if x >= 24 && x < 136 {
			return g.applyOptions()
		}
		if x >= 176 && x < 296 {
			g.cancelOptions()
			return nil
		}
	}
	if y >= 28 && y < 45 {
		if x >= 16 && x < 144 {
			s.Owner = 0
		}
		if x >= 176 && x < 304 {
			s.Owner = 1
		}
	}
	if y >= 48 && y < 64 {
		if x >= 16 && x < 108 {
			s.Page = 0
		}
		if x >= 112 && x < 204 {
			s.Page = 1
		}
		if x >= 208 && x < 304 {
			s.Page = 2
		}
	}
	if s.Page == 0 {
		if y >= 67 && y < 167 {
			row := (y - 67) / 10
			s.toggleRule(row)
		}
	} else if s.Page == 1 {
		for i, p := range engine.Powers {
			col, row := i/10, i%10
			if x >= 16+col*101 && x < 113+col*101 && y >= 67+row*10 && y < 77+row*10 {
				s.Draft.Players[s.Owner].Powers[p.ID] = !s.Draft.Players[s.Owner].Powers[p.ID]
				s.SelectedPower = p.ID
			}
		}
	} else if y >= 67 && y < 163 {
		p := &s.Draft.Players[s.Owner]
		delta := -1
		if x >= 160 {
			delta = 1
		}
		switch (y - 67) / 12 {
		case 0:
			s.Computer[s.Owner] = !s.Computer[s.Owner]
		case 1:
			p.Groups = max(0, min(engine.FollowerCapacity/2, p.Groups+delta))
		case 2:
			p.Population = max(1, min(1000000, p.Population+delta*100))
		case 3:
			p.Mana = max(0, min(1000000, p.Mana+delta*1000))
		case 4:
			p.ReactionDelay = max(0, min(1000, p.ReactionDelay+delta))
		case 5:
			s.Draft.Landscape = (s.Draft.Landscape + delta + 4) % 4
		case 6:
			s.Music = !s.Music
		case 7:
			s.Sound = !s.Sound
		}
	}
	return nil
}

func (g *Game) drawOptions() {
	if g.Options == nil {
		return
	}
	s := g.Options
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	title := "GAME OPTIONS / BLUE"
	if s.Owner == 1 {
		title = "GAME OPTIONS / RED"
	}
	g.text(title, 72, 10)
	g.button("BLUE", 16, 29, 128)
	g.button("RED", 176, 29, 128)
	g.button("RULES", 16, 49, 92)
	g.button("POWERS", 112, 49, 92)
	g.button("SETUP", 208, 49, 96)
	if s.Page == 0 {
		for i, label := range optionLabels {
			mark := "[ ]"
			if s.rule(i) {
				mark = "[X]"
			}
			g.text(mark+" "+label, 24, 68+i*10)
		}
	} else if s.Page == 1 {
		for i, p := range engine.Powers {
			col, row := i/10, i%10
			label := strings.ToUpper(p.Name)
			if len(label) > 9 {
				label = label[:9]
			}
			mark := "-"
			if s.Draft.Players[s.Owner].Powers[p.ID] {
				mark = "+"
			}
			g.text(mark+label, 16+col*101, 68+row*10)
		}
	} else {
		p := s.Draft.Players[s.Owner]
		controller := "HUMAN"
		if s.Computer[s.Owner] {
			controller = "COMPUTER"
		}
		music := "OFF"
		if s.Music {
			music = "ON"
		}
		sound := "OFF"
		if s.Sound {
			sound = "ON"
		}
		for i, label := range []string{controller, fmt.Sprintf("START GROUPS %d", p.Groups), fmt.Sprintf("GROUP PEOPLE %d", p.Population), fmt.Sprintf("START MANA %d", p.Mana), fmt.Sprintf("AI REACTION %d", p.ReactionDelay), fmt.Sprintf("LANDSCAPE %d", s.Draft.Landscape), "MUSIC " + music, "SOUND " + sound} {
			g.text("- "+label+" +", 24, 68+i*12)
		}
	}
	if s.Page == 2 {
		g.text("START VALUES: NEXT GAME", 48, 163)
	}
	if s.Page == 1 {
		if power, ok := engine.PowerByID(s.SelectedPower); ok {
			g.text(strings.ToUpper(power.Name), 24, 163)
		}
	}
	g.button("APPLY", 24, 172, 112)
	g.button("CANCEL", 176, 172, 120)
}
