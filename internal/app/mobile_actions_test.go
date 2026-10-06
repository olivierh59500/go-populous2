package app

import (
	"image"
	"image/color"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/mobileui"
	"go-populous2/internal/visualassets"
)

func mobileActionTestGame() *Game {
	g := mobileSceneTestGame()
	g.mobile = &MobileState{Width: 540, Height: 240, Frame: image.NewRGBA(image.Rect(0, 0, 540, 240)), View: mobileui.Viewport{Rect: image.Rect(0, 24, 540, 196), CenterX: 32, CenterY: 32}, SpritePadding: 96}
	g.World.Players[0].Mana = 1000000
	for _, p := range engine.Powers {
		g.World.Level.Players[0].Powers[p.ID] = true
	}
	return g
}

func TestMobileDirectionButtonsWrapAndReachTheCastController(t *testing.T) {
	for _, power := range []engine.PowerID{engine.Basalt, engine.Wind, engine.Tsunami, engine.Earthquake} {
		g := mobileActionTestGame()
		g.Selected = power
		found := map[string]bool{}
		for _, b := range g.mobileHUDButtons() {
			if b.Action == "direction-prev" || b.Action == "direction-next" {
				p := image.Pt(b.Rect.Min.X+b.Rect.Dx()/2, b.Rect.Min.Y+b.Rect.Dy()/2)
				g.mobile.Buttons = g.mobileHUDButtons()
				if g.mobileRegion(mobileui.Point{X: float64(p.X), Y: float64(p.Y)}) != mobileui.RegionUI {
					t.Fatal("direction button fell through to terrain", power, b.Action)
				}
				found[b.Action] = true
			}
		}
		if !found["direction-prev"] || !found["direction-next"] {
			t.Fatal("directional power omitted its controls", power)
		}
	}
	g := mobileActionTestGame()
	g.Selected = engine.Wind
	if err := g.handleMobileAction("direction-prev"); err != nil || g.Direction != 3 {
		t.Fatal("previous direction did not wrap north to west", err, g.Direction)
	}
	if err := g.handleMobileAction("direction-next"); err != nil || g.Direction != 0 {
		t.Fatal("next direction did not wrap west to north", err, g.Direction)
	}
	_ = g.handleMobileAction("direction-next")
	px, py := g.mobile.View.Project(32, 32, 1)
	before := g.World.Players[0].Mana
	if err := g.applyMobileTerrain(int(px), int(py)); err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, wind := range g.World.Wind {
		if wind.Active {
			active++
			if wind.Direction != 1 {
				t.Fatal("touch casting discarded the selected east direction", wind)
			}
		}
	}
	if active != 1 || g.World.Players[0].Mana != before-g.World.PowerCost(0, engine.Wind) {
		t.Fatal("touch cast did not create exactly one admitted wind controller", active)
	}
}

func TestMobileLightningPlacesThenFiresAndCancelsWithoutExtraDebit(t *testing.T) {
	g := mobileActionTestGame()
	g.Selected = engine.Lightning
	g.mobile.Buttons = g.mobileHUDButtons()
	buttons := map[string]bool{}
	for _, b := range g.mobile.Buttons {
		buttons[b.Action] = true
	}
	if !buttons["lightning-fire"] || !buttons["lightning-cancel"] {
		t.Fatal("lightning has no accessible fire/cancel controls")
	}
	px, py := g.mobile.View.Project(32, 32, 1)
	initial := g.World.Players[0].Mana
	if err := g.applyMobileTerrain(int(px), int(py)); err != nil {
		t.Fatal(err)
	}
	marker := g.World.Air.MarkerSlots[0] - 1
	if marker < 0 || !g.World.Air.Markers[marker].Active || g.World.Players[0].Mana != initial {
		t.Fatal("placing a lightning target fired or charged immediately")
	}
	if err := g.handleMobileAction("lightning-fire"); err != nil {
		t.Fatal(err)
	}
	bolts := 0
	for _, bolt := range g.World.Air.Bolts {
		if bolt.Active {
			bolts++
		}
	}
	if bolts < 1 || g.World.Players[0].Mana != initial-g.World.PowerCost(0, engine.Lightning) {
		t.Fatal("fire did not activate the targeted volley at the ordinary price", bolts)
	}
	afterFire := g.World.Players[0].Mana
	if err := g.handleMobileAction("lightning-cancel"); err != nil {
		t.Fatal(err)
	}
	if g.World.Air.MarkerSlots[0] != 0 || g.World.Air.Markers[marker].Phase != engine.LightningDisappearing || g.World.Players[0].Mana != afterFire {
		t.Fatal("cancel did not retire the target without another mana debit")
	}
	for _, bolt := range g.World.Air.Bolts {
		if bolt.Active {
			t.Fatal("cancel left a live lightning bolt")
		}
	}
}

