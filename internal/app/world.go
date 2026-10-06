package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
)

const viewSize = 8

var compassNames = [8]string{"north", "northeast", "east", "southeast", "south", "southwest", "west", "northwest"}

func (g *Game) startConquest() error {
	level := g.Assets.Levels[g.LevelIndex]
	world, err := engine.NewWorld(level, g.Assets.Landscapes[level.Landscape])
	if err != nil {
		return err
	}
	g.World, g.Screen = world, Playing
	leader := world.Players[0].Leader
	if leader > 0 {
		f := world.Followers[leader]
		g.CameraX = max(0, min(56, int(f.X)-3))
		g.CameraY = max(0, min(56, int(f.Y)-3))
	}
	g.music.TriggerCue(78)
	return nil
}

func (g *Game) updateWorld(mouseX, mouseY int, clicked bool) error {
	if g.World == nil {
		return fmt.Errorf("playing screen has no world")
	}
	w := g.World
	if g.Updates%4 == 0 {
		w.Step()
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
	for i, key := range []ebiten.Key{ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4} {
		if inpututil.IsKeyJustPressed(key) {
			w.SetMode(0, []engine.Mode{engine.Settle, engine.Rally, engine.Join, engine.Fight}[i])
		}
	}
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if mouseY < 70 && mouseX < 138 && (clicked || right) {
		dx := mouseX - 68
		sum := (mouseY - 4) * 2
		g.CameraX = max(0, min(56, (sum+dx)/2-3))
		g.CameraY = max(0, min(56, (sum-dx)/2-3))
		return nil
	}
	if clicked || right {
		x, y, ok := g.pickCorner(mouseX, mouseY)
		if ok {
			changed := false
			if right {
				changed = w.LowerAt(0, x, y)
			} else {
				changed = w.RaiseAt(0, x, y)
			}
			if changed {
				g.music.TriggerCue(78)
			}
		}
	}
	return nil
}

func (g *Game) projectCorner(x, y int) (int, int) {
	return 192 + 16*(x-g.CameraX-y+g.CameraY), 72 + 8*(x-g.CameraX+y-g.CameraY) - int(g.World.Heights[x+y*engine.CornerSize])*8
}

// pickCorner chooses the visible projected vertex instead of a rectangular
// tile hitbox. Front vertices win equal distances, matching painter ordering.
func (g *Game) pickCorner(mouseX, mouseY int) (int, int, bool) {
	if mouseX < 104 || mouseY < 45 || mouseY > 177 {
		return 0, 0, false
	}
	bestDistance, bestX, bestY := float64(12*12), 0, 0
	found := false
	for sum := 0; sum <= viewSize*2; sum++ {
		for dx := 0; dx <= viewSize; dx++ {
			dy := sum - dx
			if dy < 0 || dy > viewSize {
				continue
			}
			x, y := g.CameraX+dx, g.CameraY+dy
			if x > 64 || y > 64 {
				continue
			}
			sx, sy := g.projectCorner(x, y)
			distance := float64((sx-mouseX)*(sx-mouseX) + (sy-mouseY)*(sy-mouseY))
			if distance <= bestDistance {
				bestDistance, bestX, bestY, found = distance, x, y, true
			}
		}
	}
	return bestX, bestY, found
}

func (g *Game) drawWorld() {
	w := g.World
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), g.Assets.Visual.Background, image.Point{}, draw.Src)
	land := w.Level.Landscape
	// Draw back-to-front isometric rows, including actors on each surface.
	for sum := 0; sum < viewSize*2-1; sum++ {
		for dx := 0; dx < viewSize; dx++ {
			dy := sum - dx
			if dy < 0 || dy >= viewSize {
				continue
			}
			x, y := g.CameraX+dx, g.CameraY+dy
			cell := w.Cell(x, y)
			sx := 192 + 16*(dx-dy) - 16
			sy := 64 + 8*(dx+dy) - int(cell.BaseAltitude)*8
			index := cell.TileIndex(int(w.Tick), dx, dy)
			if index >= 0 && index < len(g.Assets.Visual.Tiles[land]) {
				tile := g.Assets.Visual.Tiles[land][index]
				draw.Draw(g.framebuffer, image.Rect(sx, sy, sx+tile.Bounds().Dx(), sy+tile.Bounds().Dy()), tile, image.Point{}, draw.Over)
			}
			for id := 1; id < len(w.Followers); id++ {
				f := w.Followers[id]
				if f.State == engine.Inactive || int(f.X) != x || int(f.Y) != y {
					continue
				}
				ax, ay := g.projectCorner(x, y)
				ay += 8
				if f.State == engine.Town {
					g.animation(fmt.Sprintf("town/%d/%d", f.Owner, f.Stage), 0, ax, ay, land)
				} else {
					mapX, mapY := f.Position()
					ax = int(math.Round(192 + 16*(mapX-float64(g.CameraX)-mapY+float64(g.CameraY))))
					ay = int(math.Round(72+8*(mapX-float64(g.CameraX)+mapY-float64(g.CameraY)))) - int(w.Heights[x+y*engine.CornerSize])*8
					g.animation(fmt.Sprintf("follower/%d/0/%s", f.Owner, compassNames[f.Direction&7]), int(f.Frame), ax, ay, land)
				}
			}
		}
	}
	g.minimap(land)
	summary := w.Summaries()
	g.text(fmt.Sprintf("POP %d", summary[0].Population), 8, 181)
	g.text(fmt.Sprintf("MANA %d", summary[0].Mana), 8, 191)
}

func (g *Game) animation(name string, frame, x, y, land int) {
	animation, ok := g.Assets.Visual.Animations[name]
	if !ok || len(animation.Frames) == 0 {
		return
	}
	frame %= len(animation.Frames)
	for _, layer := range animation.Frames[frame].Layers {
		if layer.Sprite < 0 || layer.Sprite >= len(g.Assets.Visual.Sprites[land]) {
			continue
		}
		sprite := g.Assets.Visual.Sprites[land][layer.Sprite]
		if sprite.Image == nil {
			continue
		}
		left, top := x+layer.X-sprite.AnchorX, y+layer.Y-sprite.AnchorY
		draw.Draw(g.framebuffer, image.Rect(left, top, left+sprite.Image.Bounds().Dx(), top+sprite.Image.Bounds().Dy()), sprite.Image, image.Point{}, draw.Over)
	}
}

func (g *Game) minimap(land int) {
	for y := 0; y < engine.MapSize; y++ {
		for x := 0; x < engine.MapSize; x++ {
			cell := g.World.Cell(x, y)
			sx, sy := 68+x-y, 4+(x+y)/2
			c := g.Assets.Visual.MapColors[land][cell.Code]
			if id := g.World.Occupants[x+y*engine.MapSize]; id > 0 {
				if g.World.Followers[id].Owner == 0 {
					c = color.RGBA{255, 225, 30, 255}
				} else {
					c = color.RGBA{220, 50, 30, 255}
				}
			}
			g.framebuffer.SetRGBA(sx, sy, c)
		}
	}
	x, y := g.CameraX+3, g.CameraY+3
	sx, sy := 68+x-y, 4+(x+y)/2
	for d := -3; d <= 3; d++ {
		g.framebuffer.SetRGBA(sx+d, sy, color.RGBA{255, 255, 255, 255})
		g.framebuffer.SetRGBA(sx, sy+d, color.RGBA{255, 255, 255, 255})
	}
}
