package app

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

func TestPrivateRepresentativeCampaignWorldsKeepValidContinuation(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("provide private portable campaign resources")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{0, 12, 27, 100, 250, 500, 999} {
		t.Run(fmt.Sprintf("world-%d", index), func(t *testing.T) {
			level := assets.Levels[index]
			w, err := engine.NewWorld(level, assets.Landscapes[level.Landscape])
			if err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 180; pass++ {
				w.Step()
				if w.Scenario.Err != "" {
					t.Fatalf("world event failed at pass %d: %s", pass, w.Scenario.Err)
				}
				if pass%30 != 29 {
					continue
				}
				restored, err := w.Snapshot().Restore()
				if err != nil {
					t.Fatalf("pass %d: %v", pass, err)
				}
				for range 4 {
					w.Step()
					restored.Step()
				}
				if !reflect.DeepEqual(w.Snapshot(), restored.Snapshot()) {
					t.Fatalf("world continuation diverged after pass %d", pass)
				}
			}
		})
	}
}
