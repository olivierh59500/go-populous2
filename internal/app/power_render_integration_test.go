package app

import (
	"image"
	"os"
	"testing"

	"go-populous2/internal/engine"
)

func TestPrivateEveryPowerRendersWithPortableArtwork(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("set private portable artwork directory")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, power := range engine.Powers {
		t.Run(power.Name, func(t *testing.T) {
			preview, err := newPowerPreview(assets, power.ID)
			if err != nil {
				t.Fatal(err)
			}
			g := &Game{Assets: assets, World: preview.World, CameraX: preview.CameraX, CameraY: preview.CameraY, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
			for pass := 0; pass < 120; pass++ {
				g.World.Step()
				g.drawWorld()
			}
			if _, err := g.World.Snapshot().Restore(); err != nil {
				t.Fatal("rendered power left an invalid world", err)
			}
		})
	}
}
