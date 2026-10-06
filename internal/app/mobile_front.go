package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"

	"go-populous2/internal/engine"
)

var mobileElementNames = [...]string{"People", "Plants", "Earth", "Air", "Fire", "Water"}
var mobileRuleNames = [...]string{"Build anywhere", "Sea level only", "Protect enemy terrain", "Forbid raising", "Forbid lowering", "Fatal water", "Hide enemy", "Disable emigration", "Hide disasters", "Shallow swamps"}

// mobileFrontButtons shares all drawing and hit geometry. The frontend only
// translates actions; the existing screen helpers retain the game rules.
func (g *Game) mobileFrontButtons() []mobileButton {
	if g.mobile == nil {
		return nil
	}
	m := g.mobile
	w, h := m.Width, m.Height
	panelWidth := min(w-20, 640)
	x := (w - panelWidth) / 2
	var buttons []mobileButton
	add := func(rect image.Rectangle, action, label string, enabled bool) {
		buttons = append(buttons, mobileButton{Rect: rect, Action: action, Label: label, Enabled: enabled})
	}
	grid := func(col, row, cols int, action, label string, enabled bool) {
		bw := (panelWidth - 6*(cols-1)) / cols
		left, top := x+col*(bw+6), 50+row*36
		add(image.Rect(left, top, left+bw, top+32), action, label, enabled)
	}
	footer := func(col, cols int, action, label string, enabled bool) {
		bw := (panelWidth - 6*(cols-1)) / cols
		left := x + col*(bw+6)
		add(image.Rect(left, h-38, left+bw, h-6), action, label, enabled)
	}
	if m.TextField != "" {
		for row, keys := range []string{"1234567890", "QWERTYUIOP", "ASDFGHJKL-", "ZXCVBNM.:_"} {
			for col, key := range keys {
				bw := (panelWidth - 4*9) / 10
				left, top := x+col*(bw+4), 50+row*34
				add(image.Rect(left, top, left+bw, top+30), "key-"+string(key), string(key), true)
			}
		}
		for i, a := range []struct{ action, label string }{{"cancel-keyboard", "Cancel"}, {"backspace", "Delete"}, {"clear", "Clear"}, {"key- ", "Space"}, {"accept", "Accept"}} {
			footer(i, 5, a.action, a.label, true)
		}
		return buttons
	}
	if m.Overlay == "native" {
		footer(0, 1, "close-details", "Back to touch controls", true)
		return buttons
	}
	if m.Overlay == "paint" && g.Screen == EditorScreen {
		footer(0, 1, "close-details", "Back to editor tools", true)
		return buttons
	}
	switch g.Screen {
	case MainMenu:
		choices := []struct{ action, label string }{{"conquest", "Conquest"}, {"profile", "Your deity"}, {"load", "Load game"}, {"custom", "Custom game"}, {"options", "Game options"}, {"editor", "Map editor"}, {"bluetooth", "Bluetooth"}, {"help", "How to play"}, {"resume", "Resume game"}}
		for i, c := range choices {
			enabled := c.action != "resume" || g.World != nil
			if c.action == "bluetooth" {
				enabled = g.BluetoothAvailable()
			}
			grid(i%3, i/3, 3, c.action, c.label, enabled)
		}
	case ConquestBriefing:
		for i, c := range []struct{ action, label string }{{"world-minus-10", "-10 worlds"}, {"world-minus-1", "Previous"}, {"world-plus-1", "Next"}, {"world-plus-10", "+10 worlds"}} {
			grid(i, 0, 4, c.action, c.label, !g.CustomGame)
		}
		grid(0, 1, 3, "world-code", "World code", true)
		grid(1, 1, 3, "opponent", "Opponent", true)
		grid(2, 1, 3, "spell-library", "Available powers", true)
		grid(0, 2, 2, "details", "Original details", true)
		grid(1, 2, 2, "profile", "Your deity", true)
		footer(0, 2, "back", "Back", true)
		footer(1, 2, "proceed", "Start conquest", true)
	case DeityProfile:
		grid(0, 0, 2, "profile-name", "Name: "+g.Profile.Name, true)
		grid(1, 0, 2, "profile-code", "Enter password", true)
		if m.FrontPage%2 == 0 {
			for i, name := range mobileElementNames {
				grid(i%2, 1+i/2, 2, fmt.Sprintf("experience-%d", i), fmt.Sprintf("%s: %d +", name, g.Profile.Experience[i]), g.Profile.Bolts > 0 && g.Profile.Experience[i] < 255)
			}
		} else {
			for part, name := range []string{"Head", "Eyes", "Mouth"} {
				grid(0, part+1, 2, fmt.Sprintf("face-%d-prev", part), name+" -", true)
				grid(1, part+1, 2, fmt.Sprintf("face-%d-next", part), name+" +", true)
			}
		}
		footer(0, 3, "back", "Done", true)
		footer(1, 3, "front-next", "Experience / face", true)
		footer(2, 3, "details", "Portrait / password", true)
	case OptionsScreen:
		if g.Options == nil {
			break
		}
		if m.FrontPage%2 == 0 {
			for i := 0; i < 6; i++ {
				grid(i%2, i/2, 2, fmt.Sprintf("rule-%d", i), mobileToggleLabel(mobileRuleNames[i], g.Options.rule(i)), !g.Options.RulesLocked)
			}
			grid(0, 3, 3, "option-side", fmt.Sprintf("Player %d", g.Options.Owner+1), true)
			grid(1, 3, 3, "reaction-decrease", "Reaction -", !g.Options.RulesLocked)
			grid(2, 3, 3, "reaction-increase", "Reaction +", !g.Options.RulesLocked)
		} else {
			for i := 6; i < 10; i++ {
				grid((i-6)%2, (i-6)/2, 2, fmt.Sprintf("rule-%d", i), mobileToggleLabel(mobileRuleNames[i], g.Options.rule(i)), !g.Options.RulesLocked)
			}
			grid(0, 2, 2, "music", mobileToggleLabel("Music", g.Options.Music), true)
			grid(1, 2, 2, "sound", mobileToggleLabel("Sound", g.Options.Sound), true)
			grid(0, 3, 1, "special-code", "Special code", true)
		}
		footer(0, 3, "back", "Cancel", true)
		footer(1, 3, "front-next", "More options", true)
		footer(2, 3, "proceed", "Apply", true)
	case SaveBrowserScreen:
		b := g.SaveBrowser
		if b == nil {
			break
		}
		if b.Confirm {
			grid(0, 1, 2, "file-cancel", "Keep existing file", true)
			grid(1, 1, 2, "file-replace", "Replace save", true)
		} else if b.Error != "" {
			grid(0, 2, 1, "file-dismiss", "Dismiss error", true)
		} else {
			for row := 0; row < 3; row++ {
				for col := 0; col < 2; col++ {
					i := row*2 + col
					if b.Offset+i < len(b.Files) {
						grid(col, row, 2, fmt.Sprintf("file-row-%d", i), b.Files[b.Offset+i], true)
					}
				}
			}
			grid(0, 3, 3, "file-page-up", "Previous files", b.Offset > 0)
			grid(1, 3, 3, "save-name", "Filename", true)
			grid(2, 3, 3, "file-page-down", "More files", b.Offset+6 < len(b.Files))
		}
		footer(0, 2, "back", "Back", true)
		verb := "Load"
		if b.Saving {
			verb = "Save"
		}
		footer(1, 2, "file-submit", verb, !b.Confirm && b.Error == "")
	case NetworkSetup:
		grid(0, 0, 4, "world-minus-10", "-10 worlds", true)
		grid(1, 0, 4, "world-minus-1", "Previous", true)
		grid(2, 0, 4, "world-plus-1", "Next", true)
		grid(3, 0, 4, "world-plus-10", "+10 worlds", true)
		grid(0, 1, 2, "bluetooth-host", "Host Bluetooth game", g.BluetoothAvailable() && g.Network == nil)
		grid(1, 1, 2, "bluetooth-join", "Join Bluetooth game", g.BluetoothAvailable() && g.Network == nil)
		grid(0, 2, 3, "tcp-mode", "TCP: "+mobileHostLabel(g.networkHosting), g.Network == nil)
		grid(1, 2, 3, "network-address", "TCP address", g.Network == nil)
		grid(2, 2, 3, "tcp-connect", "Connect TCP", g.Network == nil)
		grid(0, 3, 1, "bluetooth-cancel", "Cancel connection", true)
		footer(0, 1, "back", "Back", true)
	case InGameMenuScreen:
		for i, c := range []struct{ action, label string }{{"resume", "Resume"}, {"save", "Save game"}, {"load", "Load game"}, {"options", "Options"}, {"assist", "Computer assist"}, {"opponent-control", "Human / computer"}, {"restart", "Restart map"}, {"quit-map", "Main menu"}, {"about", "About"}} {
			enabled := g.Network == nil || c.action == "resume" || c.action == "quit-map" || c.action == "about"
			grid(i%3, i/3, 3, c.action, c.label, enabled)
		}
		grid(0, 3, 2, "editor", "Map editor", g.CustomGame && g.Network == nil)
		grid(1, 3, 2, "profile", "Switch deity", g.CustomGame && g.Network == nil)
	case HelpScreen:
		if m.Overlay == "spells" {
			first := (m.FrontPage % 3) * 12
			for i := first; i < min(first+12, len(engine.Powers)); i++ {
				p := engine.Powers[i]
				enabled := true
				if g.helpReturn == ConquestBriefing {
					enabled = g.briefingLevel().Players[g.playerSide()].Powers[p.ID]
				}
				grid((i-first)%3, (i-first)/3, 3, fmt.Sprintf("preview-%d", p.ID), p.Name, enabled)
			}
			footer(0, 2, "back", "Back", true)
			footer(1, 2, "front-next", "More powers", true)
		} else {
			footer(0, 2, "back", "Back", true)
			footer(1, 2, "spell-library", "All power previews", true)
		}
	case PowerHelpScreen:
		footer(0, 1, "back", "Back", true)
	case CampaignResult:
		footer(0, 2, "details", "Original results", true)
		footer(1, 2, "proceed", "Continue conquest", g.Updates-g.resultAt >= 101)
	case EndingScreen:
		footer(0, 1, "back", "Main menu", true)
	case AboutScreen:
		footer(0, 1, "back", "Back", true)
	case EditorScreen:
		if g.Editor == nil {
			break
		}
		if m.FrontPage%2 == 0 {
			for i, label := range editorLabels {
				grid(i%4, i/4, 4, fmt.Sprintf("editor-tool-%d", i), label, true)
			}
			grid(0, 2, 2, "editor-landscape", "Landscape", true)
			grid(1, 2, 2, "editor-new-map", "New map", true)
			grid(0, 3, 2, "editor-height-minus", "Height -", true)
			grid(1, 3, 2, "editor-height-plus", "Height +", true)
		} else {
			for i, field := range []string{"time", "x", "y", "effect"} {
				grid(i%2, i/2, 2, "editor-"+field, "Event "+field, true)
			}
			grid(0, 2, 2, "editor-previous-event", "Previous event", true)
			grid(1, 2, 2, "editor-next-event", "Next event", true)
			grid(0, 3, 2, "editor-local-mana-add", "Your mana +", true)
			grid(1, 3, 2, "editor-opponent-mana-add", "Enemy mana +", true)
		}
		footer(0, 4, "back", "Cancel", true)
		footer(1, 4, "front-next", "More tools", true)
		footer(2, 4, "paint-map", "Paint map", true)
		footer(3, 4, "proceed", "Apply", true)
	}
	return buttons
}

