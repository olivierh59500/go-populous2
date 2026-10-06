package app

import (
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

// Exercise complete effect lifetimes and saved continuation, not just the
// casting flag. The two worlds must retain identical actor order and RNG.
func TestEveryPowerSurvivesFullLifetimeAndSavedContinuation(t *testing.T) {
	assets := menuTestGame(t).Assets
	for _, power := range engine.Powers {
		t.Run(power.Name, func(t *testing.T) {
			preview, err := newPowerPreview(assets, power.ID)
			if err != nil {
				t.Fatal(err)
			}
			original := preview.World
			for step := 0; step < 420; step++ {
				original.Step()
				if step != 19 && step != 99 && step != 299 {
					continue
				}
				continued, err := original.Snapshot().Restore()
				if err != nil {
					t.Fatalf("pass %d has invalid continuation: %v", step, err)
				}
				for range 12 {
					original.Step()
					continued.Step()
				}
				if !reflect.DeepEqual(original.Snapshot(), continued.Snapshot()) {
					t.Fatalf("saved power diverged after pass %d", step)
				}
			}
			if _, err := original.Snapshot().Restore(); err != nil {
				t.Fatal("completed lifecycle has invalid state", err)
			}
		})
	}
}
