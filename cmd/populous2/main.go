package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"go-populous2/internal/game"
	"go-populous2/internal/populous2"
)

func main() {
	world := flag.Int("world", 0, "campaign world index (0..999)")
	code := flag.String("code", "", "original world code, for example DOEGAC")
	demo := flag.Bool("demo", false, "start an AI-vs-AI demonstration")
	custom := flag.Bool("custom", false, "start a free game with every power available")
	play := flag.Bool("play", false, "start gameplay immediately")
	deity := flag.Bool("deity", false, "open the original deity profile editor")
	frames := flag.Int("frames", 0, "close after this number of updates (0: unlimited)")
	capture := flag.String("screenshot", "", "save the first drawn application frame to a new PNG")
	captureAfter := flag.Int("capture-update", 0, "minimum update count before saving the screenshot")
	width := flag.Int("width", 960, "window width")
	height := flag.Int("height", 720, "window height")
	flag.Parse()
	if *width < 320 || *height < 240 || *frames < 0 || *captureAfter < 0 || *frames > 0 && *captureAfter >= *frames {
		log.Fatal("invalid window size or update limit")
	}
	if *code != "" {
		index, ok := populous2.DecodeLevelCode(*code)
		if !ok {
			log.Fatal("unknown Populous II world code")
		}
		*world = index
	}
	bundle, err := populous2.Load()
	if err != nil {
		log.Fatal(err)
	}
	g, err := game.New(bundle, *world, *demo, *custom)
	if err != nil {
		log.Fatal(err)
	}
	if *play || *custom {
		g.Playing = true
	}
	if *deity {
		g.OpenDeity()
	}
	g.Limit = *frames
	g.Capture = *capture
	g.CaptureAfter = *captureAfter
	if path := os.Getenv("POPULOUS2_SAVE_PATH"); path != "" {
		g.SavePath = path
	}
	ebiten.SetWindowSize(*width, *height)
	ebiten.SetWindowTitle("Populous II - Go / Ebitengine")
	ebiten.SetTPS(60)
	if *frames > 0 {
		ebiten.SetRunnableOnUnfocused(true)
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
	if *capture != "" {
		if !g.Captured() {
			log.Fatal("the game stopped before drawing the requested screenshot")
		}
		fmt.Printf("Application frame: %s\n", *capture)
	}
}
