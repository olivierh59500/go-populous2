package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
	"go-populous2/internal/mobileui"
	"go-populous2/internal/network"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// MobileState owns presentation and gestures only. The same World and network
// commands serve desktop and Android; callbacks never edit the simulation.
type MobileState struct {
	Width, Height      int
	Frame              *image.RGBA
	Image              *ebiten.Image
	View               mobileui.Viewport
	Overlay, TextField string
	FrontPage          int
	Tool               string
	SpritePadding      int
	SceneCache         mobileSceneCache
	Gesture            mobileui.Gesture
	Contacts           []mobileui.Contact
	IDs                []ebiten.TouchID
	Buttons            []mobileButton
	LastScreen         Screen
	World              *engine.World
	NeedsCenter        bool
	Cancel             atomic.Bool
	saveMu             sync.Mutex
	savePath           string
	savePending        bool
}

type mobileButton struct {
	Rect          image.Rectangle
	Action, Label string
	Power         engine.PowerID
	Enabled       bool
}

func NewMobile(assets *Assets) (*Game, error) {
	g, err := New(assets)
	if err != nil {
		return nil, err
	}
	g.mobile = &MobileState{NeedsCenter: true, SpritePadding: mobileSpritePadding(assets)}
	g.mobile.resize(540, 240)
	return g, nil
}

func (m *MobileState) resize(w, h int) {
	if m.Width == w && m.Height == h && m.Frame != nil {
		return
	}
	m.Width, m.Height = w, h
	m.Frame = image.NewRGBA(image.Rect(0, 0, w, h))
	if m.Image != nil {
		m.Image.Deallocate()
	}
	m.Image = ebiten.NewImage(w, h)
	m.View.Rect = image.Rect(0, 24, w, h-44)
	m.Gesture.Cancel()
}

// CancelInput is safe for Android lifecycle callbacks.
func (g *Game) CancelInput() {
	if g != nil && g.mobile != nil {
		g.mobile.Cancel.Store(true)
	}
}
func (g *Game) SetMobileSavePath(path string) {
	if g == nil || g.mobile == nil {
		return
	}
	m := g.mobile
	m.saveMu.Lock()
	m.savePath = path
	m.savePending = true
	m.saveMu.Unlock()
}

func (g *Game) updateMobileFrame() error {
	m := g.mobile
	m.saveMu.Lock()
	if m.savePending {
		g.SavePath = m.savePath
		m.savePending = false
	}
	m.saveMu.Unlock()
	if g.SavePath == "" {
		g.SavePath = filepath.Join(".", "go-populous2.json")
	}
	if m.Cancel.Swap(false) {
		m.Gesture.Cancel()
	}
	g.syncMobileScreen()
	if g.AutoStart && g.World == nil && g.Updates == 125 {
		if err := g.startConquest(); err != nil {
			return err
		}
	}
	g.syncMobileWorld()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.mobileBack()
	}
	g.updateMobileContacts()
	if g.Screen == Playing {
		if err := g.advancePlayingWorld(g.mobileConstructionView(m.View)); err != nil {
			return err
		}
	} else if g.Screen == EndingScreen && g.Ending != nil {
		g.Ending.Update()
	} else if g.Screen == PowerHelpScreen {
		g.updatePowerHelp(0, 0, false)
	}
	// A physics pass may enter results before this input sample is released.
	// Cancel against the resulting screen, not only the screen at frame start.
	g.syncMobileScreen()
	if g.Screen == Playing {
		m.Buttons = g.mobileHUDButtons()
	} else {
		m.Buttons = g.mobileFrontButtons()
	}
	frame := m.Gesture.Update(m.Contacts, g.mobileRegion)
	if frame.Panning && g.Screen == Playing && m.Overlay == "" {
		m.View = m.View.Pan(frame.Pan.X, frame.Pan.Y).Clamp(engine.MapSize, engine.MapSize)
	}
	for _, tap := range frame.Taps {
		if err := g.handleMobileTap(tap); err != nil {
			g.Message, g.messageUntil = err.Error(), g.Updates+120
		}
	}
	g.syncMobileScreen()
	g.drawMobileFrame()
	m.Image.WritePixels(m.Frame.Pix)
	return nil
}

