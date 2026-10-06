package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

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
	Draft              *engine.World
	Tool               EditorTool
	Height, Population int
	Return             Screen
	CameraX, CameraY   int
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
	g.World = candidate
	g.Screen = Playing
	g.Editor = nil
	return nil
}

func (s *EditorState) paint(x, y int) error {
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
		return w.EditorPlaceScenery(engine.SceneryTree, x, y)
	case EditorRock:
		return w.EditorPlaceScenery(engine.SceneryBoulder, x, y)
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
	if !clicked {
		return nil
	}
	if y >= 1 && y < 17 && x >= 0 && x < 320 {
		g.Editor.Tool = EditorTool(min(7, x/40))
		return nil
	}
	if y >= 177 && y < 194 {
		if x >= 0 && x < 72 {
			return g.applyEditor()
		}
		if x >= 248 && x < 320 {
			g.cancelEditor()
			return nil
		}
		if x >= 88 && x < 144 {
			g.Editor.Height = max(0, g.Editor.Height-1)
			return nil
		}
		if x >= 152 && x < 208 {
			g.Editor.Height = min(8, g.Editor.Height+1)
			return nil
		}
	}
	if y >= 163 && y < 176 && x < 140 {
		if x < 70 {
			g.Editor.Population = max(1, g.Editor.Population-100)
		} else {
			g.Editor.Population = min(1000000, g.Editor.Population+100)
		}
		return nil
	}
	// Picking borrows the detached draft solely for the pure projection helper.
	previous := g.World
	g.World = g.Editor.Draft
	px, py, ok := g.pickCorner(x, y)
	g.World = previous
	if !ok {
		return nil
	}
	if g.Editor.Tool >= EditorBlue {
		px = min(engine.MapSize-1, px)
		py = min(engine.MapSize-1, py)
	}
	if err := g.Editor.paint(px, py); err != nil {
		g.Message, g.messageUntil = err.Error(), g.Updates+100
	}
	return nil
}

func (g *Game) drawEditor() {
	if g.Editor == nil {
		return
	}
	previous := g.World
	g.World = g.Editor.Draft
	g.drawWorld()
	g.World = previous
	draw.Draw(g.framebuffer, image.Rect(0, 0, 320, 18), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	for i, label := range editorLabels {
		if EditorTool(i) == g.Editor.Tool {
			draw.Draw(g.framebuffer, image.Rect(i*40, 0, i*40+40, 18), image.NewUniform(color.RGBA{100, 95, 25, 255}), image.Point{}, draw.Src)
		}
		g.text(label, 2+i*40, 4)
	}
	draw.Draw(g.framebuffer, image.Rect(0, 162, 140, 177), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	g.text(fmt.Sprintf("-PEOPLE %d +", g.Editor.Population), 0, 164)
	draw.Draw(g.framebuffer, image.Rect(0, 177, 320, 200), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	g.button("PLAY", 0, 180, 72)
	g.button("-", 88, 180, 56)
	g.button("+", 152, 180, 56)
	g.button("CANCEL", 248, 180, 72)
	g.text(fmt.Sprintf("HEIGHT %d", g.Editor.Height), 89, 193)
	if g.Updates < g.messageUntil {
		g.text("CANNOT PLACE HERE", 144, 164)
	}
}
