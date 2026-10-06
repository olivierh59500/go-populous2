// Command populous2-go launches the independent Go engine during migration.
package main

import (
	"flag"
	"io/fs"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	runtimeassets "go-populous2/assets/runtime"
	"go-populous2/internal/app"
)

func main() {
	data := flag.String("data", "", "external asset directory (default: assets embedded by the local build)")
	frames := flag.Int("frames", 0, "close after a bounded number of PAL updates")
	screenshot := flag.String("screenshot", "", "write a new application framebuffer PNG")
	captureAfter := flag.Int("capture-update", 100, "PAL update to capture")
	autoStart := flag.Bool("auto-start", false, "start the first conquest after presenting the menu")
	listen := flag.String("listen", "", "host a two-player session on this TCP address")
	connect := flag.String("connect", "", "join a two-player session at this TCP address")
	savePath := flag.String("save", "go-populous2-go.json", "save/load path for the independent Go game")
	flag.Parse()
	if *frames < 0 || *captureAfter < 0 || (*frames > 0 && *screenshot != "" && *captureAfter >= *frames) {
		log.Fatal("invalid update or capture limit")
	}
	var files fs.FS
	if *data != "" {
		files = os.DirFS(*data)
	} else {
		var err error
		files, err = runtimeassets.FS()
		if err != nil {
			log.Fatal(err)
		}
	}
	bundle, err := app.LoadAssets(files)
	if err != nil {
		log.Fatal(err)
	}
	game, err := app.New(bundle)
	if err != nil {
		log.Fatal(err)
	}
	defer game.Close()
	if err := game.ConfigureNetwork(*listen, *connect); err != nil {
		log.Fatal(err)
	}
	game.Limit = *frames
	game.Capture = *screenshot
	game.CaptureAfter = *captureAfter
	game.AutoStart = *autoStart
	game.SavePath = *savePath
	ebiten.SetTPS(50)
	ebiten.SetWindowSize(960, 600)
	ebiten.SetWindowTitle("Populous II - Independent Go Engine")
	if *frames > 0 {
		ebiten.SetRunnableOnUnfocused(true)
	}
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
