package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
	"go-populous2/internal/music"
)

type Screen uint8

const (
	MainMenu Screen = iota
	DeityProfile
	ConquestBriefing
	Playing
	CampaignResult
)

// Game holds ordinary Go screen and input state. Original program counters,
// address registers and memory-controller continuations are not part of it.
type Game struct {
	Assets                       *Assets
	World                        *engine.World
	Screen                       Screen
	LevelIndex                   int
	CameraX, CameraY             int
	Profile                      engine.Deity
	ResultScore                  engine.CampaignScore
	ResultScoreError             string
	ResultProgress               engine.CampaignProgress
	resultApplied                bool
	resultAt                     int
	editingProfileName           bool
	editingProfileCode           bool
	profileCodeInput             string
	Selected                     engine.PowerID
	Category                     engine.Element
	PickingPower                 bool
	Direction                    uint8
	Message                      string
	messageUntil                 int
	Updates, Limit, CaptureAfter int
	Capture                      string
	AutoStart                    bool
	framebuffer                  *image.RGBA
	visibleFollowers             [viewSize * viewSize]int
	visibleNext                  [engine.FollowerCapacity]int
	image                        *ebiten.Image
	music                        *music.Player
	audio                        *audio.Player
	captured                     bool
	failure                      error
}

func New(assets *Assets) (*Game, error) {
	if assets == nil || assets.Visual == nil || len(assets.Levels) == 0 {
		return nil, fmt.Errorf("independent game assets are missing")
	}
	replay, err := music.NewPlayer(assets.Music, 44100)
	if err != nil {
		return nil, err
	}
	return &Game{Assets: assets, Profile: engine.NewDeity("BLUE"), framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200)), image: ebiten.NewImage(320, 200), music: replay}, nil
}

func (g *Game) Update() error {
	if g.failure != nil {
		return g.failure
	}
	g.Updates++
	if g.Limit > 0 && g.Updates >= g.Limit {
		return ebiten.Termination
	}
	if g.audio == nil {
		context := audio.CurrentContext()
		if context == nil {
			context = audio.NewContext(44100)
		}
		player, err := context.NewPlayer(g.music)
		if err != nil {
			return err
		}
		g.audio = player
		g.audio.Play()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Screen = MainMenu
	}
	if g.AutoStart && g.World == nil && g.Updates == 125 {
		if err := g.startConquest(); err != nil {
			return err
		}
	}
	x, y := ebiten.CursorPosition()
	clicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	switch g.Screen {
	case MainMenu:
		if clicked && x >= 78 && x < 245 {
			switch {
			case y >= 86 && y < 101:
				g.Screen = DeityProfile
			case y >= 102 && y < 117:
				g.Screen = ConquestBriefing
			}
		}
	case DeityProfile:
		g.updateProfile(x, y, clicked)
	case ConquestBriefing:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			g.LevelIndex = max(0, g.LevelIndex-1)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			g.LevelIndex = min(len(g.Assets.Levels)-1, g.LevelIndex+1)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || (clicked && x >= 115 && x < 215 && y >= 164 && y < 189) {
			if err := g.startConquest(); err != nil {
				return err
			}
		}
	case Playing:
		if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			g.PickingPower = !g.PickingPower
		}
		if err := g.updateWorld(x, y, clicked); err != nil {
			return err
		}
	case CampaignResult:
		if g.Updates-g.resultAt >= 101 && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || clicked) {
			if err := g.applyCampaignResult(); err != nil {
				return err
			}
		}
	}
	g.drawFrame()
	g.image.WritePixels(g.framebuffer.Pix)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.DrawImage(g.image, nil)
	if g.Capture != "" && !g.captured && g.Updates >= g.CaptureAfter {
		file, err := os.OpenFile(g.Capture, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			g.failure = err
			return
		}
		err = png.Encode(file, g.framebuffer)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			g.failure = err
			return
		}
		g.captured = true
	}
}
func (g *Game) Layout(int, int) (int, int) { return 320, 200 }
func (g *Game) Close() {
	if g.audio != nil {
		_ = g.audio.Close()
	}
}

func (g *Game) text(text string, x, y int) {
	palette := g.Assets.Visual.StartupPalette
	palette[0] = color.RGBA{40, 45, 18, 255}
	g.Assets.Visual.Font.Draw(g.framebuffer, text, x, y, palette)
}
func (g *Game) button(label string, x, y, width int) {
	draw.Draw(g.framebuffer, image.Rect(x, y, x+width, y+17), image.NewUniform(color.RGBA{95, 83, 23, 255}), image.Point{}, draw.Src)
	draw.Draw(g.framebuffer, image.Rect(x+2, y+2, x+width-2, y+15), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	g.text(label, x+(width-len(label)*8)/2, y+4)
}
func (g *Game) drawFrame() {
	switch g.Screen {
	case MainMenu:
		draw.Draw(g.framebuffer, g.framebuffer.Bounds(), g.Assets.Visual.Startup, image.Point{}, draw.Src)
		draw.Draw(g.framebuffer, image.Rect(73, 80, 249, 147), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
		g.button("CREATE YOUR DEITY", 78, 85, 168)
		g.button("CONQUEST", 78, 102, 168)
	case DeityProfile:
		g.drawProfile()
	case ConquestBriefing:
		draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
		g.text("CONQUEST", 128, 15)
		g.text(fmt.Sprintf("WORLD %d", g.LevelIndex), 24, 42)
		g.text(g.Assets.Levels[g.LevelIndex].Code, 24, 58)
		g.text("LEFT / RIGHT TO SELECT", 24, 74)
		g.button("PROCEED", 115, 168, 100)
	case Playing:
		g.drawWorld()
	case CampaignResult:
		g.drawCampaignResult()
	}
}