func mobileToggleLabel(label string, checked bool) string {
	if checked {
		return "[x] " + label
	}
	return "[ ] " + label
}

func mobileHostLabel(host bool) string {
	if host {
		return "Host"
	}
	return "Join"
}

func (g *Game) handleMobileFront(action string) error {
	if g.mobile == nil || action == "" {
		return nil
	}
	m := g.mobile
	previousScreen := g.Screen
	defer func() {
		if g.Screen != previousScreen {
			m.FrontPage = 0
			if m.Overlay != "spells" || g.Screen != HelpScreen && g.Screen != PowerHelpScreen {
				m.Overlay = ""
			}
		}
	}()
	if m.TextField != "" {
		return g.mobileKeyboardAction(action)
	}
	if action == "front-next" {
		m.FrontPage++
		return nil
	}
	if action == "details" || action == "opponent" {
		m.Overlay = "native"
		if action == "opponent" {
			g.briefingOpponent = true
		}
		return nil
	}
	if action == "paint-map" && g.Screen == EditorScreen {
		m.Overlay = "paint"
		return nil
	}
	if action == "close-details" {
		m.Overlay = ""
		g.briefingOpponent = false
		return nil
	}
	if strings.HasPrefix(action, "world-") && action != "world-code" {
		parts := strings.Split(action, "-")
		if len(parts) == 3 {
			delta, err := strconv.Atoi(parts[2])
			if err == nil && len(g.Assets.Levels) != 0 {
				if parts[1] == "minus" {
					delta = -delta
				}
				g.LevelIndex = min(max(0, g.LevelIndex+delta), len(g.Assets.Levels)-1)
			}
		}
		return nil
	}
	if action == "back" {
		return g.mobileFrontBack()
	}
	switch g.Screen {
	case MainMenu:
		switch action {
		case "conquest":
			g.CustomGame, g.Screen = false, ConquestBriefing
		case "profile":
			g.openDeityProfile(MainMenu)
		case "custom":
			return g.startCustomGame()
		case "options":
			return g.openOptions()
		case "editor":
			return g.openEditor()
		case "load":
			return g.openSaveBrowser(false)
		case "bluetooth":
			g.openNetworkSetup()
		case "help":
			g.helpReturn, g.Screen = MainMenu, HelpScreen
		case "resume":
			if g.World != nil {
				g.Screen = Playing
			}
		}
	case ConquestBriefing:
		switch action {
		case "profile":
			g.openDeityProfile(ConquestBriefing)
		case "spell-library":
			g.helpReturn, g.Screen = ConquestBriefing, HelpScreen
			m.Overlay, m.FrontPage = "spells", 0
		case "world-code":
			_ = g.handleBriefingAction(action)
			m.TextField = "world-code"
		default:
			return g.handleBriefingAction(action)
		}
	case DeityProfile:
		switch action {
		case "profile-name":
			g.beginProfileName()
			m.TextField = action
		case "profile-code":
			g.beginProfileCode()
			m.TextField = action
		default:
			g.applyOriginalProfileAction(action)
		}
	case OptionsScreen:
		if g.Options == nil {
			return fmt.Errorf("options draft missing")
		}
		switch action {
		case "music":
			g.Options.Music = !g.Options.Music
		case "sound":
			g.Options.Sound = !g.Options.Sound
		case "special-code":
			_ = g.handleOptionsAction("special-codes")
			m.TextField = action
		case "option-side":
			return g.handleOptionsAction("side")
		default:
			return g.handleOptionsAction(action)
		}
	case SaveBrowserScreen:
		b := g.SaveBrowser
		if b == nil {
			return nil
		}
		switch action {
		case "save-name":
			m.TextField = action
		case "file-page-up":
			b.Offset = max(0, b.Offset-6)
		case "file-page-down":
			b.Offset = min(max(0, len(b.Files)-6), b.Offset+6)
		default:
			if strings.HasPrefix(action, "file-row-") {
				g.handleFileAction("file-" + strings.TrimPrefix(action, "file-row-"))
			} else {
				g.handleFileAction(strings.TrimPrefix(action, "file-"))
			}
		}
	case NetworkSetup:
		switch action {
		case "bluetooth-host":
			return g.RequestBluetoothHost(g.LevelIndex)
		case "bluetooth-join":
			return g.RequestBluetoothJoin()
		case "bluetooth-cancel":
			g.CancelBluetooth()
		case "tcp-mode":
			g.networkHosting = !g.networkHosting
		case "network-address":
			m.TextField, g.editingConnection = action, true
		case "tcp-connect":
			return g.startConfiguredNetwork()
		}
	case InGameMenuScreen:
		return g.applyInGameAction(action)
	case HelpScreen:
		if action == "spell-library" {
			m.Overlay, m.FrontPage = "spells", 0
		} else if strings.HasPrefix(action, "preview-") {
			value, err := strconv.Atoi(strings.TrimPrefix(action, "preview-"))
			if err != nil || value < 0 || value >= 36 {
				return fmt.Errorf("invalid preview power")
			}
			return g.openPowerHelp(engine.PowerID(value))
		}
	case CampaignResult:
		if action == "proceed" && g.Updates-g.resultAt >= 101 {
			return g.applyCampaignResult()
		}
	case EditorScreen:
		if g.Editor == nil {
			return fmt.Errorf("editor draft missing")
		}
		if action == "proceed" {
			return g.applyEditor()
		}
		if strings.HasPrefix(action, "editor-tool-") {
			index, err := strconv.Atoi(strings.TrimPrefix(action, "editor-tool-"))
			if err == nil && index >= 0 && index < len(editorLabels) {
				g.Editor.Tool = EditorTool(index)
			}
			return nil
		}
		if action == "editor-height-minus" || action == "editor-height-plus" {
			delta := 1
			if action == "editor-height-minus" {
				delta = -1
			}
			g.Editor.Height = min(8, max(0, g.Editor.Height+delta))
			return nil
		}
		field := strings.TrimPrefix(action, "editor-")
		if err := g.applyEditorAction(field); err != nil {
			return err
		}
		if g.Editor.EditingField != "" {
			m.TextField = "editor-number"
		}
	}
	return nil
}

