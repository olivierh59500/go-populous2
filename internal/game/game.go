package game

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"

	"go-populous2/internal/fixedstep"
	legacy "go-populous2/internal/legacy"
	"go-populous2/internal/populous2"
)

const Width, Height = 640, 480

const spellButtonsY, spellButtonStep = 252, 34

type Game struct {
	Bundle                    *populous2.Bundle
	World                     *populous2.World
	LevelIndex                int
	Playing                   bool
	Profile                   populous2.Deity
	DeityScreen               bool
	ScenarioScreen            bool
	scenarioSide              int
	customScenarioOptions     [2]uint16
	hasCustomScenarioOptions  bool
	deityPortrait             *ebiten.Image
	deityPassword             string
	deityEditing              bool
	deityNameEditing          bool
	Paused                    bool
	Help                      bool
	Category                  populous2.Element
	Selected                  populous2.SpellID
	CameraX, CameraY          int
	Direction                 int
	lineStart                 *image.Point
	scheduler                 *fixedstep.Scheduler
	background                *ebiten.Image
	tiles                     [4][]*ebiten.Image
	sprites                   [4][]*ebiten.Image
	minimap                   *ebiten.Image
	miniPixels                []byte
	Message                   string
	messageTicks              int
	Updates                   int
	Limit                     int
	Capture                   string
	CaptureAfter              int
	captured                  bool
	failure                   error
	SavePath                  string
	audioReplay               *populous2.AudioReplay
	audioPlayer               *audio.Player
	muted                     bool
	lastSoundSerial           int
	lastPlagueSoundTurn       int
	lastNativeEffectSoundTurn int
	roadLastTile              int
	roadDragging              bool
}

func New(bundle *populous2.Bundle, level int, demo, custom bool) (*Game, error) {
	world, err := populous2.NewWorld(bundle, level, custom)
	if err != nil {
		return nil, err
	}
	g := &Game{Bundle: bundle, World: world, LevelIndex: level, Playing: demo, Selected: populous2.RaiseLower, scheduler: fixedstep.New(populous2.SimulationRate, 60), SavePath: "go-populous2.sav"}
	world.Demo = demo
	g.Profile = populous2.NewDeity("PLAYER")
	g.applyDeityProfile()
	g.background = ebiten.NewImageFromImage(bundle.Background)
	for terrain := range g.tiles {
		for _, tile := range bundle.Tiles[terrain] {
			g.tiles[terrain] = append(g.tiles[terrain], ebiten.NewImageFromImage(tile))
		}
		for _, sprite := range bundle.Sprites[terrain] {
			var img *ebiten.Image
			if sprite.Image != nil {
				img = ebiten.NewImageFromImage(sprite.Image)
			}
			g.sprites[terrain] = append(g.sprites[terrain], img)
		}
	}
	g.minimap = ebiten.NewImage(128, 64)
	g.miniPixels = make([]byte, 128*64*4)
	g.centerPlayer(0)
	return g, nil
}