func (g *Game) updateMobileContacts() {
	m := g.mobile
	m.IDs = ebiten.AppendTouchIDs(m.IDs[:0])
	m.Contacts = m.Contacts[:0]
	for _, id := range m.IDs {
		x, y := ebiten.TouchPosition(id)
		m.Contacts = append(m.Contacts, mobileui.Contact{ID: int(id), X: float64(x), Y: float64(y), Started: inpututil.TouchPressDuration(id) == 1})
	}
	if len(m.IDs) == 0 && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		m.Contacts = append(m.Contacts, mobileui.Contact{ID: -1, X: float64(x), Y: float64(y), Started: inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)})
	}
}

func (g *Game) mobileRegion(p mobileui.Point) mobileui.Region {
	if _, ok := mobileButtonAt(g.mobile.Buttons, int(p.X), int(p.Y)); ok {
		return mobileui.RegionUI
	}
	if g.Screen == Playing && g.mobile.Overlay == "" && image.Pt(int(p.X), int(p.Y)).In(g.mobile.View.Rect) {
		return mobileui.RegionTerrain
	}
	if g.Screen == Playing && g.mobile.Overlay == "map" && image.Pt(int(p.X), int(p.Y)).In(image.Rect((g.mobile.Width-128)/2, 32, (g.mobile.Width+128)/2, 160)) {
		return mobileui.RegionTerrain
	}
	if g.Screen == EditorScreen && g.mobile.Overlay == "paint" && image.Pt(int(p.X), int(p.Y)).In(image.Rect((g.mobile.Width-320)/2+104, 69, (g.mobile.Width+320)/2, 202)) {
		return mobileui.RegionTerrain
	}
	return mobileui.RegionNone
}
func mobileButtonAt(buttons []mobileButton, x, y int) (mobileButton, bool) {
	for _, b := range buttons {
		if image.Pt(x, y).In(b.Rect) {
			return b, true
		}
	}
	return mobileButton{}, false
}

func (g *Game) mobileBack() {
	m := g.mobile
	m.Gesture.Cancel()
	if m.TextField != "" {
		_ = g.mobileKeyboardAction("cancel-keyboard")
		return
	}
	if m.Overlay != "" {
		m.Overlay = ""
		return
	}
	if g.Screen != Playing {
		_ = g.handleMobileFront("back")
		m.Gesture.Cancel()
		return
	}
	switch g.Screen {
	case Playing:
		m.Overlay = "menu"
	case PowerHelpScreen:
		g.closePowerHelp()
	case SaveBrowserScreen:
		g.closeSaveBrowser()
	case OptionsScreen:
		g.cancelOptions()
	case EditorScreen:
		g.cancelEditor()
	case DeityProfile:
		g.closeDeityProfile()
	case AboutScreen:
		g.Screen = InGameMenuScreen
	default:
		g.Screen = MainMenu
	}
	m.Gesture.Cancel()
}

var mobileBG = color.RGBA{17, 26, 36, 255}
var mobilePanel = color.RGBA{27, 40, 53, 255}
var mobileInk = color.RGBA{229, 235, 239, 255}
var mobileAccent = color.RGBA{255, 202, 91, 255}

func (g *Game) drawMobileFrame() {
	m := g.mobile
	draw.Draw(m.Frame, m.Frame.Bounds(), image.NewUniform(mobileBG), image.Point{}, draw.Src)
	if g.Screen == Playing && g.World != nil {
		g.drawCachedMobileScene(m.Frame, m.View, m.SpritePadding, &m.SceneCache)
		g.drawMobileHUD(m.Frame)
	} else {
		g.drawMobileFront(m.Frame)
	}
}

func mobileText(dst *image.RGBA, text string, x, y int, ink color.Color) {
	// The fixed bitmap font stays readable at native logical pixels.
	drawer := font.Drawer{Dst: dst, Src: image.NewUniform(ink), Face: basicfont.Face7x13, Dot: fixed.P(x, y+11)}
	drawer.DrawString(text)
}