func (g *Game) mobileFrontBack() error {
	if g.mobile != nil {
		g.mobile.TextField, g.mobile.Overlay = "", ""
		g.mobile.FrontPage = 0
	}
	switch g.Screen {
	case DeityProfile:
		g.closeDeityProfile()
	case ConquestBriefing:
		return g.handleBriefingAction("cancel")
	case OptionsScreen:
		g.cancelOptions()
	case SaveBrowserScreen:
		if g.SaveBrowser != nil && g.SaveBrowser.Confirm {
			g.handleFileAction("cancel")
		} else {
			g.closeSaveBrowser()
		}
	case NetworkSetup:
		g.CancelBluetooth()
		g.Screen = MainMenu
	case InGameMenuScreen:
		return g.applyInGameAction("resume")
	case HelpScreen:
		g.Screen = g.helpReturn
	case PowerHelpScreen:
		g.closePowerHelp()
	case AboutScreen:
		g.Screen = InGameMenuScreen
	case EditorScreen:
		g.cancelEditor()
	case EndingScreen:
		g.Screen = MainMenu
	default:
		g.Screen = MainMenu
	}
	return nil
}

func (g *Game) mobileTextValue() string {
	switch g.mobile.TextField {
	case "profile-name":
		return g.Profile.Name
	case "profile-code":
		return g.profileCodeInput
	case "world-code":
		return g.worldCodeInput
	case "special-code":
		if g.Options != nil {
			return g.Options.SpecialCode
		}
	case "save-name":
		if g.SaveBrowser != nil {
			return g.SaveBrowser.Name
		}
	case "network-address":
		return g.connectionAddress
	case "editor-number":
		if g.Editor != nil {
			return g.Editor.NumberInput
		}
	}
	return ""
}