func (g *Game) Update() error {
	if g.failure != nil {
		return g.failure
	}
	g.Updates++
	if g.Limit > 0 && g.Updates >= g.Limit {
		return ebiten.Termination
	}
	if !g.DeityScreen && inpututil.IsKeyJustPressed(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if !g.DeityScreen && inpututil.IsKeyJustPressed(ebiten.KeyH) {
		g.Help = !g.Help
		g.lineStart = nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.ScenarioScreen {
			g.ScenarioScreen = false
		} else if g.DeityScreen {
			g.DeityScreen = false
			g.deityEditing = false
			g.applyDeityProfile()
		} else if g.Help {
			g.Help = false
		} else if g.lineStart != nil {
			g.lineStart = nil
		} else if g.Playing {
			g.Playing = false
		} else {
			return ebiten.Termination
		}
	}
	if g.Help {
		return nil
	}
	if g.DeityScreen {
		return g.updateDeity()
	}
	if g.ScenarioScreen {
		return g.updateScenarioRules()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyO) {
		g.OpenScenarioRules()
		return nil
	}
	if g.audioPlayer == nil {
		if err := g.initializeAudio(); err != nil {
			return err
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		g.muted = !g.muted
		if g.muted {
			g.audioPlayer.SetVolume(0)
		} else {
			g.audioPlayer.SetVolume(.35)
		}
	}
	if !g.Playing {
		return g.updateMenu()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.Paused = !g.Paused
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		g.save()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		g.load()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.centerPlayer(0)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		g.Direction = (g.Direction + 7) % 8
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		g.Direction = (g.Direction + 1) % 8
	}
	for i, key := range []ebiten.Key{ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF6, ebiten.KeyF7} {
		if inpututil.IsKeyJustPressed(key) {
			g.Category = populous2.Element(i)
			g.lineStart = nil
		}
	}
	for i, key := range []ebiten.Key{ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4} {
		if inpututil.IsKeyJustPressed(key) {
			g.World.Core.SetMagnetMode(0, []int{legacy.SettleMode, legacy.JoinMode, legacy.FightMode, legacy.MagnetMode}[i])
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.selectSpell(populous2.PapalMagnet)
	}
	if g.World.Core.ResultFor(0) != legacy.ResultOngoing {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			next := g.LevelIndex
			if g.World.Core.ResultFor(0) == legacy.ResultWon {
				next = min(next+1, len(g.Bundle.Levels)-1)
			}
			return g.start(next, g.World.Custom, false)
		}
		return nil
	}
	g.updateCamera()
	g.handleClick()
	if !g.Paused {
		for range g.scheduler.Advance() {
			g.World.Tick()
			if g.messageTicks > 0 {
				g.messageTicks--
			}
		}
	}
	if g.lastSoundSerial != g.World.SpellSerial {
		g.lastSoundSerial = g.World.SpellSerial
		g.playPowerSound(g.World.LastSpell, g.World.LastPlayer)
	}
	if g.audioReplay != nil && g.lastNativeEffectSoundTurn != g.World.Core.GameTurn {
		g.lastNativeEffectSoundTurn = g.World.Core.GameTurn
		var played [133]bool
		for _, actor := range g.World.NativeEffects {
			if !actor.Active {
				continue
			}
			frame, ok := g.Bundle.NativeEffectFrame(actor)
			if !ok {
				continue
			}
			cue := frame.SoundCue
			if cue > 0 && cue < len(played) && !played[cue] {
				g.audioReplay.PlayCue(cue)
				played[cue] = true
			}
		}
	}
	if g.audioReplay != nil && len(g.Bundle.PlagueAnimation) > 0 && g.World.Core.GameTurn%len(g.Bundle.PlagueAnimation) == 0 && g.lastPlagueSoundTurn != g.World.Core.GameTurn {
		g.lastPlagueSoundTurn = g.World.Core.GameTurn
		for _, p := range g.World.Core.Peeps {
			if p.Population > 0 && p.Plague {
				g.audioReplay.PlayCue(1)
				break
			}
		}
	}
	return nil
}

func (g *Game) updateMenu() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.OpenDeity()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		g.LevelIndex = max(0, g.LevelIndex-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		g.LevelIndex = min(len(g.Bundle.Levels)-1, g.LevelIndex+1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return g.start(g.LevelIndex, false, false)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyD) {
		return g.start(g.LevelIndex, false, true)
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()
	if hit(x, y, 470, 76, 105, 27) {
		g.OpenDeity()
		return nil
	}
	if hit(x, y, 110, 168, 420, 38) {
		return g.start(g.LevelIndex, false, false)
	}
	if hit(x, y, 110, 214, 420, 38) {
		return g.start(g.LevelIndex, true, false)
	}
	if hit(x, y, 110, 260, 420, 38) {
		return g.start(g.LevelIndex, false, true)
	}
	if hit(x, y, 110, 306, 420, 38) {
		g.load()
		return nil
	}
	if hit(x, y, 110, 352, 200, 32) {
		g.LevelIndex = max(0, g.LevelIndex-1)
	}
	if hit(x, y, 330, 352, 200, 32) {
		g.LevelIndex = min(len(g.Bundle.Levels)-1, g.LevelIndex+1)
	}
	return nil
}

func (g *Game) start(level int, custom, demo bool) error {
	world, err := populous2.NewWorld(g.Bundle, level, custom)
	if err != nil {
		return err
	}
	world.Demo = demo
	g.World = world
	if custom && g.hasCustomScenarioOptions {
		for player, raw := range g.customScenarioOptions {
			g.World.Rules[player] = populous2.DecodeScenarioRules(raw)
		}
	}
	g.applyDeityProfile()
	g.LevelIndex = level
	g.Playing = true
	g.Paused = false
	g.Selected = populous2.RaiseLower
	g.Category = populous2.People
	g.lineStart = nil
	g.centerPlayer(0)
	g.notify("Clic gauche: lever. Clic droit: baisser. H: aide.")
	return nil
}

func (g *Game) updateCamera() {
	keys := []struct {
		key    ebiten.Key
		dx, dy int
	}{{ebiten.KeyW, 0, -1}, {ebiten.KeyS, 0, 1}, {ebiten.KeyA, -1, 0}, {ebiten.KeyD, 1, 0}, {ebiten.KeyArrowUp, 0, -1}, {ebiten.KeyArrowDown, 0, 1}, {ebiten.KeyArrowLeft, -1, 0}, {ebiten.KeyArrowRight, 1, 0}}
	for _, entry := range keys {
		n := inpututil.KeyPressDuration(entry.key)
		if n == 1 || n > 15 && n%5 == 0 {
			g.CameraX = clamp(g.CameraX+entry.dx, 0, 56)
			g.CameraY = clamp(g.CameraY+entry.dy, 0, 56)
		}
	}
}

func (g *Game) centerPlayer(player int) {
	pos := g.World.Core.Magnets[player].GoTo
	index := g.World.Core.Magnets[player].Carried - 1
	if index >= 0 && index < len(g.World.Core.Peeps) && g.World.Core.Peeps[index].Population > 0 {
		pos = g.World.Core.Peeps[index].AtPos
	} else {
		for _, p := range g.World.Core.Peeps {
			if p.Population > 0 && int(p.Player) == player {
				pos = p.AtPos
				break
			}
		}
	}
	g.CameraX = clamp(pos%64-3, 0, 56)
	g.CameraY = clamp(pos/64-3, 0, 56)
}

func (g *Game) handleClick() {
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if g.Selected == populous2.Road {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
			g.roadDragging = false
		}
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			left = true
		}
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
			right = true
		}
	}
	if g.Selected == populous2.Batholith && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && g.Updates%8 == 0 {
		left = true
	}
	if !left && !right {
		return
	}
	x, y := ebiten.CursorPosition()
	if left && y >= 448 {
		for i, mode := range []int{legacy.SettleMode, legacy.JoinMode, legacy.FightMode, legacy.MagnetMode} {
			if hit(x, y, 136+i*87, 448, 83, 27) {
				g.World.Core.SetMagnetMode(0, mode)
				return
			}
		}
		if hit(x, y, 490, 448, 65, 27) {
			g.save()
			return
		}
		if hit(x, y, 562, 448, 73, 27) {
			g.Playing = false
			return
		}
	}
	if hit(x, y, 8, 56, 256, 128) {
		u, v := (x-8)/2-64, (y-56)/2
		mx, my := (u+v*2)/2, (v*2-u)/2
		g.CameraX = clamp(mx-3, 0, 56)
		g.CameraY = clamp(my-3, 0, 56)
		return
	}
	if left {
		for i := 0; i < 6; i++ {
			if hit(x, y, 4+(i%3)*41, 200+(i/3)*25, 39, 23) {
				g.Category = populous2.Element(i)
				g.lineStart = nil
				return
			}
		}
		for i, spell := range g.categorySpells() {
			if hit(x, y, 4, spellButtonsY+i*spellButtonStep, 123, 32) {
				g.selectSpell(spell.ID)
				return
			}
		}
	}
	if g.Paused || g.World.Demo {
		return
	}
	mx, my, ok := g.targetAt(x, y)
	if !ok {
		return
	}
	if g.Selected == populous2.Road {
		pos := mx + my*64
		if g.roadDragging && g.roadLastTile == pos {
			return
		}
		g.roadDragging = true
		g.roadLastTile = pos
		if right {
			g.World.RemoveRoad(mx, my)
			return
		}
	}
	if g.Selected == populous2.RaiseLower {
		if right && g.World.Sprog(0, mx, my) {
			g.notify("Les adorateurs sortent de l'habitation.")
			return
		}
		if !g.World.Sculpt(0, mx, my, left) {
			g.notify("Terrain indisponible ou mana insuffisant.")
		}
		return
	}
	if right {
		g.Selected = populous2.RaiseLower
		g.lineStart = nil
		return
	}
	spell, ok := populous2.SpellByID(g.World.Spells, g.Selected)
	if !ok {
		return
	}
	target := populous2.Target{X: mx, Y: my, Direction: g.Direction}
	if spell.Aim == populous2.AimLine {
		if g.lineStart == nil {
			p := image.Pt(mx, my)
			g.lineStart = &p
			g.notify("Choisis la seconde extremite.")
			return
		}
		target.X, target.Y = g.lineStart.X, g.lineStart.Y
		target.X2, target.Y2 = mx, my
		g.lineStart = nil
	}
	if g.World.Cast(0, spell.ID, target) {
		g.notify(spell.Name + " lance.")
	} else {
		g.notify("Pouvoir indisponible, cible invalide ou mana insuffisant.")
	}
}

