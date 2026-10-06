package app

import (
	"fmt"
	"image"
	"image/draw"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
)

type EditorTool uint8

const (
	EditorRaise EditorTool = iota
	EditorLower
	EditorLevel
	EditorBlue
	EditorRed
	EditorTree
	EditorRock
	EditorErase
)

type EditorState struct {
	Draft                     *engine.World
	Tool                      EditorTool
	Height, Population        int
	Return                    Screen
	CameraX, CameraY          int
	EventIndex                int
	Updates                   int
	Presentation              PlayingPresentation
	AnimationSounds           AnimationSoundGate
	EditingField, NumberInput string
}

func (g *Game) openEditor() error {
	if g.Network != nil {
		return fmt.Errorf("the map editor is unavailable during multiplayer")
	}
	var draft *engine.World
	var err error
	if g.World != nil {
		draft, err = g.World.Snapshot().Restore()
	} else {
		level := g.Assets.Levels[g.LevelIndex]
		if g.CustomLevel != nil {
			level = *g.CustomLevel
		}
		draft, err = engine.NewWorld(level, g.Assets.Landscapes[level.Landscape])
	}
	if err != nil {
		return err
	}
	draft.Editor = true
	g.Editor = &EditorState{Draft: draft, Return: g.Screen, Height: 1, Population: 100, CameraX: g.CameraX, CameraY: g.CameraY}
	g.Screen = EditorScreen
	return nil
}

func (g *Game) cancelEditor() {
	if g.Editor != nil {
		g.CameraX, g.CameraY = g.Editor.CameraX, g.Editor.CameraY
		g.Screen = g.Editor.Return
		g.Editor = nil
	}
}

func (g *Game) applyEditor() error {
	if g.Editor == nil || g.Editor.Draft == nil {
		return fmt.Errorf("editor draft missing")
	}
	if g.Network != nil {
		return fmt.Errorf("the map editor cannot apply during multiplayer")
	}
	candidate, err := g.Editor.Draft.Snapshot().Restore()
	if err != nil {
		return fmt.Errorf("edited world is invalid: %w", err)
	}
	candidate.Editor = false
	candidate.Result = 0
	g.World = candidate
	g.presentation.Reset()
	g.CustomGame = true
	g.restoreCustomSetup()
	g.resultApplied = false
	g.Paused = false
	g.AnimationSounds = AnimationSoundGate{}
	g.Screen = Playing
	g.Editor = nil
	return nil
}

func (s *EditorState) paint(x, y int) error {
	if s != nil {
		defer s.Presentation.Reset()
	}
	if s == nil || s.Draft == nil {
		return fmt.Errorf("editor draft missing")
	}
	w := s.Draft
	switch s.Tool {
	case EditorRaise, EditorLower, EditorLevel:
		if x < 0 || y < 0 || x >= engine.CornerSize || y >= engine.CornerSize {
			return fmt.Errorf("terrain vertex outside world")
		}
		height := int(w.Heights[x+y*engine.CornerSize])
		if s.Tool == EditorRaise {
			height = min(8, height+1)
		} else if s.Tool == EditorLower {
			height = max(0, height-1)
		} else {
			height = s.Height
		}
		return w.EditorSetHeight(x, y, height)
	case EditorBlue, EditorRed:
		owner := 0
		if s.Tool == EditorRed {
			owner = 1
		}
		return w.EditorPlaceFollower(owner, x, y, s.Population)
	case EditorTree:
		return w.EditorCycleScenery(engine.SceneryTree, x, y)
	case EditorRock:
		return w.EditorCycleScenery(engine.SceneryBoulder, x, y)
	case EditorErase:
		return w.EditorClearCell(x, y)
	default:
		return fmt.Errorf("unknown editor tool")
	}
}

var editorLabels = [8]string{"RAISE", "LOWER", "LEVEL", "BLUE", "RED", "TREE", "ROCK", "ERASE"}

