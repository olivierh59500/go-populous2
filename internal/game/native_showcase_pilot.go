package game

import (
	"fmt"
	"strings"
)

// NativeShowcasePilot presents the retained menus before handing control to
// the regular, input-only settlement player. Captions follow the visible
// controller rather than an assumed sequence of loading times.
type NativeShowcasePilot struct {
	Caption string
	player  *NativePresentationPilot
	phase   int
	since   int
	clicked bool
	help    bool
	gameAt  int
	heroAt  int
}

func NewNativeShowcasePilot() *NativeShowcasePilot {
	return &NativeShowcasePilot{player: NewNativePresentationPilot()}
}

func (p *NativeShowcasePilot) Status(g *NativeGame) string {
	if g != nil && g.Frame != nil {
		return p.player.Status(g)
	}
	return fmt.Sprintf("menu tour phase %d: %s", p.phase, strings.ReplaceAll(p.Caption, "\n", " "))
}

// Next uses normal mouse motion, held clicks and releases. No menu register,
// terrain, resource, campaign selection or outcome is changed directly.
func (p *NativeShowcasePilot) Next(g *NativeGame) (NativeInput, error) {
	if g == nil || g.Host == nil {
		return NativeInput{}, fmt.Errorf("showcase pilot has no native game")
	}
	if !p.player.started {
		p.player.started = true
		p.player.x, p.player.y = 155, 95
		p.player.failed = make(map[int]int)
	}
	if g.Frame != nil {
		if p.gameAt == 0 {
			p.gameAt = g.Updates
		}
		input, err := p.player.Next(g)
		if p.heroAt == 0 && (strings.Contains(p.player.Stage, "hero") || strings.Contains(p.player.Stage, "Hero")) {
			p.heroAt = g.Updates
		}
		p.Caption = p.gameCaption(g.Updates - p.gameAt)
		return input, err
	}
	if len(p.player.queue) != 0 {
		return p.player.advanceAction(g)
	}
	menuReady := g.Director.Menu != nil && (g.Director.Menu.Menu.PC == 0x3bd8 || g.Director.Menu.Menu.PC == 0x3bb8) && !g.Director.Menu.Menu.ChildActive
	deityReady := g.DeityEditor.State != nil && g.DeityEditor.State.PC == 0xb7ec
	chooserReady := g.Director.Selection != nil && (g.Director.Selection.PC == 0x3d9a || g.Director.Selection.PC == 0x3dea) && !g.Director.Selection.ChildActive
	helpReady := g.Director.Children.Help.Started && !g.Director.Children.Help.Finished && g.Director.Children.Help.PC == 0x51dc
	switch p.phase {
	case 0:
		p.Caption = "Populous II Go\nA recreated god game: shape the land and lead your followers."
		if menuReady {
			p.enter(g, 1)
		}
	case 1:
		if g.Updates-p.since < 250 {
			p.Caption = "Populous II Go\nA recreated god game: shape the land and lead your followers."
		} else {
			p.Caption = "The main menu offers conquest, custom worlds and saved games.\nFirst, let's look at our deity profile."
		}
		if menuReady && g.Updates-p.since >= 500 && !p.clicked {
			return p.action(g, 2)
		}
		if deityReady {
			p.enter(g, 2)
		}
	case 2:
		p.Caption = "Your deity profile stores your name, appearance and experience.\nCampaign victories develop your divine powers."
		if deityReady && g.Updates-p.since >= 650 && !p.clicked {
			action, err := p.branchAction(g, 0xb882, 0xba50, 110)
			if err != nil {
				return p.player.idle(), err
			}
			return p.action(g, action)
		}
		if menuReady && p.clicked {
			p.enter(g, 3)
		}
	case 3:
		p.Caption = "Let's begin a conquest.\nEach world has its own opponent, landscape and available powers."
		if menuReady && g.Updates-p.since >= 250 && !p.clicked {
			return p.action(g, 4)
		}
		if chooserReady {
			p.enter(g, 4)
		}
	case 4:
		p.Caption = "The world briefing shows the enemy and the power selection.\nClick a power icon to read its explanation."
		if chooserReady && g.Updates-p.since >= 500 && !p.clicked {
			x, y, ok, err := p.helpPosition(g)
			if err != nil {
				return p.player.idle(), err
			}
			if ok {
				p.clicked = true
				p.player.click(x, y, false, -1, 0)
				return p.player.advanceAction(g)
			}
			p.enter(g, 6)
		}
		if helpReady {
			p.help = true
			p.enter(g, 5)
		}
		if p.clicked && !p.help && g.Updates-p.since > 900 && chooserReady {
			p.enter(g, 6)
		}
	case 5:
		p.Caption = "Power help explains how to use the selected ability.\nSelect OK to return to the world briefing."
		if helpReady && g.Updates-p.since >= 550 && !p.clicked {
			return p.action(g, 2)
		}
		if chooserReady && p.clicked {
			p.enter(g, 6)
		}
	case 6:
		p.Caption = "Proceed to the first conquest.\nWe'll build strong settlements before confronting the enemy."
		if chooserReady && g.Updates-p.since >= 250 && !p.clicked {
			return p.action(g, 6)
		}
	}
	return p.player.idle(), nil
}