func (g *Game) selectSpell(id populous2.SpellID) {
	spell, ok := populous2.SpellByID(g.World.Spells, id)
	if !ok {
		return
	}
	if !g.World.Available(0, id) {
		g.notify("Ce pouvoir n'est pas disponible sur ce monde.")
		return
	}
	g.lineStart = nil
	if spell.Aim == populous2.AimLeader || spell.Aim == populous2.AimGlobal {
		if g.World.Cast(0, id, populous2.Target{}) {
			g.notify(spell.Name + " lance.")
		} else {
			g.notify("Mana insuffisant ou aucun chef disponible.")
		}
		return
	}
	g.Selected = id
	g.notify(spell.Help)
}

func (g *Game) categorySpells() []populous2.Spell {
	var result []populous2.Spell
	for _, spell := range g.World.Spells {
		if spell.Element == g.Category {
			result = append(result, spell)
		}
	}
	return result
}

func (g *Game) targetAt(x, y int) (int, int, bool) {
	if x < 128 || y < 100 || y >= 440 {
		return 0, 0, false
	}
	found := false
	mx, my := 0, 0
	for row := 0; row < 8; row++ {
		for column := 0; column < 8; column++ {
			cell := g.World.TerrainCell(g.CameraX+column, g.CameraY+row)
			px, py := project(column, row, cell.BaseAltitude)
			if cell.Contains((x-px)/2, (y-py)/2) {
				found = true
				mx, my = g.CameraX+column, g.CameraY+row
			}
		}
	}
	return mx, my, found
}