func drawMobileButton(dst *image.RGBA, b mobileButton) {
	fill, ink := color.Color(color.RGBA{42, 58, 72, 255}), color.Color(mobileInk)
	if !b.Enabled {
		ink = color.RGBA{137, 157, 171, 255}
	}
	draw.Draw(dst, b.Rect, image.NewUniform(fill), image.Point{}, draw.Src)
	label := b.Label
	limit := max(1, (b.Rect.Dx()-14)/7)
	if len(label) > limit {
		label = label[:limit]
	}
	mobileText(dst, label, b.Rect.Min.X+7, b.Rect.Min.Y+(b.Rect.Dy()-13)/2, ink)
}

func (g *Game) drawMobileNativePanel(dst *image.RGBA) {
	g.drawFrame()
	p := image.Pt((dst.Bounds().Dx()-320)/2, 24)
	draw.Draw(dst, image.Rectangle{Min: p, Max: p.Add(image.Pt(320, 200))}, g.framebuffer, image.Point{}, draw.Src)
}

func (g *Game) mobileStatus(dst *image.RGBA) {
	if g.Updates < g.messageUntil || g.Network != nil {
		text := g.Message
		if g.Network != nil {
			text = g.Network.Status().Message
		}
		limit := max(1, (dst.Bounds().Dx()-12)/7)
		if len(text) > limit {
			text = text[:limit]
		}
		mobileText(dst, text, 6, dst.Bounds().Dy()-59, mobileAccent)
	}
}

func (g *Game) mobileCenterSelection() {
	if selectedFollowerExists(g.World, g.SelectedFollower) {
		g.centerOnFollower(g.SelectedFollower)
	}
	g.mobile.NeedsCenter = true
}

func mobileAmount(v int) string {
	if v >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(v)/1000000)
	}
	if v >= 10000 {
		return fmt.Sprintf("%.1fk", float64(v)/1000)
	}
	return fmt.Sprint(v)
}

func (g *Game) applyMobileTerrain(px, py int) error {
	m := g.mobile
	if g.Network != nil && !g.Network.Status().Ready {
		return nil
	}
	if g.Inspecting {
		if id := g.pickMobileFollower(m.View, px, py); id != 0 {
			g.SelectedFollower = id
		}
		return nil
	}
	x, y, ok := g.pickMobileCorner(m.View, px, py)
	if !ok {
		return nil
	}
	target := engine.PowerTarget{X: x, Y: y, Lower: m.Tool == "lower", Direction: g.Direction}
	view := g.mobileConstructionView(m.View)
	if g.Selected == engine.RaiseLower {
		if g.Network != nil {
			kind := "terrain-click"
			if target.Lower {
				kind = "terrain-secondary"
			}
			return g.submitNetwork(network.Command{Kind: kind, Target: target, View: view})
		}
		if target.Lower && g.World.Sprog(g.playerSide(), x, y) {
			return nil
		}
		displayed := g.displayedGame()
		rights := displayed.World.CursorTerrainRights(g.playerSide(), view)
		return g.World.CastWithCursorTerrainRights(g.playerSide(), target, view, rights)
	}
	if g.Network != nil {
		return g.submitNetwork(network.Command{Kind: "power", Power: g.Selected, Target: target})
	}
	return g.World.Cast(g.playerSide(), g.Selected, target)
}

