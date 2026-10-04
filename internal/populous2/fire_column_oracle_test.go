package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

type nativeFireActor struct {
	Kind, Owner, Phase, Speed             uint8
	FixedX, FixedY, Animation             uint16
	VelocityX, VelocityY, MoveTimer, Life int16
}

type nativeFireStep struct {
	Tick        int
	Actor       nativeFireActor
	RNG         uint32
	TilesSHA256 string
}

type nativeFireFixture struct {
	Name               string
	Seed               uint32
	Experience         uint8
	Target             [2]int
	Accepted           bool
	Initial            nativeFireActor
	InitialRNG         uint32
	InitialTilesSHA256 string
	Trace              []nativeFireStep
}

// The checked-in traces execute the original instructions independently of
// this implementation. Every update compares the actor, full RNG state, and
// all 4096 logical terrain codes, including the final removal update.
func TestFireColumnAgainstOriginal68000Traces(t *testing.T) {
	data, err := os.ReadFile("testdata/fire_column_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []nativeFireFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 12 {
		t.Fatal("original fire-column fixture catalog is incomplete")
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fmt.Sprintf("%s-xp%d", fixture.Name, fixture.Experience), func(t *testing.T) {
			w := flatGroundWorld(t)
			w.NativeEffects = [NativeEffectCapacity]NativeEffectActor{}
			w.Scenery = [SceneryCapacity]SceneryActor{}
			w.rebuildSceneryIndex()
			w.Marks = [4096]Mark{}
			if fixture.Name == "water" {
				w.Core.Alt = [legacy.EndWidth * legacy.EndWidth]int{}
				w.Core.MapBlk = [4096]byte{}
			} else if fixture.Name == "uphill" {
				for y := 0; y <= 64; y++ {
					for x := 0; x <= 64; x++ {
						w.Core.Alt[x+y*65] = 1 + clamp(x-28, 0, 7)
					}
				}
			}
			core := w.Core.Snapshot()
			core.RNG = fixture.Seed
			w.Core = legacy.WorldFromSnapshot(core, w.Core.Rules)
			w.Experience[0][Fire] = fixture.Experience
			if got := w.Cast(0, FireColumn, Target{X: fixture.Target[0], Y: fixture.Target[1]}); got != fixture.Accepted {
				t.Fatalf("creation accepted=%t; original=%t", got, fixture.Accepted)
			}
			assertNativeFireState(t, w, fixture.Initial, fixture.InitialRNG, fixture.InitialTilesSHA256, 0)
			for _, step := range fixture.Trace {
				w.tickNativeEffects()
				assertNativeFireState(t, w, step.Actor, step.RNG, step.TilesSHA256, step.Tick)
			}
		})
	}
}

func assertNativeFireState(t *testing.T, w *World, want nativeFireActor, rng uint32, tilesHash string, tick int) {
	t.Helper()
	actor := w.NativeEffects[0]
	owner := uint8(0)
	if actor.Active {
		owner = actor.Player + 1
	}
	got := nativeFireActor{Kind: actor.Kind, Owner: owner, Phase: actor.State, Speed: actor.Speed, FixedX: uint16(actor.X), FixedY: uint16(actor.Y), Animation: uint16(actor.Animation), VelocityX: actor.VX, VelocityY: actor.VY, MoveTimer: actor.Timer, Life: actor.Life}
	if got != want {
		t.Fatalf("update %d actor mismatch:\nGo: %+v\n68000: %+v", tick, got, want)
	}
	if actual := w.Core.Snapshot().RNG; actual != rng {
		t.Fatalf("update %d RNG: Go %08x, native %08x", tick, actual, rng)
	}
	var tiles [4096]byte
	for index := range tiles {
		tiles[index] = w.nativeTileAt(index%64, index/64)
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(tiles[:])); actual != tilesHash {
		t.Fatalf("update %d terrain differs from the original: %s", tick, actual)
	}
}