func project(x, y, alt int) (int, int) { return (176 + (x-y)*16) * 2, 40 + (64+(x+y)*8-alt*8)*2 }

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{15, 17, 22, 255})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(0, 40)
	screen.DrawImage(g.background, op)
	if g.ScenarioScreen {
		g.drawScenarioRules(screen)
	} else if g.DeityScreen {
		g.drawDeity(screen)
	} else if !g.Playing {
		g.drawMenu(screen)
	} else {
		g.drawGame(screen)
	}
	if g.Help {
		g.drawHelp(screen)
	}
	if g.Capture != "" && !g.captured && g.Updates >= g.CaptureAfter {
		g.captured = true
		img := image.NewRGBA(image.Rect(0, 0, Width, Height))
		screen.ReadPixels(img.Pix)
		f, err := os.OpenFile(g.Capture, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			err = png.Encode(f, img)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		g.failure = err
	}
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	panel(screen, 90, 70, 460, 340)
	label(screen, "POPULOUS II", 238, 87, ink)
	button(screen, 470, 76, 105, 27, "DIEU  (G)", true, false)
	label(screen, "TRIALS OF THE OLYMPIAN GODS", 163, 111, muted)
	level := g.Bundle.Levels[g.LevelIndex]
	label(screen, fmt.Sprintf("MONDE %d  %s  /  DECOR %d", g.LevelIndex+1, level.Code, level.Terrain+1), 146, 139, ink)
	for i, text := range []string{"CONQUETE", "PARTIE LIBRE - TOUS LES POUVOIRS", "DEMONSTRATION", "CHARGER LA SAUVEGARDE"} {
		button(screen, 110, 168+i*46, 420, 38, text, true, false)
	}
	button(screen, 110, 352, 200, 32, "< MONDE PRECEDENT", true, false)
	button(screen, 330, 352, 200, 32, "MONDE SUIVANT >", true, false)
	label(screen, "ENTREE: jouer   D: demo   H: aide   F: plein ecran", 118, 391, muted)
	label(screen, "O : regles du monde", 118, 410, muted)
	if g.Message != "" {
		label(screen, g.Message, 12, 450, ink)
	}
}

