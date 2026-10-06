package game

import (
	"fmt"
	"strings"
)

// NativeShowcasePilot presents the retained menus before handing control to
// the regular, input-only settlement player. Captions follow the visible
// controller rather than an assumed sequence of loading times.
type NativeShowcasePilot struct {
	Caption       string
	player        *NativePresentationPilot
	phase         int
	since         int
	clicked       bool
	help          bool
	gameAt        int
	actionCaption string
	actionUntil   int
	seenActions   map[string]bool
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
		p.observeAction(g)
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
	if p.actionCaption != "" && p.gameAt+age < p.actionUntil {
		return p.actionCaption
	}
	switch {
	case age < 500:
		return "Our followers need level ground to build homes.\nThe minimap locates both populations across the world."
	case age < 1800:
		return "Raise or lower the terrain to create useful building space.\nNeighboring settlements keep their own stable plateaus."
	case age < 2800:
		return "More homes support a growing population.\nProductive settlements supply the mana needed for divine powers."
	case age < 3800:
		return "Expand steadily and protect established towns.\nLand shaping is deliberate: each action spends real game resources."
	case age < 4800:
		return "Watch the enemy while your population and mana increase.\nA strong economy makes the next divine intervention possible."
	case age < 5800:
		return "Settlements need room to expand, so finish one plateau at a time.\nKeep established homes supported while creating new building space."
	case age < 6800:
		return "A growing population can support an expedition.\nKeep productive towns behind to fund further divine powers."
	case age < 7800:
		return "The magnet directs your leader and gathering followers.\nConcentrating a force gives you more control over the next advance."
	case age < 8800:
		return "The enemy keeps building while our followers move across the world.\nUse the overview to follow the expedition and inspect opposing towns."
	case age < 9800:
		return "Followers have distinct modes for settling, rallying and fighting.\nChoose the behavior that matches the current tactical situation."
	case age < 10800:
		return "Spells need a target and enough earned mana.\nReserve resources for construction while putting pressure on the enemy."
	case age < 12000:
		return "Balance expansion with divine intervention.\nThe world continues at its normal gameplay cadence throughout."
	case age < 13300:
		return "Keep the settlement economy growing while the expedition advances.\nPopulation, mana and position all shape your next decision."
	default:
		return "This is the opening of a conquest, with much more still to explore.\nBuild, cast powers, lead heroes and progress through the campaign."
	}
}

func (p *NativeShowcasePilot) observeAction(g *NativeGame) {
	if p.seenActions == nil {
		p.seenActions = make(map[string]bool)
	}
	stage := p.player.Stage
	if p.seenActions[stage] {
		return
	}
	caption := ""
	switch stage {
	case "Rallying a strong expedition":
		caption = "Place the magnet near the leader and switch followers to rally mode.\nGather a strong expedition while your towns keep producing."
	case "Leading the expedition towards the opponent":
		caption = "Move the magnet towards an enemy settlement.\nThe gathered followers now advance towards the opposing population."
	case "Engaging the opponent while towns keep producing":
		caption = "Switch the expedition to fight mode.\nFollowers engage the enemy while settlements support the campaign."
	case "Spending earned mana on the opponent":
		caption = "Select an available offensive power and target an enemy town.\nThe spell spends earned mana rather than bypassing the game rules."
	case "Creating Perseus from the rallied leader":
		caption = "Turn the rallied leader into Perseus using an unlocked divine power.\nHeroes provide another way to confront the opposing god."
	}
	if caption != "" {
		p.seenActions[stage] = true
		p.actionCaption, p.actionUntil = caption, g.Updates+600
	}
}
