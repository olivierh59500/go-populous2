package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"go-populous2/internal/engine"
	"go-populous2/internal/gamcodec"
	"go-populous2/internal/mobileui"
	"go-populous2/internal/music"
	"go-populous2/internal/platformbridge"
)

type Screen uint8

const (
	MainMenu Screen = iota
	DeityProfile
	ConquestBriefing
	Playing
	CampaignResult
	EndingScreen
	NetworkSetup
	OptionsScreen
	EditorScreen
	HelpScreen
	PowerHelpScreen
	SaveBrowserScreen
	InGameMenuScreen
	AboutScreen
)

// Game holds ordinary Go screen and input state. Original program counters,
// address registers and memory-controller continuations are not part of it.
type Game struct {
	mobile                       *MobileState
	sceneProjection              *mobileui.Viewport
	bluetooth                    *platformbridge.Bridge
	bluetoothAvailable           *atomic.Bool
	Assets                       *Assets
	World                        *engine.World
	OriginalSave                 *gamcodec.Document
	LocalSide                    int
	Network                      *NetworkController
	Options                      *OptionsState
	Editor                       *EditorState
	PowerPreview                 *PowerPreview
	CustomLevel                  *engine.Level
	CustomComputer               [2]bool
	CustomGame                   bool
	connectionAddress            string
	networkHosting               bool
	editingConnection            bool
	Screen                       Screen
	LevelIndex                   int
	CameraX, CameraY             int
	Profile                      engine.Deity
	AnimationSounds              AnimationSoundGate
	ResultScore                  engine.CampaignScore
	ResultScoreError             string
	ResultProgress               engine.CampaignProgress
	Ending                       *EndingPlayback
	resultApplied                bool
	resultAt                     int
	editingProfileName           bool
	editingProfileCode           bool
	profileCodeInput             string
	profileReturn                Screen
	profileCaret                 int
	editingWorldCode             bool
	worldCodeInput               string
	briefingOpponent             bool
	Selected                     engine.PowerID
	SelectedFollower             int
	heroScanCursor               int
	effectScanCursor             int
	Inspecting                   bool
	selectionTransferTick        uint64
	SelectionReturn              FollowerSelectionReturn
	Category                     engine.Element
	PickingPower                 bool
	Paused                       bool
	helpReturn                   Screen
	Direction                    uint8
	Message                      string
	messageUntil                 int
	Updates, Limit, CaptureAfter int
	Capture                      string
	SavePath                     string
	SaveBrowser                  *SaveBrowser
	menuReturn                   Screen
	controlMode                  uint8
	presentation                 PlayingPresentation
	AutoStart                    bool
	framebuffer                  *image.RGBA
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
	if err := assets.requireOriginalInterface(); err != nil {
		return nil, err
	}
	replay, err := music.NewPlayer(assets.Music, 44100)
	if err != nil {
		return nil, err
	}
	return &Game{Assets: assets, Profile: engine.NewDeity("BLUE"), framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200)), image: ebiten.NewImage(320, 200), music: replay, bluetooth: platformbridge.New(), bluetoothAvailable: new(atomic.Bool)}, nil
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
	g.updateBluetooth()
	if err := g.pollNetwork(); err != nil {
		g.Message, g.messageUntil = err.Error(), g.Updates+250
	}
	if g.mobile != nil {
		return g.updateMobileFrame()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.Screen == PowerHelpScreen {
			g.closePowerHelp()
		} else if g.Screen == HelpScreen {
			g.Screen = g.helpReturn
		} else if g.Screen == AboutScreen {
			g.Screen = InGameMenuScreen
		} else if g.Screen == DeityProfile {
			if !g.editingProfileName && !g.editingProfileCode {
				g.closeDeityProfile()
			}
		} else if g.Screen == ConquestBriefing && g.editingWorldCode {
			// Original text modals ignore Escape; Enter accepts their buffer.
		} else if g.Screen == ConquestBriefing && g.briefingOpponent {
			g.briefingOpponent = false
		} else if g.Screen == Playing {
			if err := g.openInGameMenu(); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+100
			}
		} else if g.Screen == InGameMenuScreen {
			g.Screen = g.menuReturn
		} else if g.Screen == SaveBrowserScreen {
			g.closeSaveBrowser()
		} else if g.Screen == EditorScreen {
			if g.Editor != nil && g.Editor.EditingField != "" {
				g.Editor.EditingField, g.Editor.NumberInput = "", ""
			} else {
				g.cancelEditor()
			}
		} else if g.Screen == OptionsScreen {
			g.cancelOptions()
		} else {
			if g.Network != nil {
				g.Network.Close()
				g.Network = nil
			}
			g.Screen = MainMenu
		}
	}
	if g.AutoStart && g.World == nil && g.Updates == 125 {
		if err := g.startConquest(); err != nil {
			return err
		}
	}
	x, y := ebiten.CursorPosition()
	clicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if inpututil.IsKeyJustPressed(ebiten.KeyH) && g.Screen == Playing {
		g.helpReturn, g.Screen = g.Screen, HelpScreen
	}
	switch g.Screen {
	case MainMenu:
		if inpututil.IsKeyJustPressed(ebiten.KeyM) {
			g.openNetworkSetup()
			break
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyH) {
			g.helpReturn, g.Screen = MainMenu, HelpScreen
			break
		}
		if clicked {
			if handled, quit, err := g.handleOriginalStartup(x, y); handled {
				if err != nil {
					g.Message, g.messageUntil = err.Error(), g.Updates+150
				}
				if quit {
					return ebiten.Termination
				}
				break
			}
		}
		if clicked && x >= 110 && x < 214 && y >= 175 && y < 192 {
			if err := g.openSaveBrowser(false); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+150
			}
		}
		if clicked && x >= 78 && x < 245 {
			switch {
			case y >= 86 && y < 101:
				g.openDeityProfile(MainMenu)
			case y >= 102 && y < 117:
				g.CustomGame = false
				g.Screen = ConquestBriefing
			case y >= 120 && y < 137:
				g.openNetworkSetup()
			case y >= 138 && y < 155:
				if err := g.startCustomGame(); err != nil {
					g.Message, g.messageUntil = err.Error(), g.Updates+100
				}
			case y >= 156 && y < 173:
				g.helpReturn, g.Screen = MainMenu, HelpScreen
			}
		}
	case DeityProfile:
		g.updateProfile(x, y, clicked)
	case ConquestBriefing:
		if err := g.updateBriefing(x, y, clicked); err != nil {
			g.Message, g.messageUntil = err.Error(), g.Updates+150
		}
	case Playing:
		if inpututil.IsKeyJustPressed(ebiten.KeyI) {
			g.Inspecting = !g.Inspecting
		}
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.Network == nil {
			g.Paused = !g.Paused
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyO) {
			if err := g.openOptions(); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+100
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			if err := g.openEditor(); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+100
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
			if err := g.openSaveBrowser(false); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+200
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
			if err := g.openSaveBrowser(true); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+200
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			g.PickingPower = !g.PickingPower
		}
		if g.Screen != Playing {
			break
		}
		if err := g.updateWorld(x, y, clicked); err != nil {
			return err
		}
	case CampaignResult:
		if g.Updates-g.resultAt >= 101 && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || clicked && g.resultContinueHit(x, y)) {
			if err := g.applyCampaignResult(); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+150
			}
		}
	case EndingScreen:
		g.Ending.Update()
	case NetworkSetup:
		g.updateNetworkSetup(x, y, clicked)
	case OptionsScreen:
		if err := g.updateOptions(x, y, clicked); err != nil {
			g.Message, g.messageUntil = err.Error(), g.Updates+100
		}
	case EditorScreen:
		if err := g.updateEditor(x, y, clicked); err != nil {
			g.Message, g.messageUntil = err.Error(), g.Updates+100
		}
	case HelpScreen:
		if clicked && y >= 155 && y < 173 {
			if err := g.openPowerHelp(g.Selected); err != nil {
				g.Message, g.messageUntil = err.Error(), g.Updates+100
			}
		} else if clicked || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.Screen = g.helpReturn
		}
	case PowerHelpScreen:
		if err := g.updatePowerHelp(x, y, clicked); err != nil {
			g.Message, g.messageUntil = err.Error(), g.Updates+100
		}
	case SaveBrowserScreen:
		g.updateSaveBrowser(x, y, clicked)
	case InGameMenuScreen:
		if clicked {
			if g.Assets.InGameLayout != nil {
				if err := g.applyInGameAction(g.Assets.InGameLayout.ActionAt(x, y)); err != nil {
					g.Message, g.messageUntil = err.Error(), g.Updates+150
				}
			}
		}
	case AboutScreen:
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || clicked && (g.Assets.AboutLayout == nil || g.Assets.AboutLayout.ActionAt(x, y) == "resume") {
			g.Screen = InGameMenuScreen
		}
	}
	g.drawFrame()
	g.updateSystemPointerVisibility()
	g.drawGamePointer(x, y)
	if g.Screen == EditorScreen {
		g.drawEditorPointer(x, y)
	}
	g.drawRequesterPointer(x, y)
	g.image.WritePixels(g.framebuffer.Pix)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	frame := g.framebuffer
	if g.mobile != nil {
		screen.DrawImage(g.mobile.Image, nil)
		frame = g.mobile.Frame
	} else {
		screen.DrawImage(g.image, nil)
	}
	if g.Capture != "" && !g.captured && g.Updates >= g.CaptureAfter {
		file, err := os.OpenFile(g.Capture, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			g.failure = err
			return
		}
		err = png.Encode(file, frame)
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
func (g *Game) Layout(width, height int) (int, int) {
	if g.mobile != nil {
		w, h := mobileui.LogicalSize(width, height)
		g.mobile.resize(w, h)
		return w, h
	}
	return 320, 200
}
func (g *Game) Close() {
	if g.bluetooth != nil {
		g.bluetooth.Stop()
	}
	if g.Network != nil {
		g.Network.Close()
	}
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
		if g.drawOriginalStartup() {
			break
		}
		draw.Draw(g.framebuffer, g.framebuffer.Bounds(), g.Assets.Visual.Startup, image.Point{}, draw.Src)
		draw.Draw(g.framebuffer, image.Rect(73, 80, 249, 147), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
		g.button("CREATE YOUR DEITY", 78, 85, 168)
		g.button("CONQUEST", 78, 102, 168)
		g.button("MULTIPLAYER", 78, 120, 168)
		g.button("CUSTOM GAME", 78, 138, 168)
		g.button("HELP", 78, 156, 168)
		g.button("LOAD GAME", 110, 175, 104)
		if g.Updates < g.messageUntil {
			g.drawMessage(0)
		}
	case DeityProfile:
		g.drawProfile()
	case ConquestBriefing:
		g.drawBriefing()
	case Playing:
		if g.presentation.Ready {
			renderer := g.presentation.Renderer(g)
			renderer.drawWorld()
			g.AnimationSounds = renderer.AnimationSounds
		} else {
			g.drawWorld()
		}
	case CampaignResult:
		g.drawCampaignResult()
	case EndingScreen:
		g.drawEnding()
	case NetworkSetup:
		g.drawNetworkSetup()
	case OptionsScreen:
		g.drawOptions()
	case EditorScreen:
		g.drawEditor()
	case HelpScreen:
		g.drawHelp()
	case PowerHelpScreen:
		g.drawPowerHelp()
	case SaveBrowserScreen:
		g.drawSaveBrowser()
	case InGameMenuScreen:
		g.drawInGameMenu()
	case AboutScreen:
		g.drawAbout()
	}
}