func (g *Game) mobileHUDButtons() []mobileButton {
	m := g.mobile
	w, h := m.Width, m.Height
	buttons := m.Buttons[:0]
	labels := []string{"Raise", "Lower", "Flag", "Look", "Powers", "People", "Map", "Menu"}
	actions := []string{"raise", "lower", "flag", "inspect", "powers", "people", "map", "menu"}
	for i, label := range labels {
		x0 := i * w / len(labels)
		x1 := (i + 1) * w / len(labels)
		buttons = append(buttons, mobileButton{Rect: image.Rect(x0+2, h-42, x1-2, h-2), Label: label, Action: actions[i], Enabled: true})
	}
	if m.Overlay == "" {
		if g.Selected == engine.Basalt || g.Selected == engine.Wind || g.Selected == engine.Tsunami || g.Selected == engine.Earthquake {
			buttons = append(buttons, mobileButton{Rect: image.Rect(w-132, 27, w-68, 59), Action: "direction-prev", Label: "Dir -", Enabled: true}, mobileButton{Rect: image.Rect(w-65, 27, w-3, 59), Action: "direction-next", Label: "Dir +", Enabled: true})
		}
		if g.Selected == engine.Lightning {
			buttons = append(buttons, mobileButton{Rect: image.Rect(w-164, 27, w-84, 59), Action: "lightning-fire", Label: "Fire", Enabled: true}, mobileButton{Rect: image.Rect(w-81, 27, w-3, 59), Action: "lightning-cancel", Label: "Cancel", Enabled: true})
		}
		if g.Inspecting || selectedFollowerExists(g.World, g.SelectedFollower) {
			buttons = append(buttons, mobileButton{Rect: image.Rect(w-93, 63, w-3, 95), Action: "group", Label: "Group", Enabled: true})
		}
		return buttons
	}
	left, right := max(8, (w-480)/2), min(w-8, (w+480)/2)
	add := func(label, action string, x, y, width int, enabled bool) {
		buttons = append(buttons, mobileButton{Rect: image.Rect(x, y, x+width, y+31), Label: label, Action: action, Enabled: enabled})
	}
	switch m.Overlay {
	case "powers":
		for i, label := range []string{"People", "Plants", "Earth", "Air", "Fire", "Water"} {
			cw := (right - left) / 6
			add(label, "element-"+strconv.Itoa(i), left+i*cw, 29, cw-3, true)
		}
		for row := 0; row < 5; row++ {
			id := engine.PowerID(int(g.Category)*6 + row)
			p, ok := engine.PowerByID(id)
			if !ok {
				continue
			}
			cw := (right - left) / 2
			x := left + (row%2)*cw
			y := 64 + (row/2)*34
			label := p.Name + " " + mobileAmount(g.World.PowerCost(g.playerSide(), id))
			add(label, "power-"+strconv.Itoa(int(id)), x, y, cw-3, g.World.Level.Players[g.playerSide()].Powers[id])
		}
		add("Help selected", "power-help", left, 168, (right-left)/2-3, true)
		add("Back", "close", left+(right-left)/2, 168, (right-left)/2-3, true)
	case "people":
		for i, label := range []string{"Settle", "Rally", "Join", "Fight", "Find leader", "Find hero"} {
			cw := (right - left) / 2
			add(label, []string{"mode-settle", "mode-rally", "mode-join", "mode-fight", "leader", "hero"}[i], left+(i%2)*cw, 32+(i/2)*36, cw-3, true)
		}
		add("Back", "close", left, 144, right-left, true)
	case "map":
		add("Return to terrain", "close", left, 168, right-left, true)
	case "group":
		add("Center on group", "group-center", left, 150, (right-left)/2-3, true)
		add("Back", "close", left+(right-left)/2, 150, (right-left)/2-3, true)
	case "menu":
		items := [][2]string{{"Resume", "close"}, {"Pause / resume", "pause"}, {"Options", "options"}, {"Save", "save"}, {"Load", "load"}, {"Spell library", "help"}, {"Bluetooth host", "bt-host"}, {"Bluetooth join", "bt-join"}, {"Leave network", "bt-cancel"}, {"Quit map", "quit"}}
		for i, item := range items {
			cw := (right - left) / 2
			add(item[0], item[1], left+(i%2)*cw, 29+(i/2)*33, cw-3, !strings.HasPrefix(item[1], "bt-") || item[1] == "bt-cancel" || g.BluetoothAvailable())
		}
	}
	return buttons
}