func (g *Game) updateEditor(x, y int, clicked bool) error {
	if g.Editor == nil {
		return fmt.Errorf("editor screen has no draft")
	}
	s := g.Editor
	if s.EditingField != "" {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= '0' && r <= '9' && len(s.NumberInput) < 10 {
				s.NumberInput += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(s.NumberInput) > 0 {
			s.NumberInput = s.NumberInput[:len(s.NumberInput)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			return s.applyNumber()
		}
		return nil
	}
	g.advanceEditorPresentation()
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return g.applyEditor()
	}
	for index, key := range []ebiten.Key{ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4} {
		if inpututil.IsKeyJustPressed(key) {
			s.Tool = []EditorTool{EditorBlue, EditorRed, EditorTree, EditorRock}[index]
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		s.Tool = EditorRaise
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		s.Tool = EditorLower
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		s.Tool = EditorLevel
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		s.Height = max(0, s.Height-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		s.Height = min(8, s.Height+1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		s.Tool = EditorErase
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		g.CameraX = max(0, g.CameraX-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		g.CameraX = min(56, g.CameraX+1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		g.CameraY = max(0, g.CameraY-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		g.CameraY = min(56, g.CameraY+1)
	}
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if !clicked && !right {
		return nil
	}
	if clicked && g.Assets.EditorLayout != nil {
		if action := g.Assets.EditorLayout.ActionAt(x, y); action != "" {
			return g.applyEditorAction(action)
		}
	}
	px, py, ok := g.pickCorner(x, y)
	if !ok {
		return nil
	}
	if s.Tool >= EditorBlue {
		px = min(engine.MapSize-1, px)
		py = min(engine.MapSize-1, py)
	}
	if right && s.Tool >= EditorBlue {
		s.Presentation.Reset()
		return s.Draft.EditorRemoveFirst(px, py)
	}
	if right && s.Tool == EditorRaise {
		s.Presentation.Reset()
		return s.Draft.EditorSetHeight(px, py, max(0, int(s.Draft.Heights[px+py*engine.CornerSize])-1))
	}
	if err := s.paint(px, py); err != nil {
		g.Message, g.messageUntil = err.Error(), g.Updates+100
	}
	return nil
}

func (g *Game) applyEditorAction(action string) error {
	s := g.Editor
	defer s.Presentation.Reset()
	switch action {
	case "blue", "red", "tree", "rock":
		tool := map[string]EditorTool{"blue": EditorBlue, "red": EditorRed, "tree": EditorTree, "rock": EditorRock}[action]
		if s.Tool == tool {
			s.Tool = EditorRaise
		} else {
			s.Tool = tool
		}
	case "local-mana-add", "local-mana-subtract", "opponent-mana-add", "opponent-mana-subtract":
		side := g.playerSide()
		if action == "opponent-mana-add" || action == "opponent-mana-subtract" {
			side = 1 - side
		}
		delta := 8000
		if action == "local-mana-subtract" || action == "opponent-mana-subtract" {
			delta = -8000
		}
		s.Draft.Players[side].Mana = max(0, min(2147483647, s.Draft.Players[side].Mana+delta))
	case "landscape":
		s.Draft.Level.Landscape = (s.Draft.Level.Landscape + 1) % 4
		s.Draft.Landscape = g.Assets.Landscapes[s.Draft.Level.Landscape]
	case "new-map":
		fresh, err := s.Draft.EditorRegenerate()
		if err != nil {
			return err
		}
		s.Draft = fresh
	case "next-event":
		s.EventIndex = min(len(s.Draft.Scenario.Events)-1, s.EventIndex+1)
	case "previous-event":
		s.EventIndex = max(0, s.EventIndex-1)
	case "time", "x", "y", "effect":
		s.EditingField = action
		s.NumberInput = ""
	}
	return nil
}

var editorScenarioCodes = map[engine.ScenarioEventKind]int{engine.ScenarioNoEvent: 0, engine.ScenarioFireColumn: 6, engine.ScenarioWhirlwind: 22, engine.ScenarioEarthquake: 40, engine.ScenarioTrees: 46, engine.ScenarioVolcano: 62, engine.ScenarioStorm: 64, engine.ScenarioFireRain: 38, engine.ScenarioRoadMaker: 90, engine.ScenarioLandLowerer: 92, engine.ScenarioWhirlwindMaker: 94, engine.ScenarioTreePlanter: 96, engine.ScenarioFireMaker: 98, engine.ScenarioMonster: 100, engine.ScenarioWhirlpool: 24, engine.ScenarioBatholith: 48, engine.ScenarioBatholithAlternate: 50, engine.ScenarioBaptism: 52, engine.ScenarioSwamp: 54, engine.ScenarioTsunami: 56, engine.ScenarioBasalt: 74, engine.ScenarioWind: 76, engine.ScenarioFlowers: 80, engine.ScenarioCreateBlueFollower: 82, engine.ScenarioCreateRedFollower: 84, engine.ScenarioPlantTree: 86, engine.ScenarioPlantRock: 88, engine.ScenarioRemoveActor: 102}

func (s *EditorState) applyNumber() error {
	value, err := strconv.Atoi(s.NumberInput)
	if err != nil {
		return fmt.Errorf("enter a valid editor number")
	}
	event := s.Draft.Scenario.Events[s.EventIndex]
	switch s.EditingField {
	case "time":
		if value < 0 || value > 65535 {
			return fmt.Errorf("event time must be0..65535")
		}
		event.Time = uint16(value)
	case "x", "y":
		if value <= 0 || value >= 64 {
			return fmt.Errorf("event coordinate must be1..63")
		}
		if s.EditingField == "x" {
			event.X = uint8(value)
			event.Direction = 0
			if event.Kind == engine.ScenarioWind {
				event.Direction = 1
			}
		} else {
			event.Y = uint8(value)
		}
	case "effect":
		found := false
		for kind, code := range editorScenarioCodes {
			if value == code {
				event.Kind = kind
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("this scenario effect is not supported")
		}
	}
	if event.Kind == engine.ScenarioBasalt {
		event.Direction = event.X >> 5
	}
	if event.Kind == engine.ScenarioWind && s.EditingField == "effect" {
		event.Direction = 1
	}
	if err := s.Draft.EditorSetScenarioEvent(s.EventIndex, event); err != nil {
		return err
	}
	s.EditingField, s.NumberInput = "", ""
	s.Presentation.Reset()
	return nil
}

func (g *Game) editorValues() (map[string]string, map[string]bool) {
	s := g.Editor
	event := s.Draft.Scenario.Events[s.EventIndex]
	x := int(event.X)
	if event.Kind == engine.ScenarioEarthquake {
		x |= int(event.Direction) << 6
	}
	if event.Kind == engine.ScenarioWind {
		x |= int((event.Direction+3)&3) << 6
	}
	values := map[string]string{"time": strconv.Itoa(int(event.Time)), "x": strconv.Itoa(x), "y": strconv.Itoa(int(event.Y)), "effect": strconv.Itoa(editorScenarioCodes[event.Kind])}
	if s.EditingField != "" {
		values[s.EditingField] = s.NumberInput + "_"
	}
	return values, map[string]bool{"blue": s.Tool == EditorBlue, "red": s.Tool == EditorRed, "tree": s.Tool == EditorTree, "rock": s.Tool == EditorRock}
}

func (g *Game) drawEditor() {
	if g.Editor == nil {
		return
	}
	view := *g
	view.World = g.Editor.Draft
	if g.Editor.Presentation.Ready {
		view = g.Editor.Presentation.Renderer(&view)
	}
	// Editor tools and camera input stay current while world actors retain
	// the source renderer's pre-physics presentation state.
	view.CameraX, view.CameraY = g.CameraX, g.CameraY
	view.AnimationSounds = g.Editor.AnimationSounds
	view.drawWorld()
	g.Editor.AnimationSounds = view.AnimationSounds

	if g.Assets.EditorLayout != nil {
		layout := *g.Assets.EditorLayout
		layout.Palette = g.Assets.Visual.Palettes[g.Editor.Draft.Level.Landscape]
		values, flags := g.editorValues()
		layout.Draw(g.framebuffer, g.Assets.Visual.Font, values, flags)
	}
	// The source shows the currently selected scripted effect on the overview.
	event := g.Editor.Draft.Scenario.Events[g.Editor.EventIndex]
	if g.Assets.Pointers != nil {
		land := g.Editor.Draft.Level.Landscape
		id := g.Assets.Pointers.MapMarkerSprite
		if id >= 0 && id < len(g.Assets.Visual.Sprites[land]) {
			sprite := g.Assets.Visual.Sprites[land][id].Image
			if sprite != nil {
				px, py := 65-int(event.Y)+int(event.X), 3+(int(event.Y)+int(event.X))/2
				draw.Draw(g.framebuffer, image.Rect(px, py, px+sprite.Bounds().Dx(), py+sprite.Bounds().Dy()), sprite, image.Point{}, draw.Over)
			}
		}
	}
}

func (g *Game) advanceEditorPresentation() {
	s := g.Editor
	if s == nil || s.EditingField != "" {
		return
	}
	s.Updates++
	if s.Updates%4 != 0 {
		return
	}
	view := *g
	view.World = s.Draft
	s.Presentation.Capture(&view, !g.Paused)
	if !g.Paused {
		s.Draft.StepWithViewport(engine.Viewport{X: g.CameraX, Y: g.CameraY, Size: viewSize})
	}
}