func (g *Game) mobileKeyboardAction(action string) error {
	m := g.mobile
	if action == "cancel-keyboard" {
		g.editingProfileCode, g.editingProfileName, g.editingWorldCode = false, false, false
		if g.Options != nil {
			g.Options.EditingSpecialCode = false
		}
		if g.Editor != nil {
			g.Editor.EditingField, g.Editor.NumberInput = "", ""
		}
		m.TextField = ""
		return nil
	}
	if action == "accept" {
		switch m.TextField {
		case "profile-name", "profile-code":
			g.applyProfileInput(profileInput{Enter: true})
		case "world-code":
			if !g.acceptBriefingCode() {
				return fmt.Errorf("unknown world code")
			}
		case "special-code":
			if g.Options != nil {
				g.Options.acceptSpecialCode()
			}
		case "editor-number":
			if g.Editor != nil {
				if err := g.Editor.applyNumber(); err != nil {
					return err
				}
			}
		}
		m.TextField = ""
		return nil
	}
	var chars []rune
	if strings.HasPrefix(action, "key-") {
		chars = []rune(strings.TrimPrefix(action, "key-"))
		if len(chars) != 1 {
			return nil
		}
	}
	backspace, clear := action == "backspace", action == "clear"
	if m.TextField == "profile-name" || m.TextField == "profile-code" {
		if clear {
			if m.TextField == "profile-name" {
				g.Profile.Name = ""
			} else {
				g.profileCodeInput = ""
			}
			g.profileCaret = 0
		}
		g.applyProfileInput(profileInput{Characters: chars, Backspace: backspace})
		return nil
	}
	if m.TextField == "network-address" {
		g.applyNetworkSetupInput(networkSetupInput{Characters: chars, Backspace: backspace, Clear: clear})
		return nil
	}
	var value *string
	limit := 0
	allowed := func(r rune) bool { return r >= 32 && r <= 126 }
	switch m.TextField {
	case "world-code":
		value, limit = &g.worldCodeInput, 10
		allowed = func(r rune) bool { return r >= 'A' && r <= 'Z' }
	case "special-code":
		if g.Options != nil {
			value, limit = &g.Options.SpecialCode, 17
		}
	case "save-name":
		if g.SaveBrowser != nil {
			value, limit = &g.SaveBrowser.Name, 96
			g.SaveBrowser.Confirm, g.SaveBrowser.Error = false, ""
			allowed = func(r rune) bool { return r >= 32 && r <= 126 && !strings.ContainsRune(`/\:*?"<>|`, r) }
		}
	case "editor-number":
		if g.Editor != nil {
			value, limit = &g.Editor.NumberInput, 10
			allowed = func(r rune) bool { return r >= '0' && r <= '9' }
		}
	}
	if value == nil {
		return nil
	}
	if clear {
		*value = ""
	}
	if backspace && len(*value) > 0 {
		*value = (*value)[:len(*value)-1]
	}
	for _, r := range chars {
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		if allowed(r) && len(*value) < limit {
			*value += string(r)
		}
	}
	return nil
}

