package main

import (
	"flag"
	"github.com/hajimehoshi/ebiten/v2"
	"go-populous2/internal/game"
	"go-populous2/internal/populous2"
	"log"
)

func main() {
	frames := flag.Int("frames", 0, "close after a bounded number of native updates")
	capture := flag.String("screenshot", "", "save a native application framebuffer PNG to a new path")
	captureAfter := flag.Int("capture-update", 100, "native update to capture")
	autoStart := flag.Bool("auto-start", false, "diagnostic: click the original custom-game menu button")
	listen := flag.String("listen", "", "listen for native serial bytes over TCP at this address")
	connect := flag.String("connect", "", "connect native serial bytes over TCP to this address")
	flag.Parse()
	bundle, err := populous2.Load()
	if err != nil {
		log.Fatal(err)
	}
	g, err := game.NewNative(bundle)
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()
	if err := g.SetNetwork(*listen, *connect); err != nil {
		log.Fatal(err)
	}
	g.Limit = *frames
	g.Capture, g.CaptureAfter = *capture, *captureAfter
	g.AutoStart = *autoStart
	ebiten.SetTPS(50)
	ebiten.SetWindowSize(960, 600)
	ebiten.SetWindowTitle("Populous II - Native Go runtime")
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	if *frames > 0 {
		ebiten.SetRunnableOnUnfocused(true)
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