func (g *Game) handleMobileAction(action string) error {
	m := g.mobile
	m.Gesture.Cancel()
	if strings.HasPrefix(action, "element-") {
		n, _ := strconv.Atoi(strings.TrimPrefix(action, "element-"))
		if n >= 0 && n < 6 {
			g.Category = engine.Element(n)
		}
		return nil
	}
	if strings.HasPrefix(action, "power-") && action != "power-help" {
		n, err := strconv.Atoi(strings.TrimPrefix(action, "power-"))
		if err != nil {
			return err
		}
		id := engine.PowerID(n)
		p, ok := engine.PowerByID(id)
		if !ok || !g.World.Level.Players[g.playerSide()].Powers[id] {
			return nil
		}
		g.Selected, g.Category, g.Inspecting = id, p.Element, false
		m.Tool = ""
		m.Overlay = ""
		return nil
	}
	switch action {
	case "raise", "lower":
		g.Selected, g.Category, g.Inspecting = engine.RaiseLower, engine.People, false
		m.Tool = action
		m.Overlay = ""
	case "flag":
		g.Selected, g.Category, g.Inspecting = engine.PapalMagnet, engine.People, false
		m.Tool = ""
		m.Overlay = ""
	case "inspect":
		g.Inspecting = true
		m.Overlay = ""
	case "powers", "people", "map", "menu", "group":
		m.Overlay = action
	case "close":
		m.Overlay = ""
	case "direction-prev":
		g.Direction = (g.Direction + 3) % 4
	case "direction-next":
		g.Direction = (g.Direction + 1) % 4
	case "lightning-fire":
		if g.Network != nil {
			return g.submitNetwork(network.Command{Kind: "lightning-activate"})
		}
		return g.World.ActivateLightning(g.playerSide())
	case "lightning-cancel":
		if g.Network != nil {
			return g.submitNetwork(network.Command{Kind: "lightning-dismiss"})
		}
		g.World.DismissLightning(g.playerSide())
	case "group-center":
		g.mobileCenterSelection()
		m.Overlay = ""
	case "pause":
		if g.Network == nil {
			g.Paused = !g.Paused
		}
	case "options":
		m.Overlay = ""
		return g.openOptions()
	case "save":
		m.Overlay = ""
		return g.openSaveBrowser(true)
	case "load":
		m.Overlay = ""
		return g.openSaveBrowser(false)
	case "help":
		m.Overlay = "spells"
		g.helpReturn = Playing
		g.Screen = HelpScreen
	case "power-help":
		m.Overlay = ""
		return g.openPowerHelp(g.Selected)
	case "quit":
		if g.Network != nil {
			g.Network.Close()
			g.Network = nil
		}
		g.CancelBluetooth()
		m.Overlay = ""
		g.Screen = MainMenu
	case "bt-host":
		m.Overlay = ""
		return g.RequestBluetoothHost(g.LevelIndex)
	case "bt-join":
		g.Paused = true
		return g.RequestBluetoothJoin()
	case "bt-cancel":
		g.CancelBluetooth()
		m.Overlay = ""
	case "leader":
		g.SelectedFollower = g.World.Players[g.playerSide()].Leader
		g.mobileCenterSelection()
		m.Overlay = ""
	case "hero":
		g.heroScanCursor = (g.heroScanCursor + 1) % engine.FollowerCapacity
		for n := 0; n < engine.FollowerCapacity; n++ {
			id := (g.heroScanCursor + n) % engine.FollowerCapacity
			f := g.World.Followers[id]
			if f.State != engine.Inactive && int(f.Owner) == g.playerSide() && f.IsHero() {
				g.heroScanCursor = id
				g.selectFollowerTemporarily(id)
				g.mobileCenterSelection()
				break
			}
		}
		m.Overlay = ""
	case "mode-settle", "mode-rally", "mode-join", "mode-fight":
		mode := map[string]engine.Mode{"mode-settle": engine.Settle, "mode-rally": engine.Rally, "mode-join": engine.Join, "mode-fight": engine.Fight}[action]
		if g.Network != nil {
			return g.submitNetwork(network.Command{Kind: "mode", Mode: mode})
		}
		g.World.SetMode(g.playerSide(), mode)
		m.Overlay = ""
	}
	return nil
}