func TestMobileFrameCacheReplacesNetworkWorldInsideTheSameMainCycle(t *testing.T) {
	g := mobileActionTestGame()
	g.Assets.Visual.Animations["magnet/0"] = visualassets.Animation{Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 0}}}}}
	g.World.Magnets[0] = engine.MagnetActor{X: 32*256 + 128, Y: 32*256 + 128}
	g.World.Actors.Link(engine.ActorRef{Kind: engine.ActorMagnet}, g.World.Magnets[0].X, g.World.Magnets[0].Y)
	g.mobile.Buttons = g.mobileHUDButtons()
	g.drawMobileFrame()
	if g.mobile.SceneCache.Image.Bounds() != g.mobile.View.Rect {
		t.Fatal("cache includes toolbar pixels or loses the absolute map coordinates")
	}
	px, py := mobileProjectSurface(g.World.Cell(32, 32), g.World.Magnets[0].X, g.World.Magnets[0].Y, g.mobile.View)
	if g.mobile.Frame.RGBAAt(px, py-1) != (color.RGBA{R: 255, A: 255}) {
		t.Fatal("integrated mobile frame omitted the cached scene")
	}
	// Network rounds publish detached worlds; pointer replacement must invalidate
	// the cache even when a publication falls inside the same display quarter.
	next := new(engine.World)
	*next = *g.World
	next.Magnets[0].X += 256
	next.Actors.Move(engine.ActorRef{Kind: engine.ActorMagnet}, next.Magnets[0].X, next.Magnets[0].Y)
	g.World = next
	g.drawMobileFrame()
	if g.mobile.SceneCache.World != next || g.mobile.Frame.RGBAAt(px+16, py+7) != (color.RGBA{R: 255, A: 255}) || g.mobile.Frame.RGBAAt(px, py-1).R != 0 {
		t.Fatal("network world replacement reused stale cached actors")
	}
	if g.mobile.Frame.RGBAAt(0, 0) != mobileBG || g.mobile.Frame.RGBAAt(0, 239) != mobileBG {
		t.Fatal("wide scene overwrote toolbar pixels")
	}
}

func TestMobileNetworkPublicationPreservesCameraAndOngoingPan(t *testing.T) {
	g := mobileActionTestGame()
	g.mobile.World = g.World
	g.Network = &NetworkController{}
	g.mobile.View.CenterX, g.mobile.View.CenterY = 43.25, 11.5
	contacts := []mobileui.Contact{{ID: 1, X: 200, Y: 100}, {ID: 2, X: 230, Y: 120}}
	g.mobile.Gesture.Update(contacts, g.mobileRegion)
	next := new(engine.World)
	*next = *g.World
	g.World = next
	g.syncMobileWorld()
	if g.mobile.World != next || g.mobile.NeedsCenter || g.mobile.View.CenterX != 43.25 || g.mobile.View.CenterY != 11.5 {
		t.Fatal("network publication reset the independently panned camera")
	}
	contacts[0].X += 10
	contacts[1].X += 10
	if frame := g.mobile.Gesture.Update(contacts, g.mobileRegion); !frame.Panning || frame.Pan.X != 10 {
		t.Fatal("network publication canceled the ongoing drag", frame)
	}
	// Starting a new session or selecting a group explicitly requests centering.
	g.mobile.NeedsCenter = true
	g.CameraX, g.CameraY = 40, 8
	g.syncMobileWorld()
	if g.mobile.View.CenterX != 43.5 || g.mobile.View.CenterY != 11.5 || g.mobile.NeedsCenter {
		t.Fatal("explicit centering was lost while preserving network pans")
	}
}

func TestMobileNativeEditorGestureCoordinatesMatchTheDesktopVertex(t *testing.T) {
	for _, width := range []int{320, 540, 961} {
		g := mobileActionTestGame()
		g.Screen = EditorScreen
		g.mobile.Width, g.mobile.Overlay = width, "paint"
		g.Editor = &EditorState{Draft: g.World, Tool: EditorRaise}
		g.CameraX, g.CameraY = 28, 28
		g.mobile.Buttons = g.mobileFrontButtons()
		view := *g
		view.World = g.Editor.Draft
		px, py := view.projectCorner(32, 32)
		point := mobileui.Point{X: float64(px + (width-320)/2), Y: float64(py + 24)}
		if g.mobileRegion(point) != mobileui.RegionTerrain {
			t.Fatal("visible native editor vertex was not paintable", width, point)
		}
		before := g.Editor.Draft.Heights[32+32*engine.CornerSize]
		if err := g.handleMobileTap(mobileui.Tap{Point: point, Start: point, Region: mobileui.RegionTerrain}); err != nil {
			t.Fatal(err)
		}
		if g.Editor.Draft.Heights[32+32*engine.CornerSize] != before+1 {
			t.Fatal("native editor origin changed with device width", width)
		}
		if g.mobileRegion(mobileui.Point{X: float64((width-320)/2 + 10), Y: 80}) != mobileui.RegionNone {
			t.Fatal("native editor HUD was admitted as terrain")
		}
	}
}