func (g *Game) drawMobileFront(dst *image.RGBA) {
	if g.mobile == nil {
		return
	}
	bg, ink := color.RGBA{20, 29, 36, 255}, color.RGBA{232, 201, 134, 255}
	draw.Draw(dst, dst.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	if g.mobile.Overlay == "native" || g.mobile.Overlay == "paint" || g.Screen == PowerHelpScreen || g.Screen == AboutScreen || g.Screen == EndingScreen {
		g.drawMobileNativePanel(dst)
	} else {
		title, subtitle := g.mobileFrontHeading()
		mobileText(dst, title, 12, 10, ink)
		if g.mobile.TextField != "" {
			subtitle = g.mobileTextValue() + "_"
		}
		maxChars := max(1, (dst.Bounds().Dx()-24)/8)
		if len(subtitle) > maxChars {
			subtitle = subtitle[len(subtitle)-maxChars:]
		}
		mobileText(dst, subtitle, 12, 29, color.RGBA{173, 196, 198, 255})
		if g.Screen == HelpScreen && g.mobile.Overlay != "spells" && g.mobile.TextField == "" {
			for i, line := range []string{"Tap land to raise or lower it.", "Use two fingers to move the view.", "Select an element, then a power.", "Choose settle, rally, join or fight.", "Inspect a follower to see its group.", "Lightning: place, then activate.", "Open the menu to save or change options."} {
				mobileText(dst, line, 12, 56+i*18, color.RGBA{198, 211, 215, 255})
			}
		}
		if g.Screen == CampaignResult && g.World != nil {
			verdict := "The opposing deity won."
			if g.World.Result == g.playerSide()+1 {
				verdict = "Your followers are victorious!"
			}
			mobileText(dst, verdict, 12, 66, ink)
			mobileText(dst, fmt.Sprintf("Score: %d", g.ResultScore.Value), 12, 91, ink)
			mobileText(dst, g.ResultScoreError, 12, 116, ink)
		}
	}
	for _, button := range g.mobileFrontButtons() {
		drawMobileButton(dst, button)
	}
	if g.Updates < g.messageUntil && g.Message != "" && g.mobile.TextField == "" {
		maxChars := max(1, (dst.Bounds().Dx()-24)/8)
		message := g.Message[:min(len(g.Message), maxChars)]
		mobileText(dst, message, 12, dst.Bounds().Dy()-51, color.RGBA{244, 184, 115, 255})
	}
}

func (g *Game) mobileFrontHeading() (string, string) {
	if g.mobile.TextField != "" {
		return "ENTER " + strings.ToUpper(strings.ReplaceAll(g.mobile.TextField, "-", " ")), ""
	}
	switch g.Screen {
	case MainMenu:
		return "POPULOUS II - GO", "Trials of the Olympian Gods"
	case ConquestBriefing, NetworkSetup:
		level := g.briefingLevel()
		land := [4]string{"Fertile", "Winter", "Barren", "Sludge"}[min(max(level.Landscape, 0), 3)]
		title := "CONQUEST"
		if g.Screen == NetworkSetup {
			title = "TWO PLAYERS"
		}
		return title, fmt.Sprintf("World %d - %s - %s", g.LevelIndex, level.Code, land)
	case DeityProfile:
		return "YOUR DEITY", fmt.Sprintf("%s - %d experience bolts available", g.Profile.Name, g.Profile.Bolts)
	case OptionsScreen:
		if g.Options != nil {
			status := "Custom rules"
			if g.Options.RulesLocked {
				status = "Campaign rules are read-only"
			}
			return "GAME OPTIONS", fmt.Sprintf("%s - reaction %d", status, g.Options.Draft.Players[g.Options.Owner].ReactionDelay)
		}
	case SaveBrowserScreen:
		if g.SaveBrowser != nil {
			b := g.SaveBrowser
			title := "LOAD GAME"
			if b.Saving {
				title = "SAVE GAME"
			}
			if b.Error != "" {
				return title, b.Error
			}
			return title, b.Name
		}
	case InGameMenuScreen:
		side, assist, matchup := g.inGameControlLabels()
		return "GAME MENU", fmt.Sprintf("%s - assist %s - %s", side, assist, matchup)
	case HelpScreen:
		if g.mobile.Overlay == "spells" {
			return "POWER PREVIEWS", "Original animated demonstrations"
		}
		return "HOW TO PLAY", "Touch controls"
	case CampaignResult:
		return "CONQUEST RESULT", "Your campaign progress"
	case EditorScreen:
		if g.Editor != nil {
			return "MAP EDITOR", fmt.Sprintf("Tool %s - height %d - event %d", editorLabels[g.Editor.Tool], g.Editor.Height, g.Editor.EventIndex+1)
		}
	}
	return "POPULOUS II", ""
}
