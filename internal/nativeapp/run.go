package nativeapp

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"go-populous2/internal/game"
	"go-populous2/internal/populous2"
)

func Run() {
	frames := flag.Int("frames", 0, "close after a bounded number of native updates")
	capture := flag.String("screenshot", "", "save a native application framebuffer PNG to a new path")
	captureAfter := flag.Int("capture-update", 100, "native update to capture")
	autoStart := flag.Bool("auto-start", false, "diagnostic: click the original custom-game menu button")
	autoAction := flag.Int("auto-menu-action", 0, "diagnostic: click a numbered original menu action")
	listen := flag.String("listen", "", "listen for native serial bytes over TCP at this address")
	connect := flag.String("connect", "", "connect native serial bytes over TCP to this address")
	saveRoot := flag.String("save-root", "", "existing directory used by the original file requester (default: working directory)")
	exportRoot := flag.String("export-root", "", "existing directory for original editor ILBM screen exports")
	originalProtection := flag.Bool("original-protection", false, "enable the original manual statue challenge")
	unpaced := flag.Bool("unpaced", false, "diagnostic: admit source work at every PAL update")
	flag.Parse()
	if *frames < 0 || *captureAfter < 0 || *frames > 0 && *capture != "" && *captureAfter >= *frames {
		log.Fatal("invalid native update or capture limit")
	}
	if *saveRoot == "" {
		var err error
		*saveRoot, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}
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
	if *saveRoot != "" {
		*saveRoot, err = filepath.Abs(*saveRoot)
		if err != nil {
			log.Fatal(err)
		}
		if err := g.SetSaveRoot(*saveRoot); err != nil {
			log.Fatal(err)
		}
	}
	if *exportRoot != "" {
		if err := g.SetScreenExportRoot(*exportRoot); err != nil {
			log.Fatal(err)
		}
	}
	g.Limit = *frames
	g.Capture, g.CaptureAfter = *capture, *captureAfter
	g.AutoStart = *autoStart
	g.AutoMenuAction = *autoAction
	g.OriginalProtection = *originalProtection
	g.Unpaced = *unpaced
	ebiten.SetTPS(50)
	ebiten.SetWindowSize(960, 600)
	ebiten.SetWindowTitle("Populous II - Go / Ebitengine")
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	if *frames > 0 {
		ebiten.SetRunnableOnUnfocused(true)
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