func (g *Game) drawMobileHUD(dst *image.RGBA) {
	m := g.mobile
	draw.Draw(dst, image.Rect(0, 0, m.Width, 24), image.NewUniform(mobileBG), image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(0, m.Height-44, m.Width, m.Height), image.NewUniform(mobileBG), image.Point{}, draw.Src)
	s := g.World.Summaries()[g.playerSide()]
	mobileText(dst, "MANA "+mobileAmount(s.Mana), 6, 5, mobileInk)
	mobileText(dst, "PEOPLE "+mobileAmount(s.Population), 130, 5, mobileInk)
	label := "Inspect"
	if !g.Inspecting {
		if p, ok := engine.PowerByID(g.Selected); ok {
			label = p.Name
		}
		if m.Tool == "lower" {
			label = "Lower / release town"
		}
	}
	mobileText(dst, label, max(265, m.Width-7*len(label)-8), 5, mobileAccent)
	if m.Overlay != "" {
		draw.Draw(dst, image.Rect(0, 24, m.Width, m.Height-44), image.NewUniform(mobilePanel), image.Point{}, draw.Src)
	}
	for _, b := range m.Buttons {
		drawMobileButton(dst, b)
	}
	if m.Overlay == "map" {
		g.drawMobileOverview(dst)
	}
	if m.Overlay == "group" {
		g.drawMobileGroup(dst)
	}
	if m.Overlay == "" && (g.Selected == engine.Basalt || g.Selected == engine.Wind || g.Selected == engine.Tsunami || g.Selected == engine.Earthquake) {
		mobileText(dst, []string{"North", "East", "South", "West"}[g.Direction%4], m.Width-128, 99, mobileAccent)
	}
	g.mobileStatus(dst)
}

func (g *Game) drawMobileGroup(dst *image.RGBA) {
	if !selectedFollowerExists(g.World, g.SelectedFollower) {
		mobileText(dst, "Tap a group or town with Look.", 20, 50, mobileInk)
		return
	}
	view := g.displayedGame()
	id := g.SelectedFollower
	f := g.World.Followers[id]
	view.SelectedFollower = id
	view.framebuffer = image.NewRGBA(image.Rect(0, 0, 320, 200))
	view.music = nil
	view.drawSelectionPanel()
	if panel := g.Assets.SelectionPanel; panel != nil {
		r := panel.HitRect()
		target := image.Pt((dst.Bounds().Dx()-r.Dx())/2, 35)
		draw.Draw(dst, image.Rectangle{Min: target, Max: target.Add(r.Size())}, view.framebuffer, r.Min, draw.Over)
	}
	mobileText(dst, fmt.Sprintf("People %d  Weapons %d", f.Population, f.Weapons), 20, 96, mobileInk)
	mobileText(dst, fmt.Sprintf("Position %d, %d  Town stage %d", f.X, f.Y, f.Stage), 20, 115, mobileInk)
}

func (g *Game) drawMobileOverview(dst *image.RGBA) {
	m := g.mobile
	scale := 2
	ox := (m.Width - 128) / 2
	oy := 32
	w := g.displayedGame().World
	for y := 0; y < engine.MapSize; y++ {
		for x := 0; x < engine.MapSize; x++ {
			cell := w.Cell(x, y)
			c := g.Assets.Visual.Palettes[w.Level.Landscape][min(15, int(cell.BaseAltitude)+2)]
			if cell.IsWater() {
				c = color.RGBA{20, 92, 132, 255}
			}
			if id := w.Occupants[x+y*engine.MapSize]; id > 0 {
				if w.Followers[id].Owner == 0 {
					c = color.RGBA{90, 180, 255, 255}
				} else {
					c = color.RGBA{255, 100, 75, 255}
				}
			}
			draw.Draw(dst, image.Rect(ox+x*scale, oy+y*scale, ox+(x+1)*scale, oy+(y+1)*scale), image.NewUniform(c), image.Point{}, draw.Src)
		}
	}
}
