package app

import (
	"image"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/mobileui"
)

// The optional local art package measures actual bank composition instead of
// timing a substitute renderer or publishing original resources in tests.
func BenchmarkPrivateMobileScene(b *testing.B) {
	root := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if root == "" {
		b.Skip("set POPULOUS2_GENERATED_ASSETS_TEST_DIR to the exported art package")
	}
	assets, err := LoadAssets(os.DirFS(root))
	if err != nil {
		b.Fatal(err)
	}
	level := assets.Levels[0]
	w, err := engine.NewWorld(level, assets.Landscapes[level.Landscape])
	if err != nil {
		b.Fatal(err)
	}
	for range 40 {
		w.Step()
	}
	g := &Game{Assets: assets, World: w, Screen: Playing}
	leader := w.Followers[w.Players[0].Leader]
	g.CameraX, g.CameraY = max(0, int(leader.X)-3), max(0, int(leader.Y)-3)
	g.presentation.Capture(g, false)
	view := mobileui.Viewport{Rect: image.Rect(0, 24, 540, 196), CenterX: float64(g.CameraX) + 3.5, CenterY: float64(g.CameraY) + 3.5}
	dst := image.NewRGBA(image.Rect(0, 0, 540, 240))
	padding := mobileSpritePadding(assets)
	b.Logf("atlas padding=%d, parcel bounds=%v", padding, view.Bounds(engine.MapSize, engine.MapSize, 8, padding))
	b.Run("raw-main-frame", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			g.drawMobileScene(dst, view, padding)
		}
	})
	b.Run("cached-display-frame", func(b *testing.B) {
		var cache mobileSceneCache
		g.drawCachedMobileScene(dst, view, padding, &cache)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			g.drawCachedMobileScene(dst, view, padding, &cache)
		}
	})
}
