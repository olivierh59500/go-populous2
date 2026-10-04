package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

type nativeWhirlwindStep struct {
	Tick                            int
	Actor                           nativeFireActor
	RNG                             uint32
	TilesSHA256                     string
	Moved, SpawnWhirlpool, Finished bool
}

type nativeWhirlwindFixture struct {
	Name               string
	Seed               uint32
	Experience         uint8
	Target             [2]int
	Accepted           bool
	Initial            nativeFireActor
	InitialRNG         uint32
	InitialTilesSHA256 string
	Trace              []nativeWhirlwindStep
}

// These independent 68000 traces stop before advancing to the next effect
// record. The other 249 records are occupied: native child creation executes
// unchanged, but cannot allocate. Actor interactions are absent. This checks
// parent routing, animation, random draws, and child requests in isolation.
func TestWhirlwindAgainstOriginal68000Traces(t *testing.T) {
	data, err := os.ReadFile("testdata/whirlwind_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []nativeWhirlwindFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 12 {
		t.Fatal("original whirlwind fixture catalog is incomplete")
	}
	rules, err := DecodeWhirlwindRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fmt.Sprintf("%s-xp%d", fixture.Name, fixture.Experience), func(t *testing.T) {
			var pool [NativeEffectCapacity]NativeEffectActor
			var tiles [4096]byte
			terrain := func(x, y int) (int, uint8) {
				switch fixture.Name {
				case "water":
					return 0, 0
				case "uphill":
					left, right := 1+clamp(x-28, 0, 7), 1+clamp(x+1-28, 0, 7)
					if right > left {
						return left, 6
					}
					return left - 1, 15
				default:
					return 0, 15
				}
			}
			for index := range tiles {
				_, tiles[index] = terrain(index%64, index/64)
			}
			core := legacy.WorldFromSnapshot(legacy.WorldSnapshot{RNG: fixture.Seed}, legacy.TerrainRules{})
			assert := func(want nativeFireActor, rng uint32, hash string, tick int) {
				t.Helper()
				actor := pool[0]
				owner := uint8(0)
				if actor.Active {
					owner = actor.Player + 1
				}
				got := nativeFireActor{Kind: actor.Kind, Owner: owner, Phase: actor.State, Speed: actor.Speed, FixedX: uint16(actor.X), FixedY: uint16(actor.Y), Animation: uint16(actor.Animation), VelocityX: actor.VX, VelocityY: actor.VY, MoveTimer: actor.Timer, Life: actor.Life}
				if got != want {
					t.Fatalf("update %d actor mismatch:\nGo: %+v\n68000: %+v", tick, got, want)
				}
				if actual := core.Snapshot().RNG; actual != rng {
					t.Fatalf("update %d RNG: Go %08x, native %08x", tick, actual, rng)
				}
				if actual := fmt.Sprintf("%x", sha256.Sum256(tiles[:])); actual != hash {
					t.Fatalf("update %d terrain differs from the original: %s", tick, actual)
				}
			}
			if got := rules.Create(&pool, 0, fixture.Target[0], fixture.Target[1], fixture.Experience); got != fixture.Accepted {
				t.Fatalf("creation accepted=%t; original=%t", got, fixture.Accepted)
			}
			assert(fixture.Initial, fixture.InitialRNG, fixture.InitialTilesSHA256, 0)
			for _, expected := range fixture.Trace {
				step := rules.Tick(&pool[0], core.NextRandom, terrain)
				assert(expected.Actor, expected.RNG, expected.TilesSHA256, expected.Tick)
				if step.Moved != expected.Moved || step.SpawnWhirlpool != expected.SpawnWhirlpool || step.Finished != expected.Finished {
					t.Fatalf("update %d events: %+v; native moved=%t child=%t finished=%t", expected.Tick, step, expected.Moved, expected.SpawnWhirlpool, expected.Finished)
				}
				if step.Moved || step.Finished {
					if step.X != int(pool[0].X)>>8 || step.Y != int(pool[0].Y)>>8 {
						t.Fatalf("update %d event lost current cell: %+v", expected.Tick, step)
					}
				}
			}
		})
	}
}

func TestWhirlwindFirstFreeSlotRetainsVelocity(t *testing.T) {
	rules, err := DecodeWhirlwindRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	var pool [NativeEffectCapacity]NativeEffectActor
	for index := range pool {
		pool[index] = NativeEffectActor{Active: true, Kind: 0x22, X: 123, Y: 234}
	}
	before := pool
	if rules.Create(&pool, 0, 32, 32, 255) || pool != before {
		t.Fatal("full shared pool was changed by rejected creation")
	}
	pool[17].Active, pool[17].VX, pool[17].VY = false, 91, -27
	if !rules.Create(&pool, 1, 32, 32, 255) {
		t.Fatal("reusable shared slot was rejected")
	}
	actor := pool[17]
	if actor.VX != 91 || actor.VY != -27 || actor.Player != 1 || actor.Life != 455 {
		t.Fatalf("native reused-slot state was lost: %+v", actor)
	}
	for index := range pool {
		if index != 17 && pool[index] != before[index] {
			t.Fatal("creation changed a different shared-pool actor")
		}
	}
}
