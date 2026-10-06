package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
	"go-populous2/internal/network"
)

const viewSize = 8

var compassNames = [8]string{"north", "northeast", "east", "southeast", "south", "southwest", "west", "northwest"}

func (g *Game) startConquest() error {
	if g.Network != nil && g.Network.Status().Ready {
		return fmt.Errorf("leave the current two-player game before starting another")
	}
	level := g.Assets.Levels[g.LevelIndex]
	if g.CustomGame && g.CustomLevel != nil {
		level = *g.CustomLevel
	}
	world, err := engine.NewWorld(level, g.Assets.Landscapes[level.Landscape])
	if err != nil {
		return err
	}
	world.Players[0].Experience = g.Profile.Experience
	if g.CustomGame && g.CustomLevel != nil {
		world.Players[0].Computer, world.Players[1].Computer = g.CustomComputer[0], g.CustomComputer[1]
	}
	if g.Network != nil {
		world.Players[0].Computer, world.Players[1].Computer = false, false
		if err := g.Network.Start(world); err != nil {
			return err
		}
	}
	g.World, g.Screen = world, Playing
	g.LocalSide, g.OriginalSave = 0, nil
	g.Paused = false
	g.AnimationSounds = AnimationSoundGate{}
	g.resultApplied = false
	g.SelectedFollower, g.Inspecting = world.Players[g.playerSide()].Leader, false
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
	if g.Network != nil {
		g.advanceNetwork()
	} else if !g.Paused && g.Updates%4 == 0 {
		w.Step()
		if w.Scenario.Err != "" {
			g.Message, g.messageUntil = "WORLD EVENT: "+w.Scenario.Err, g.Updates+200
		}
		if w.Result != 0 {
			g.finishWorld()
			return nil
		}
	}
	g.refreshSelectedFollower()
	if g.handleSelectionPanelClick(mouseX, mouseY, clicked) {
		return nil
	}
	if g.PickingPower {
		if clicked {
			if mouseY >= 47 && mouseY < 63 && mouseX >= 24 && mouseX < 296 {
				g.Category = engine.Element(min(5, (mouseX-24)/45))
			}
			row := (mouseY - 77) / 19
			if mouseX >= 40 && mouseX < 280 && mouseY >= 77 && row >= 0 && row < 5 {
				id := engine.PowerID(int(g.Category)*6 + row)
				if power, ok := engine.PowerByID(id); ok && power.Implemented {
					g.Selected, g.PickingPower = id, false
				}
			}
		}
		return nil
	}
	if clicked {
		if handled, err := g.handleHUDClick(mouseX, mouseY); handled {
			if err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+150
			}
			return nil
		}
		if id := g.pickFollower(mouseX, mouseY); id != 0 {
			g.SelectedFollower = id
			if g.Inspecting {
				return nil
			}
		} else if g.Inspecting && mouseX >= 104 && mouseY >= 45 && mouseY < 178 {
			return nil
		}
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
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.Selected = engine.PapalMagnet
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.Selected = engine.RaiseLower
	}
	for index, key := range []ebiten.Key{ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF5} {
		if inpututil.IsKeyJustPressed(key) {
			g.Selected = []engine.PowerID{engine.Trees, engine.Flowers, engine.Swamp, engine.Fungus}[index]
		}
	}
	for index, key := range []ebiten.Key{ebiten.KeyF6, ebiten.KeyF7} {
		if inpututil.IsKeyJustPressed(key) {
			g.Selected = []engine.PowerID{engine.FireColumn, engine.FireRain}[index]
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF8) {
		g.Selected = engine.Basalt
	}
	if g.Selected == engine.Lightning && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		var err error
		if g.Network != nil {
			err = g.submitNetwork(network.Command{Kind: "lightning-activate"})
		} else {
			err = w.ActivateLightning(g.playerSide())
		}
		if err != nil {
			g.Message, g.messageUntil = err.Error(), g.Updates+100
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		g.Direction = (g.Direction + 3) % 4
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		g.Direction = (g.Direction + 1) % 4
	}
	for i, key := range []ebiten.Key{ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4} {
		if inpututil.IsKeyJustPressed(key) {
			mode := []engine.Mode{engine.Settle, engine.Rally, engine.Join, engine.Fight}[i]
			if g.Network != nil {
				_ = g.submitNetwork(network.Command{Kind: "mode", Mode: mode})
			} else {
				w.SetMode(g.playerSide(), mode)
			}
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
			if g.Selected == engine.RaiseLower {
				if err := g.applyTerrainClick(x, y, right); err != nil {
					g.Message, g.messageUntil = err.Error(), g.Updates+100
				} else {
					g.music.TriggerCue(78)
				}
				return nil
			}
			if right && g.Selected == engine.Lightning {
				if g.Network != nil {
					if err := g.submitNetwork(network.Command{Kind: "lightning-dismiss"}); err != nil {
						g.Message, g.messageUntil = err.Error(), g.Updates+100
					}
				} else {
					w.DismissLightning(g.playerSide())
				}
				return nil
			}
			target := engine.PowerTarget{X: x, Y: y, Lower: right, Direction: g.Direction}
			var err error
			if g.Network != nil {
				err = g.submitNetwork(network.Command{Kind: "power", Power: g.Selected, Target: target})
			} else {
				err = w.Cast(g.playerSide(), g.Selected, target)
			}
			if err == nil {
				g.music.TriggerCue(78)
			} else {
				g.Message, g.messageUntil = err.Error(), g.Updates+100
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
	g.drawTerrainBackdrop()
	land := w.Level.Landscape
	var commands [viewSize*viewSize + engine.FollowerCapacity + engine.EffectCapacity + engine.SceneryCapacity + engine.WallCapacity + 2]RenderCommand
	for _, command := range sourceRenderPlan(w, g.CameraX, g.CameraY, commands[:0]) {
		if command.Actor.Kind != engine.ActorNone {
			g.drawRegisteredActor(command.Actor, land)
			continue
		}
		x, y := command.X, command.Y
		dx, dy := x-g.CameraX, y-g.CameraY
		cell := w.Cell(x, y)
		sx := 192 + 16*(dx-dy) - 16
		sy := 64 + 8*(dx+dy) - int(cell.BaseAltitude)*8
		index := cell.TileIndex(int(w.Tick), dx, dy)
		if index >= 0 && index < len(g.Assets.Visual.Tiles[land]) {
			tile := g.Assets.Visual.Tiles[land][index]
			draw.Draw(g.framebuffer, image.Rect(sx, sy, sx+tile.Bounds().Dx(), sy+tile.Bounds().Dy()), tile, image.Point{}, draw.Over)
		}
		g.drawTownSurroundingsAt(x, y, land)
	}
	g.drawLightningBeams(land)
	g.minimap(land)
	g.drawHUDControls()
	g.drawSelectionPanel()
	summary := w.Summaries()[g.playerSide()]
	g.text(fmt.Sprintf("POP %d", summary.Population), 8, 181)
	g.text(fmt.Sprintf("MANA %d", summary.Mana), 8, 191)
	if power, ok := engine.PowerByID(g.Selected); ok {
		g.text(strings.ToUpper(power.Name), 144, 181)
	}
	if g.Selected == engine.Basalt || g.Selected == engine.Earthquake || g.Selected == engine.Wind {
		g.text("DIRECTION "+[4]string{"NORTH", "EAST", "SOUTH", "WEST"}[g.Direction&3], 144, 191)
	}
	if g.Updates < g.messageUntil {
		g.drawMessage(178)
	}
	if g.PickingPower {
		g.drawPowerMenu()
	}
	if g.Paused {
		g.button("PAUSED", 120, 91, 80)
	}
}

func (g *Game) drawPowerMenu() {
	draw.Draw(g.framebuffer, image.Rect(16, 29, 304, 183), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	g.text("DIVINE POWERS", 104, 32)
	for index, label := range [6]string{"PEOP", "PLNT", "EARTH", "AIR", "FIRE", "WATER"} {
		g.button(label, 24+index*45, 47, 43)
	}
	for row := 0; row < 5; row++ {
		id := engine.PowerID(int(g.Category)*6 + row)
		power, ok := engine.PowerByID(id)
		if !ok {
			continue
		}
		name := strings.ToUpper(power.Name)
		if !g.World.Level.Players[g.playerSide()].Powers[id] {
			name += " OFF"
		} else {
			name += fmt.Sprintf(" %d", g.World.PowerCost(g.playerSide(), id))
		}
		g.button(name, 40, 77+row*19, 240)
	}
}

var heroNames = [7]string{"", "perseus", "adonis", "heracles", "odysseus", "achilles", "helen"}
var neutralNames = [7]string{"", "road-maker", "land-lowerer", "whirlwind-maker", "tree-planter", "fire-maker", "monster"}

func (g *Game) animation(name string, frame, x, y, land int) {
	g.animationCropped(name, frame, x, y, land, 0)
}

func (g *Game) animationCropped(name string, frame, x, y, land, age int) {
	animation, ok := g.Assets.Visual.Animations[name]
	if !ok || len(animation.Frames) == 0 {
		return
	}
	if animation.Loop {
		frame %= len(animation.Frames)
	} else {
		frame = min(frame, len(animation.Frames)-1)
	}
	g.playAnimationCue(name, frame, 0)
	for index, layer := range animation.Frames[frame].Layers {
		if age != 0 && index > 0 {
			break
		}
		if layer.Sprite < 0 || layer.Sprite >= len(g.Assets.Visual.Sprites[land]) {
			continue
		}
		sprite := g.Assets.Visual.Sprites[land][layer.Sprite]
		if sprite.Image == nil {
			continue
		}
		height := sprite.Image.Bounds().Dy()
		anchor := sprite.AnchorY
		if age != 0 {
			height -= absInt(age)
			anchor -= 8 + absInt(age)
		}
		if height <= 0 {
			continue
		}
		left, top := x+layer.X-sprite.AnchorX, y+layer.Y-anchor
		draw.Draw(g.framebuffer, image.Rect(left, top, left+sprite.Image.Bounds().Dx(), top+height), sprite.Image, image.Point{}, draw.Over)
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (g *Game) minimap(land int) {
	for y := 0; y < engine.MapSize; y++ {
		for x := 0; x < engine.MapSize; x++ {
			cell := g.World.Cell(x, y)
			sx, sy := 68+x-y, 4+(x+y)/2
			c := g.Assets.Visual.MapColors[land][cell.Code]
			if f, visible := visibleOverviewFollower(g.World, g.playerSide(), x, y); visible {
				if f.Owner == 0 {
					c = color.RGBA{255, 225, 30, 255}
				} else {
					c = color.RGBA{220, 50, 30, 255}
				}
			}

			g.framebuffer.SetRGBA(sx, sy, c)
		}
	}
	g.drawOverviewEffects(land)
	x, y := g.CameraX+3, g.CameraY+3
	sx, sy := 68+x-y, 4+(x+y)/2
	for d := -3; d <= 3; d++ {
		g.framebuffer.SetRGBA(sx+d, sy, color.RGBA{255, 255, 255, 255})
		g.framebuffer.SetRGBA(sx, sy+d, color.RGBA{255, 255, 255, 255})
	}
}
