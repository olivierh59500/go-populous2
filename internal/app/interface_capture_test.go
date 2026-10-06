package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"go-populous2/internal/engine"
)

// Optional local captures inspect the actual independent screen renderer.
// Existing files are never replaced and no original imagery is distributed.
func TestPrivateOriginalInterfaceCaptures(t *testing.T) {
	assetsPath, out := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR"), os.Getenv("POPULOUS2_INTERFACE_CAPTURE_DIR")
	if assetsPath == "" || out == "" {
		t.Skip("provide private portable assets and a new capture directory")
	}
	assets, err := LoadAssets(os.DirFS(assetsPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	g := &Game{Assets: assets, Profile: engine.NewDeity("PLAYER"), framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	world, err := engine.NewWorld(assets.Levels[0], assets.Landscapes[assets.Levels[0].Landscape])
	if err != nil {
		t.Fatal(err)
	}
	g.World = world
	for pass := 0; pass < 40; pass++ {
		world.Step()
	}
	leader := world.Players[0].Leader
	f := world.Followers[leader]
	g.CameraX, g.CameraY = max(0, min(56, int(f.X)-3)), max(0, min(56, int(f.Y)-3))
	g.SelectedFollower = leader
	for _, screen := range []struct {
		name   string
		screen Screen
	}{{"profile", DeityProfile}, {"conquest", ConquestBriefing}, {"game", Playing}, {"menu", InGameMenuScreen}} {
		g.Screen = screen.screen
		g.drawFrame()
		file, err := os.OpenFile(filepath.Join(out, screen.name+".png"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, g.framebuffer)
		closeErr := file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}
