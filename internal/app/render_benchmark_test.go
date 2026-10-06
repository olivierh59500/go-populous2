package app

import (
	"image"
	"os"
	"testing"

	"go-populous2/internal/engine"
)

func BenchmarkPrivateWorldRenderer(b *testing.B) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		b.Skip("provide private portable artwork")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		b.Fatal(err)
	}
	level := assets.Levels[12]
	world, err := engine.NewWorld(level, assets.Landscapes[level.Landscape])
	if err != nil {
		b.Fatal(err)
	}
	for range 160 {
		world.Step()
	}
	leader := world.Followers[world.Players[0].Leader]
	g := &Game{Assets: assets, World: world, CameraX: max(0, min(56, int(leader.X)-3)), CameraY: max(0, min(56, int(leader.Y)-3)), framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.drawWorld()
	}
}