func (p *NativeShowcasePilot) enter(g *NativeGame, phase int) {
	p.phase, p.since, p.clicked = phase, g.Updates, false
}

func (p *NativeShowcasePilot) action(g *NativeGame, action int) (NativeInput, error) {
	x, y, err := g.nativeActionPosition(action)
	if err != nil {
		return p.player.idle(), err
	}
	p.clicked = true
	p.player.click(x+3, y+3, false, -1, 0)
	return p.player.advanceAction(g)
}

func (p *NativeShowcasePilot) branchAction(g *NativeGame, table, target, length int) (int, error) {
	for action := 2; action < length; action += 2 {
		value, err := g.Host.Memory.Code.Read16(table + action)
		if err != nil {
			return 0, err
		}
		if table+int(int16(value)) == target {
			return action, nil
		}
	}
	return 0, fmt.Errorf("showcase requester exit is unavailable")
}

func (p *NativeShowcasePilot) helpPosition(g *NativeGame) (int, int, bool, error) {
	profile, err := g.Host.Memory.BSS.Read16(0xeb42)
	if err != nil {
		return 0, 0, false, err
	}
	for slot := 0; slot < 36; slot++ {
		icon, err := g.Host.Memory.Code.Read16(0x21102 + slot*2)
		if err != nil {
			return 0, 0, false, err
		}
		allowed, err := g.Host.Memory.BSS.Read8(0xe76a + int(profile)*314 + 0x70 + slot)
		if err != nil {
			return 0, 0, false, err
		}
		if icon == 0 || int8(allowed) <= 0 {
			continue
		}
		row, column := 5-slot/6, slot%6
		if column >= 5 {
			continue
		}
		// Inverse of the original chooser's isometric mouse-to-power test.
		x := 126 + 16*(column+5-row-row)
		y := 8*(column+5) - 2
		return x, y, true, nil
	}
	return 0, 0, false, nil
}

func (p *NativeShowcasePilot) gameCaption(age int) string {
	switch {
	case age < 500:
		return "Our followers need level ground to build homes.\nThe minimap locates both populations across the world."
	case age < 1800:
		return "Raise or lower the terrain to create useful building space.\nNeighboring settlements keep their own stable plateaus."
	case age < 3000:
		return "More homes support a growing population.\nProductive settlements supply the mana needed for divine powers."
	case age < 4500:
		return "Expand steadily and protect established towns.\nLand shaping is deliberate: each action spends real game resources."
	case p.heroAt != 0 && p.gameAt+age-p.heroAt < 600:
		return "A hero can turn your followers' strength into an offensive force.\nPrepare the expedition while maintaining the settlement economy."
	case p.player.SpellActions > 0 && age < 11000:
		return "Divine powers consume mana, so timing matters.\nPressure the opponent while keeping your own settlements productive."
	case age < 8000:
		return "Watch the enemy while your population and mana increase.\nA strong economy makes the next divine intervention possible."
	case age < 12000:
		return "Balance expansion with divine intervention.\nThe world continues at its normal gameplay cadence throughout."
	default:
		return "This is the opening of a conquest, with much more still to explore.\nBuild, cast powers, lead heroes and progress through the campaign."
	}
}