func TestMobileMapReleaseJumpsWithinWorldWithoutCasting(t *testing.T) {
	for _, width := range []int{320, 540, 961} {
		for _, tile := range [][2]int{{0, 0}, {32, 27}, {63, 63}} {
			g := mobileActionTestGame()
			g.mobile.Width, g.mobile.Overlay = width, "map"
			g.mobile.Buttons = g.mobileHUDButtons()
			before := g.World.Snapshot()
			point := mobileui.Point{X: float64((width-128)/2 + 2*tile[0]), Y: float64(32 + 2*tile[1])}
			if g.mobileRegion(point) != mobileui.RegionTerrain {
				t.Fatal("overview square was not touchable", width, point)
			}
			if err := g.handleMobileTap(mobileui.Tap{Point: point, Start: point, Region: mobileui.RegionTerrain}); err != nil {
				t.Fatal(err)
			}
			if g.mobile.View.CenterX != float64(tile[0]) || g.mobile.View.CenterY != float64(tile[1]) || g.mobile.Overlay != "" || g.World.Snapshot() != before {
				t.Fatal("overview release cast a spell or selected the wrong map coordinate", width, tile, g.mobile.View)
			}
		}
	}
	g := mobileActionTestGame()
	g.mobile.Overlay = "map"
	g.mobile.Buttons = g.mobileHUDButtons()
	outside := mobileui.Point{X: 5, Y: 90}
	before := g.mobile.View
	_ = g.handleMobileTap(mobileui.Tap{Point: outside, Start: outside, Region: mobileui.RegionTerrain})
	if g.mobile.View != before || g.mobile.Overlay != "map" {
		t.Fatal("release outside the displayed overview moved the camera")
	}
}

func TestMobileBackCancelsPersistentToolbarFinger(t *testing.T) {
	g := mobileActionTestGame()
	g.mobile.Overlay = "menu"
	g.mobile.Buttons = g.mobileHUDButtons()
	menu := g.mobile.Buttons[7]
	contacts := []mobileui.Contact{{ID: 7, X: float64(menu.Rect.Min.X + 10), Y: float64(menu.Rect.Min.Y + 10)}}
	g.mobile.Gesture.Update(contacts, g.mobileRegion)
	g.mobileBack()
	if g.mobile.Overlay != "" {
		t.Fatal("Back did not close the menu")
	}
	g.mobile.Buttons = g.mobileHUDButtons()
	if frame := g.mobile.Gesture.Update(nil, g.mobileRegion); len(frame.Taps) != 0 {
		t.Fatal("finger held on Menu reopened the overlay after Back", frame.Taps)
	}
}

func TestMobileResultTransitionCancelsReleaseBeforeNewButtons(t *testing.T) {
	g := mobileActionTestGame()
	g.mobile.LastScreen = Playing
	g.mobile.Buttons = g.mobileHUDButtons()
	button := g.mobile.Buttons[7]
	contacts := []mobileui.Contact{{ID: 9, X: float64(button.Rect.Min.X + 10), Y: float64(button.Rect.Min.Y + 10)}}
	g.mobile.Gesture.Update(contacts, g.mobileRegion)
	// finishWorld can change the screen during the main simulation pass, before
	// the current sample is dispatched to the newly composed result controls.
	g.Screen = CampaignResult
	g.syncMobileScreen()
	g.mobile.Buttons = []mobileButton{{Rect: button.Rect, Action: "proceed", Enabled: true}}
	if g.mobile.LastScreen != CampaignResult {
		t.Fatal("result screen was not synchronized after simulation")
	}
	if frame := g.mobile.Gesture.Update(nil, g.mobileRegion); len(frame.Taps) != 0 {
		t.Fatal("held gameplay release activated a result button", frame.Taps)
	}
}

func TestMobileScreenActionCancelsHeldGestureUntilAllFingersLift(t *testing.T) {
	g := mobileActionTestGame()
	g.mobile.Buttons = g.mobileHUDButtons()
	button := g.mobile.Buttons[7]
	point := mobileui.Point{X: float64(button.Rect.Min.X + 10), Y: float64(button.Rect.Min.Y + 10)}
	contacts := []mobileui.Contact{{ID: 7, X: point.X, Y: point.Y}}
	g.mobile.Gesture.Update(contacts, g.mobileRegion)
	if err := g.handleMobileAction("help"); err != nil {
		t.Fatal(err)
	}
	if g.Screen != HelpScreen {
		t.Fatal("screen action did not enter help")
	}
	g.mobile.Buttons = g.mobileFrontButtons()
	if frame := g.mobile.Gesture.Update(nil, g.mobileRegion); len(frame.Taps) != 0 {
		t.Fatal("held gameplay finger activated the new screen on release", frame.Taps)
	}
}