func (g *Game) drawGame(screen *ebiten.Image) {
	world := g.World.Core
	label(screen, fmt.Sprintf("POPULOUS II  |  %s  |  MONDE %d", g.World.Level.Code, g.LevelIndex+1), 8, 6, ink)
	mode := "CONQUETE"
	if g.World.Custom {
		mode = "LIBRE"
	}
	if g.World.Demo {
		mode = "DEMO"
	}
	if g.Paused {
		mode += "  PAUSE"
	}
	label(screen, fmt.Sprintf("%s  CAMERA %02d,%02d  TOUR %d", mode, g.CameraX, g.CameraY, world.GameTurn), 8, 22, muted)
	g.drawMinimap(screen)
	populations := world.PlayerPopulations()
	label(screen, fmt.Sprintf("BLEU %7d", populations[0]), 466, 90, blue)
	label(screen, fmt.Sprintf("ROUGE%7d", populations[1]), 466, 114, red)
	label(screen, fmt.Sprintf("MANA %7d", world.Magnets[0].Mana), 466, 150, ink)
	view := screen.SubImage(image.Rect(128, 100, 640, 440)).(*ebiten.Image)
	terrain := g.World.Level.Terrain
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pos := g.CameraX + x + (g.CameraY+y)*64
			cell := g.World.TerrainCell(g.CameraX+x, g.CameraY+y)
			block := cell.Tile(world.GameTurn, x, y)
			px, py := project(x, y, cell.BaseAltitude)
			drawImage(view, g.tiles[terrain][block], px, py, 2)
			if overlay := world.MapBk2[pos]; overlay != 0 {
				sprite := 697
				if overlay >= legacy.TreeBlock && overlay < legacy.TreeBlock+3 {
					sprite = 696
				}
				g.drawSprite(view, sprite, px+32, py+48)
			}
			mark := g.World.Marks[pos]
			if mark.Life > 0 && mark.NativeTile == 0 {
				g.drawMark(view, mark, px, py)
			}
			if world.Magnets[0].GoTo == pos {
				g.drawSprite(view, 142, px+32, py+32)
			}
			if world.Magnets[1].GoTo == pos {
				g.drawSprite(view, 143, px+32, py+32)
			}
		}
	}
	for _, actor := range g.World.Scenery {
		if !actor.Active {
			continue
		}
		x, y := actor.X-g.CameraX, actor.Y-g.CameraY
		if x < 0 || y < 0 || x >= 8 || y >= 8 {
			continue
		}
		cell := g.World.TerrainCell(actor.X, actor.Y)
		px, py := project(x, y, cell.BaseAltitude)
		ox, oy := cell.ActorOffset(128, 128)
		frames := g.Bundle.Scenery.Frames[actor.Animation]
		if len(frames) == 0 {
			continue
		}
		frame := frames[actor.Frame%len(frames)]
		for _, layer := range frame.Layers {
			g.drawSprite(view, layer.Sprite, px+32+(ox+layer.X)*2, py+16+(oy+layer.Y)*2)
		}
	}
	for _, actor := range g.World.Walls.Actors {
		if !actor.Active {
			continue
		}
		x, y := actor.X-g.CameraX, actor.Y-g.CameraY
		if x < 0 || y < 0 || x >= 8 || y >= 8 {
			continue
		}
		cell := g.World.TerrainCell(actor.X, actor.Y)
		px, py := project(x, y, cell.BaseAltitude)
		ox, oy := cell.ActorOffset(128, 128)
		for _, layer := range g.World.WallRules.Layers(actor) {
			g.drawSprite(view, layer.Sprite, px+32+(ox+layer.X)*2, py+16+(oy+layer.Y)*2)
		}
	}
	for i, peep := range world.Peeps {
		// Followers are drawn after scenery and wall actor layers.
		if peep.Population <= 0 || peep.Flags&legacy.InRuin != 0 {
			continue
		}
		x, y := peep.AtPos%64-g.CameraX, peep.AtPos/64-g.CameraY
		if x < 0 || y < 0 || x >= 8 || y >= 8 {
			continue
		}
		cell := g.World.TerrainCell(peep.AtPos%64, peep.AtPos/64)
		px, py := project(x, y, cell.BaseAltitude)
		ox, oy := cell.ActorOffset(128, 128)
		cx, cy := px+32+ox*2, py+16+oy*2
		if peep.Plague && len(g.Bundle.PlagueAnimation) > 0 {
			frame := g.Bundle.PlagueAnimation[world.GameTurn%len(g.Bundle.PlagueAnimation)]
			for _, layer := range frame.Layers {
				g.drawSprite(view, layer.Sprite, cx+layer.X*2, cy+layer.Y*2)
			}
		}
		if peep.Flags&legacy.InTown != 0 {
			stage := clamp(peep.TownStage, 0, populous2.TownStages-1)
			sprite := 681 + stage
			g.drawSprite(view, sprite, cx, py+48)
			c := blue
			if peep.Player == 1 {
				c = red
			}
			vector.DrawFilledRect(view, float32(cx+2), float32(py+8), 3, 10, c, false)
		} else {
			base := 1
			if peep.Player == 1 {
				base = 49
			}
			sprite := base + directionIndex(peep.Direction)*3 + (world.GameTurn/2)%3
			if i < len(g.World.Heroes) && g.World.Heroes[i].Active {
				for _, layer := range g.Bundle.HeroRules.Layers(g.World.Heroes[i].Spell, directionIndex(peep.Direction), world.GameTurn) {
					g.drawSprite(view, layer.Sprite, cx+layer.X*2, cy+layer.Y*2)
				}
				continue
			}
			g.drawSprite(view, sprite, cx, cy)
		}
	}
	for _, effect := range g.World.Effects {
		x, y := effect.X-g.CameraX, effect.Y-g.CameraY
		if x < 0 || y < 0 || x >= 8 || y >= 8 {
			continue
		}
		px, py := project(x, y, int(world.MapAlt[effect.X+effect.Y*64]))
		sprite := 97 + world.GameTurn%8
		g.drawSprite(view, sprite, px+32, py+32)
	}
	for _, a := range g.World.NativeEffects {
		if !a.Active {
			continue
		}
		frame, ok := g.Bundle.NativeEffectFrame(a)
		if !ok {
			continue
		}
		wx, wy := int(a.X)>>8, int(a.Y)>>8
		x, y := wx-g.CameraX, wy-g.CameraY
		if x < 0 || y < 0 || x >= 8 || y >= 8 {
			continue
		}
		cell := g.World.TerrainCell(wx, wy)
		px, py := project(x, y, cell.BaseAltitude)
		ox, oy := cell.ActorOffset(uint8(a.X), uint8(a.Y))
		for _, layer := range frame.Layers {
			g.drawSprite(view, layer.Sprite, px+32+(ox+layer.X)*2, py+16+(oy+layer.Y)*2)
		}
	}
	for _, death := range g.World.FlameDeaths {
		x, y := death.X-g.CameraX, death.Y-g.CameraY
		if x < 0 || y < 0 || x >= 8 || y >= 8 {
			continue
		}
		cell := g.World.TerrainCell(death.X, death.Y)
		px, py := project(x, y, cell.BaseAltitude)
		ox, oy := cell.ActorOffset(128, 128)
		for _, layer := range g.Bundle.FireColumns.Frames[death.Animation].Layers {
			g.drawSprite(view, layer.Sprite, px+32+(ox+layer.X)*2, py+16+(oy+layer.Y)*2)
		}
	}
	if !g.World.Demo {
		x, y := ebiten.CursorPosition()
		mx, my, ok := g.targetAt(x, y)
		if ok {
			px, py := project(mx-g.CameraX, my-g.CameraY, int(world.MapAlt[mx+my*64]))
			diamond(view, px+32, py+32, color.RGBA{235, 235, 160, 255})
		}
	}
	for i, name := range []string{"PEU", "VEG", "TER", "AIR", "FEU", "EAU"} {
		button(screen, 4+(i%3)*41, 200+(i/3)*25, 39, 23, name, true, g.Category == populous2.Element(i))
	}
	for i, spell := range g.categorySpells() {
		cost := g.World.ManaCost(0, spell.ID)
		available := g.World.Available(0, spell.ID)
		button(screen, 4, spellButtonsY+i*spellButtonStep, 123, 32, spell.Name, available, g.Selected == spell.ID)
		c := muted
		if available && world.Magnets[0].Mana >= cost {
			c = gold
		}
		label(screen, fmt.Sprintf("%d mana", cost), 10, spellButtonsY+17+i*spellButtonStep, c)
	}
	label(screen, fmt.Sprintf("DIR %d  Q / E", g.Direction), 6, 423, muted)
	for i, name := range []string{"COLONISER", "RASSEMBLER", "COMBATTRE", "SUIVRE CHEF"} {
		button(screen, 136+i*87, 448, 83, 27, name, true, world.Magnets[0].Flags == []int{legacy.SettleMode, legacy.JoinMode, legacy.FightMode, legacy.MagnetMode}[i])
	}
	button(screen, 490, 448, 65, 27, "SAUVER", true, false)
	button(screen, 562, 448, 73, 27, "MENU", true, false)
	if g.messageTicks > 0 {
		vector.DrawFilledRect(screen, 128, 424, 512, 20, color.RGBA{10, 13, 18, 238}, false)
		label(screen, truncate(g.Message, 82), 132, 427, ink)
	}
	if result := world.ResultFor(0); result != legacy.ResultOngoing {
		panel(screen, 146, 235, 462, 118)
		text := "VICTOIRE"
		if result == legacy.ResultLost {
			text = "DEFAITE"
		}
		label(screen, text, 317, 254, ink)
		label(screen, "ENTREE: monde suivant / recommencer", 180, 290, muted)
		label(screen, "ECHAP: menu", 280, 320, muted)
	}
}

