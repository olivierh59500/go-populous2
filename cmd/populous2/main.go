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
	scenarioRules := flag.Bool("rules", false, "open the per-side scenario rules")
	fireColumns := flag.Bool("fire-columns", false, "present three native fire columns near the current camera")
	whirlwinds := flag.Bool("whirlwinds", false, "present three native whirlwinds near the camera with diagnostic attrition/water overrides")
	simulationRate := flag.Int("simulation-rate", populous2.SimulationRate, "simulation updates per second (nominal PAL: 50)")
	frames := flag.Int("frames", 0, "close after this number of updates (0: unlimited)")
	capture := flag.String("screenshot", "", "save the first drawn application frame to a new PNG")
	captureAfter := flag.Int("capture-update", 0, "minimum update count before saving the screenshot")
	width := flag.Int("width", 960, "window width")
	height := flag.Int("height", 720, "window height")
	flag.Parse()
	if *width < 320 || *height < 240 || *frames < 0 || *captureAfter < 0 || *frames > 0 && *captureAfter >= *frames || *simulationRate < 1 || *simulationRate > 120 {
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
	g.SetSimulationRate(*simulationRate)
	if *play || *custom {
		g.Playing = true
	}
	if *deity {
		g.OpenDeity()
	}
	if *scenarioRules {
		g.OpenScenarioRules()
	}
	if *fireColumns || *whirlwinds {
		g.Playing = true
		g.World.Custom = true
		g.World.Core.Magnets[0].Mana = 1000000
		g.World.Core.Computer[1].Mode = 0
		if *whirlwinds {
			// This bounded art/controller presentation is separate from conquest
			// pacing: keep followers alive while inspecting the native effects.
			g.World.Core.FollowerAttrition = func(int, bool) int { return 0 }
			for player := range g.World.Rules {
				g.World.Rules[player] = populous2.DecodeScenarioRules(g.World.Rules[player].Raw &^ (1 << 5))
			}
			g.Category, g.Selected = populous2.Air, populous2.Whirlwind
		}
		for _, p := range [][2]int{{3, 3}, {4, 4}, {5, 3}} {
			target := populous2.Target{X: g.CameraX + p[0], Y: g.CameraY + p[1]}
			if *fireColumns {
				g.World.Cast(0, populous2.FireColumn, target)
			}
			if *whirlwinds {
				g.World.Cast(0, populous2.Whirlwind, target)
			}
		}
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
