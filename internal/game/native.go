package game

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"

	embedded "go-populous2/assets"
	"go-populous2/internal/populous2"
)

// NativeGame uses the source register-bearing runtime rather than World.Tick.
// Unimplemented source UI/file/result ports remain explicit errors until the
// complete native path supplies their actual bodies.
type NativeGame struct {
	Host           *populous2.NativeRuntimeHost
	Director       populous2.NativeRuntimeDirector
	Rules          populous2.NativeStartupCampaignHostRules
	Startup        populous2.NativeRuntimeDirectorCallbacks
	Frame          *populous2.NativeRuntimeFrame
	Registers      populous2.NativeFrameRegisterContext
	Stream         *populous2.NativeRuntimePCMAccess
	Operations     populous2.NativeRuntimeAudioOperations
	Mouse          populous2.NativeHostMouse
	image          *ebiten.Image
	pixels         []byte
	player         *audio.Player
	booted         bool
	keys           map[ebiten.Key]bool
	Limit, Updates int
	Capture        string
	CaptureAfter   int
	captured       bool
	failure        error
	AutoStart      bool
	AutoMenuAction int
	autoClicked    bool
	beam           uint16
	Commands       *populous2.NativeRuntimeCommandChildren
	Network        *populous2.NativeNetworkEndpoint
	Transport      *populous2.NativeRuntimeTransport
	LocalResume    populous2.NativeTransportResumeState
	NetworkStartup *populous2.NativeRuntimeDirector
	networkRefresh bool
	Files          *populous2.NativeRuntimeFileStore
	FileBrowser    *populous2.NativeRuntimeFileBrowserState
	FileRules      populous2.NativeRuntimeFileBrowserRules
	DeityEditor    populous2.NativeRuntimeDeity
}

func NewNative(bundle *populous2.Bundle) (*NativeGame, error) {
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		return nil, err
	}
	low := make([]byte, 256)
	low[5] = 0xd0
	h, err := populous2.NewNativeRuntimeHost(bundle, files, populous2.NativeRuntimeHostConfig{HunkBases: []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000}, Regions: []populous2.NativeHostRegion{{Name: "portable Exec vector", Base: 0, Bytes: low}}, AllocationStart: 0x700000, AllocationLimit: 0x800000})
	if err != nil {
		return nil, err
	}
	rules, err := populous2.DecodeNativeStartupCampaignHostRules(bundle.Executable)
	if err != nil {
		h.Close()
		return nil, err
	}
	return &NativeGame{Host: h, Rules: rules, Registers: populous2.NativeFrameRegisterContext{AddressBase: 0x200000}, image: ebiten.NewImage(320, 200), pixels: make([]byte, 320*200*4), keys: make(map[ebiten.Key]bool)}, nil
}

func (g *NativeGame) boot() error {
	h := g.Host
	// Native DOS callbacks use this explicit portable library label; no
	// host OS structure is dereferenced by the source filesystem adapter.
	if err := h.Memory.BSS.Write32(0x14c, 0xc00000); err != nil {
		return err
	}
	if _, err := h.InitializePresentation(populous2.NativeMouseSample{}); err != nil {
		return err
	}
	complete, err := h.AdvanceAllocations(0x1a43e, &g.Registers, populous2.NativeErrorFrameCallbacks{})
	if err != nil {
		return err
	}
	if !complete {
		return fmt.Errorf("native initial allocation requires retained error input")
	}
	device, _, err := h.InitializeAudio(&g.Registers, 0)
	if err != nil {
		return err
	}
	stream, err := h.CreatePCM(device, populous2.NativePALAudioTiming(44100, 0), populous2.NativePALPaulaDMAConfig(0))
	if err != nil {
		return err
	}
	g.Stream = stream
	operations, err := stream.WithinExecuteCallbacks()
	if err != nil {
		return err
	}
	g.Operations = operations
	control := populous2.NativeAudioControlFrameCallbacks{Memory: h.Memory.BSS, Frame: &g.Registers, CodeBase: h.Memory.CodeBase, Command: operations.Command, MusicCommand: operations.MusicCommand}
	sound := func(cue uint16, c *populous2.NativeFrameRegisterContext) error { return operations.DirectCue(cue, c) }
	ownership := func(bool, *populous2.NativeFrameRegisterContext, *[7]populous2.NativeRequesterAddress) error {
		return nil
	}
	g.Startup = populous2.NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: populous2.NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: populous2.NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: populous2.NativeStartupResetFrameCallbacks{Hardware: func(populous2.NativeFrameHardwareWrite) error { return nil }}, Audio: &control}, Campaign: populous2.NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: populous2.NativeCampaignHelpFrameCallbacks{AudioCommand: operations.Command, AudioControl: control, NativeCampaignFrameCallbacks: populous2.NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: populous2.NativeFileFrameCallbacks{Sound: sound}}}}, Ownership: ownership}}
	g.booted = true
	g.Startup.MenuChild = g.initialChild
	return nil
}

