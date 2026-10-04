package populous2

import (
	"crypto/sha256"
	"fmt"
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestBatholithAgainstOriginal68000Routine(t *testing.T) {
	for _, ref := range []struct {
		seed, rng       uint32
		hash            string
		x, y, animation int
	}{
		{0, 0x7714b9fe, "d705eb09403358892d6eb41d50ec79ee50eb84f138fb79021fba252140673dcc", 0, 0, 0},
		{1, 0xa374e3e9, "f725e681fa541e31b43ca00a34f89092267859c5a74cdcec1e7f9a5a61357ecb", 0, 0, 0},
		{4311, 0x11428447, "85eea23a3bc05659cedf6f34e7ab1f0f22eab7533f1ac82092727b9f1b08631a", 35, 32, 2792},
		{5038, 0x4ff81a8e, "85eea23a3bc05659cedf6f34e7ab1f0f22eab7533f1ac82092727b9f1b08631a", 29, 29, 2800},
	} {
		w := flatGroundWorld(t)
		w.Scenery = [SceneryCapacity]SceneryActor{}
		w.rebuildSceneryIndex()
		s := w.Core.Snapshot()
		s.RNG = ref.seed
		w.Core = legacy.WorldFromSnapshot(s, w.Core.Rules)
		if !w.Cast(0, Batholith, Target{X: 32, Y: 32}) {
			t.Fatal("native batholith cast rejected")
		}
		var heights [4225]byte
		for i, h := range w.Core.Alt {
			heights[i] = byte(h)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(heights[:])); got != ref.hash || w.Core.Snapshot().RNG != ref.rng {
			t.Fatalf("batholith seed %d differs: %s/%08x", ref.seed, got, w.Core.Snapshot().RNG)
		}
		if ref.animation != 0 {
			a := w.Scenery[0]
			if !a.Active || a.Kind != SceneryBoulder || a.X != ref.x || a.Y != ref.y || a.Animation != ref.animation {
				t.Fatal("native batholith boulder differs")
			}
		} else if w.Scenery[0].Active {
			t.Fatal("raise branch added an invented boulder")
		}
	}
}