func (g *Game) drawSprite(screen *ebiten.Image, index, x, y int) {
	terrain := g.World.Level.Terrain
	if index < 0 || index >= len(g.sprites[terrain]) || g.sprites[terrain][index] == nil {
		return
	}
	sprite := g.Bundle.Sprites[terrain][index]
	drawImage(screen, g.sprites[terrain][index], x-sprite.AnchorX*2, y-sprite.AnchorY*2, 2)
}

func (g *Game) drawMark(screen *ebiten.Image, mark populous2.Mark, x, y int) {
	switch mark.Spell {
	case populous2.Trees:
		g.drawSprite(screen, 696, x+32, y+42)
	case populous2.Flowers:
		vector.DrawFilledCircle(screen, float32(x+32), float32(y+32), 4, gold, false)
	case populous2.Fungus:
		vector.DrawFilledCircle(screen, float32(x+32), float32(y+32), 6, color.RGBA{230, 90, 170, 255}, false)
	case populous2.Plague:
		diamond(screen, x+32, y+32, color.RGBA{160, 150, 145, 255})
	case populous2.Road, populous2.Basalt:
		vector.DrawFilledRect(screen, float32(x+20), float32(y+28), 24, 5, color.RGBA{200, 190, 150, 255}, false)
	case populous2.Wall:
		vector.DrawFilledRect(screen, float32(x+19), float32(y+15), 26, 16, color.RGBA{175, 162, 128, 255}, false)
	case populous2.FireColumn, populous2.FireRain:
		g.drawSprite(screen, 99+g.World.Core.GameTurn%4, x+32, y+32)
	}
}