func (g *NativeGame) createFrame() error {
	operations, h := g.Operations, g.Host
	sound := func(cue uint16, c *populous2.NativeFrameRegisterContext) error { return operations.DirectCue(cue, c) }
	g.Commands = &populous2.NativeRuntimeCommandChildren{Host: h, Rules: g.Rules, Supplied: g.Startup, Audio: populous2.NativeAudioControlFrameCallbacks{Command: operations.Command, MusicCommand: operations.MusicCommand, CodeBase: h.Memory.CodeBase}}
	g.Commands.Other = g.commandFileChild
	frame, err := h.NewFrame(populous2.NativeRuntimeFrameBindings{Audio: operations,
		RenderChildren: populous2.NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return g.beam, nil }, Ownership: func(bool, *populous2.NativeFrameRegisterContext) error { return nil }, Sound: sound},
		InputChildren:  populous2.NativeGameplayHUDHostCallbacks{Campaign: g.Startup.Campaign, Ownership: g.Startup.Ownership, Audio: operations},
		Menu:           populous2.NativeInGameHostCallbacks{Ownership: func(bool, *populous2.NativeFrameRegisterContext) error { return nil }, NativeFileFrameCallbacks: populous2.NativeFileFrameCallbacks{Sound: sound, Call: g.menuChild}, SerialTransport: g.serialChild},
		Session: populous2.NativeFrameSessionCallbacks{CommandChild: g.Commands.Call, Transport: func(caller int, mode uint8, c *populous2.NativeCommandRegisterContext, phase *uint32) (bool, error) {
			if err := g.networkReady(); err != nil {
				return false, err
			}
			if g.Transport == nil {
				return false, fmt.Errorf("native multiplayer command has no actual connection")
			}
			return g.Transport.PacketCallback(caller, mode, c, phase)
		}},
	})
	if err != nil {
		return err
	}
	g.Frame = frame
	return nil
}

func (g *NativeGame) Update() error {
	if g.failure != nil {
		return g.failure
	}
	g.Updates++
	if g.Limit > 0 && g.Updates >= g.Limit {
		return ebiten.Termination
	}
	err := g.Host.Access.Execute(func() error {
		if !g.booted {
			if err := g.boot(); err != nil {
				return err
			}
		}
		if err := g.pollKeys(); err != nil {
			return err
		}
		// Portable raster sampling occurs at the explicitly configured
		// VBlank edge. It is not a claim of original instruction bus phase.
		g.beam = 0
		x, y := ebiten.CursorPosition()
		left, right := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
		if (g.AutoStart || g.AutoMenuAction != 0) && g.Frame == nil && g.Updates >= 100 && !g.autoClicked {
			var err error
			action := g.AutoMenuAction
			if action == 0 {
				action = 6
			}
			x, y, err = g.nativeActionPosition(action)
			if err != nil {
				return err
			}
			left, right = true, false
		}
		sample := g.Mouse.Sample(&g.Host.Session.Presentation.Input, x, y, left, right)
		if (g.AutoStart || g.AutoMenuAction != 0) && g.Frame == nil && sample.Left {
			g.autoClicked = true
		}
		if _, err := g.Host.Session.Presentation.VBlank(sample, g.Host.Memory.BSS, &g.Registers); err != nil {
			return err
		}
		if g.Frame == nil {
			step, err := g.Director.Advance(g.Host, &g.Rules, &g.Registers, g.Startup)
			if err != nil {
				return err
			}
			if step.Complete {
				if g.FileBrowser != nil && g.FileBrowser.Files.RefreshPending {
					if err := g.FileBrowser.Files.RefreshLoadedViews(g.Host); err != nil {
						return err
					}
				}
				exit, err := g.Host.Memory.BSS.Read16(0x3aa)
				if err != nil {
					return err
				}
				if exit != 0 {
					return ebiten.Termination
				}
				if err := g.createFrame(); err != nil {
					return err
				}
			}
		} else {
			if g.Host.Session.Phase == populous2.NativeFrameSessionIdle {
				if err := g.Host.Session.BeginRaw(g.Host.World, g.Registers); err != nil {
					return err
				}
			}
			complete, err := g.Frame.Advance()
			if err != nil {
				return err
			}
			if complete {
				g.Registers = g.Host.Session.Frame
				if g.FileBrowser != nil && g.FileBrowser.Files.RefreshPending {
					if err := g.FileBrowser.Files.RefreshLoadedViews(g.Host); err != nil {
						return err
					}
					g.networkRefresh = true
				}
				if g.Commands.RefreshPending || g.networkRefresh {
					if err := g.Host.RefreshWorldCaches(); err != nil {
						return err
					}
					g.Commands.RefreshPending = false
					g.networkRefresh = false
					if err := g.createFrame(); err != nil {
						return err
					}
				}
				if g.Host.Session.ExitRequested {
					return ebiten.Termination
				}
			}
		}
		return g.Host.Session.Presentation.WriteRGBA(g.pixels, true)
	})
	if err != nil {
		return err
	}
	if g.player == nil {
		context := audio.CurrentContext()
		if context == nil {
			context = audio.NewContext(44100)
		}
		player, err := context.NewPlayer(g.Stream)
		if err != nil {
			return err
		}
		g.player = player
		g.player.Play()
	}
	g.image.WritePixels(g.pixels)
	return nil
}

func (g *NativeGame) Draw(screen *ebiten.Image) {
	screen.DrawImage(g.image, nil)
	if g.Capture != "" && !g.captured && g.Updates >= g.CaptureAfter {
		file, err := os.OpenFile(g.Capture, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			g.failure = err
			return
		}
		img := &image.RGBA{Pix: g.pixels, Stride: 320 * 4, Rect: image.Rect(0, 0, 320, 200)}
		err = png.Encode(file, img)
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
func (g *NativeGame) Layout(int, int) (int, int) { return 320, 200 }
func (g *NativeGame) Close() {
	if g.player != nil {
		g.player.Close()
	}
	if g.Transport != nil {
		g.Transport.Conn.Close()
	}
	if g.Network != nil {
		g.Network.Close()
	}
	if g.Files != nil {
		g.Files.Close()
	}
	g.Host.Close()
}