func (g *Game) drawMinimap(screen *ebiten.Image) {
	clear(g.miniPixels)
	w := g.World.Core
	palette := g.World.Landscape.Palettes[0]
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			cell := g.World.TerrainCell(x, y)
			index := int(g.World.Landscape.MapColor[cell.Code]) & 15
			c := palette[index]
			mx, my := 64+x-y, (x+y)/2
			o := (my*128 + mx) * 4
			if mx >= 0 && mx < 128 && my < 64 {
				g.miniPixels[o], g.miniPixels[o+1], g.miniPixels[o+2], g.miniPixels[o+3] = c.R, c.G, c.B, 255
			}
		}
	}
	for _, p := range w.Peeps {
		if p.Population <= 0 {
			continue
		}
		if !g.World.FollowerVisibleOnMap(0, int(p.Player)) {
			continue
		}
		x, y := p.AtPos%64, p.AtPos/64
		mx, my := 64+x-y, (x+y)/2
		c := blue
		if p.Player == 1 {
			c = red
		}
		o := (my*128 + mx) * 4
		g.miniPixels[o], g.miniPixels[o+1], g.miniPixels[o+2], g.miniPixels[o+3] = c.R, c.G, c.B, 255
	}
	if g.World.EffectVisibleOnMap(0) {
		for _, a := range g.World.NativeEffects {
			if a.Active {
				g.minimapDot(int(a.X)>>8, int(a.Y)>>8, gold)
			}
		}
		for _, e := range g.World.Effects {
			g.minimapDot(e.X, e.Y, gold)
		}
	}
	g.minimap.WritePixels(g.miniPixels)
	drawImage(screen, g.minimap, 8, 56, 2)
	x, y := g.CameraX+3, g.CameraY+3
	diamond(screen, 8+(64+x-y)*2, 56+(x+y), ink)
}

func (g *Game) minimapDot(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= 64 || y >= 64 {
		return
	}
	mx, my := 64+x-y, (x+y)/2
	o := (my*128 + mx) * 4
	g.miniPixels[o], g.miniPixels[o+1], g.miniPixels[o+2], g.miniPixels[o+3] = c.R, c.G, c.B, 255
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	panel(screen, 45, 60, 550, 362)
	lines := []string{"COMMANDES", "", "Clic gauche / droit : lever / baisser le terrain", "Clic droit sur un logement : faire sortir un adorateur", "W A S D ou fleches : deplacer la vue", "Clic sur la carte : deplacer la vue", "Six boutons a gauche : choisir un element", "Puis choisir un pouvoir et cliquer sur le terrain", "Routes et murs : choisir deux extremites", "Q / E : choisir la direction des effets", "M : placer l'aimant   C : retrouver le chef", "1 : coloniser  2 : rassembler  3 : combattre  4 : suivre", "Espace : pause   F : plein ecran", "F5 : sauvegarder   F9 : charger", "H : fermer l'aide   Echap : menu", "", "Les pouvoirs autorises changent selon le monde."}
	for i, text := range lines {
		label(screen, text, 65, 78+i*19, ink)
	}
}

func (g *Game) save() {
	data, err := json.Marshal(g.World.Snapshot())
	if err == nil {
		var f *os.File
		f, err = os.CreateTemp(filepath.Dir(g.SavePath), ".populous2-save-*.tmp")
		if err == nil {
			name := f.Name()
			defer os.Remove(name)
			_, err = f.Write(data)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
			if err == nil {
				err = os.Rename(name, g.SavePath)
			}
		}
	}
	if err != nil {
		g.notify("Sauvegarde impossible: " + err.Error())
	} else {
		g.notify("Partie sauvegardee.")
	}
}

func (g *Game) load() {
	f, err := os.Open(g.SavePath)
	if err != nil {
		g.notify("Chargement impossible: " + err.Error())
		return
	}
	world, err := populous2.ReadSave(g.Bundle, f)
	f.Close()
	if err != nil {
		g.notify("Chargement impossible: " + err.Error())
		return
	}
	g.World = world
	g.LevelIndex = world.Level.Number
	g.Profile = world.Deity
	if world.Custom {
		g.hasCustomScenarioOptions = true
		for player, rules := range world.Rules {
			g.customScenarioOptions[player] = rules.Raw
		}
	}
	g.Playing = true
	g.Paused = false
	g.lineStart = nil
	g.Selected = populous2.RaiseLower
	g.centerPlayer(0)
	g.notify("Partie chargee.")
}

func (g *Game) notify(message string)      { g.Message = message; g.messageTicks = 48 }
func (g *Game) Layout(int, int) (int, int) { return Width, Height }

func (g *Game) Captured() bool { return g.captured }

var (
	pixelFace = text.NewGoXFace(basicfont.Face7x13)
	ink       = color.RGBA{234, 225, 197, 255}
	muted     = color.RGBA{162, 160, 148, 255}
	blue      = color.RGBA{68, 163, 242, 255}
	red       = color.RGBA{234, 91, 78, 255}
	gold      = color.RGBA{232, 193, 97, 255}
)

func label(screen *ebiten.Image, value string, x, y int, c color.Color) {
	if strings.TrimSpace(value) == "" {
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, value, pixelFace, op)
}

func button(screen *ebiten.Image, x, y, w, h int, text string, enabled, selected bool) {
	fill := color.RGBA{45, 43, 36, 250}
	border := color.RGBA{115, 102, 77, 255}
	c := ink
	if !enabled {
		fill = color.RGBA{29, 30, 30, 250}
		c = color.RGBA{112, 113, 111, 255}
	}
	if selected {
		border = gold
		fill = color.RGBA{73, 62, 32, 255}
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), fill, false)
	vector.StrokeRect(screen, float32(x)+.5, float32(y)+.5, float32(w)-1, float32(h)-1, 1, border, false)
	label(screen, text, x+5, y+4, c)
}

func panel(screen *ebiten.Image, x, y, w, h int) {
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{20, 23, 28, 247}, false)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, gold, false)
}
func drawImage(screen, img *ebiten.Image, x, y int, scale float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}
func diamond(screen *ebiten.Image, x, y int, c color.Color) {
	points := [5][2]float32{{float32(x), float32(y - 8)}, {float32(x + 16), float32(y)}, {float32(x), float32(y + 8)}, {float32(x - 16), float32(y)}, {float32(x), float32(y - 8)}}
	for i := 0; i < 4; i++ {
		vector.StrokeLine(screen, points[i][0], points[i][1], points[i+1][0], points[i+1][1], 1, c, false)
	}
}
func hit(x, y, bx, by, bw, bh int) bool { return x >= bx && x < bx+bw && y >= by && y < by+bh }
func directionIndex(delta int) int {
	d := []int{-64, -63, 1, 65, 64, 63, -1, -65}
	for i, v := range d {
		if delta == v {
			return i
		}
	}
	return 0
}
func clamp(n, lo, hi int) int { return max(lo, min(hi, n)) }
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}
