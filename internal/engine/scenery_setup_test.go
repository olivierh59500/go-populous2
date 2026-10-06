package engine

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"

	"go-populous2/internal/populous2"
)

func TestStartupSceneryIsDeterministicAndUsesSharedPool(t *testing.T) {
	a, b := testFlatWorld(), testFlatWorld()
	a.random, b.random = 4311, 4311
	a.SetupScenery()
	b.SetupScenery()
	if a.Nature.Scenery != b.Nature.Scenery || a.random != b.random {
		t.Fatal("startup scenery is nondeterministic")
	}
	trees, rocks := 0, 0
	for id, actor := range a.Nature.Scenery {
		if actor.Kind == SceneryNone {
			continue
		}
		if actor.Kind == SceneryTree {
			trees++
		} else if actor.Kind == SceneryBoulder {
			rocks++
		}
		if actor.Age != 24 || actor.Variant > 3 || !inside(int(actor.X), int(actor.Y)) {
			t.Fatal("invalid startup decoration")
		}
		if _, _, linked := a.Actors.Position(ActorRef{Kind: ActorScenery, Index: uint16(id)}); !linked {
			t.Fatal("startup scenery is absent from mixed actor membership")
		}
	}
	if trees == 0 || rocks == 0 || trees+rocks > SceneryCapacity {
		t.Fatal("startup lost a scenery family or exceeded capacity")
	}
}

func TestStartupSceneryRejectsWaterAndExistingObjects(t *testing.T) {
	w := &World{random: 4311}
	w.SetupScenery()
	for _, actor := range w.Nature.Scenery {
		if actor.Kind != SceneryNone {
			t.Fatal("startup planted scenery on open water")
		}
	}
	w = testFlatWorld()
	w.random = 4311
	for at := range w.Occupants {
		w.Occupants[at] = 1
	}
	w.SetupScenery()
	for _, actor := range w.Nature.Scenery {
		if actor.Kind != SceneryNone {
			t.Fatal("startup overwrote occupied parcels")
		}
	}
}

func TestPrivateStartupSceneryAgainstVerifiedInitialization(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set POPULOUS2_EXPORT_TEST_DIR for private scenery initialization comparison")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	backing := func(data []byte) populous2.FollowerCleanupMemory {
		span := func(at, n int) error {
			if at < 0 || at > len(data)-n {
				return fmt.Errorf("private scenery backing outside bounds")
			}
			return nil
		}
		return populous2.FollowerCleanupMemory{
			Read8: func(at int) (uint8, error) {
				if err := span(at, 1); err != nil {
					return 0, err
				}
				return data[at], nil
			},
			Read16: func(at int) (uint16, error) {
				if err := span(at, 2); err != nil {
					return 0, err
				}
				return binary.BigEndian.Uint16(data[at:]), nil
			},
			Read32: func(at int) (uint32, error) {
				if err := span(at, 4); err != nil {
					return 0, err
				}
				return binary.BigEndian.Uint32(data[at:]), nil
			},
			Write8: func(at int, v uint8) error {
				if err := span(at, 1); err != nil {
					return err
				}
				data[at] = v
				return nil
			},
			Write16: func(at int, v uint16) error {
				if err := span(at, 2); err != nil {
					return err
				}
				binary.BigEndian.PutUint16(data[at:], v)
				return nil
			},
			Write32: func(at int, v uint32) error {
				if err := span(at, 4); err != nil {
					return err
				}
				binary.BigEndian.PutUint32(data[at:], v)
				return nil
			},
		}
	}
	for _, seed := range []uint32{0, 1, 4311, 5038, 9999, 32767, 65535, 0x12345678} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			w := testFlatWorld()
			w.random = randomState(seed)
			raw := make([]byte, 0x18000)
			for at := range MapSize * MapSize {
				raw[0xf44+at*4+1] = 15
			}
			binary.BigEndian.PutUint32(raw[0xeb28:], seed)
			c := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
			c.D[2] = uint32(0xffffff9d)
			var a [7]populous2.NativeRequesterAddress
			cb := populous2.NativeStartupResetFrameCallbacks{Frame: &c, Code: backing(source.Executable.Hunks[0].Data), Memory: backing(raw), CodeBase: 0x100000}
			for _, routine := range []int{0xd9d8, 0xdbd4} {
				if _, err := populous2.RunNativeStartupWorldFrame(routine, cb, &a); err != nil {
					t.Fatal(err)
				}
			}
			w.SetupScenery()
			if uint32(w.random) != binary.BigEndian.Uint32(raw[0xeb28:]) {
				t.Fatal("startup scenery RNG differs", w.random, binary.BigEndian.Uint32(raw[0xeb28:]))
			}
			for id, got := range w.Nature.Scenery {
				at := 0x6bd0 + id*14
				if (got.Kind != SceneryNone) != (raw[at+12] != 0) {
					t.Fatal("startup scenery allocation differs", id)
				}
				if got.Kind == SceneryNone {
					continue
				}
				kind := SceneryTree
				variants := source.Scenery.Trees.Animations
				if raw[at] == 0x18 {
					kind = SceneryBoulder
					variants = source.Scenery.Boulders.Animations
				}
				variant := -1
				art := int(binary.BigEndian.Uint16(raw[at+10:]))
				for n, a := range variants {
					if a == art {
						variant = n
						break
					}
				}
				if got.Kind != kind || got.X != raw[at+6] || got.Y != raw[at+8] || int(got.Variant) != variant || uint8(got.Age) != raw[at+1] {
					t.Fatal("startup scenery semantic state differs", id, got, kind, raw[at+6], raw[at+8], variant)
				}
			}
		})
	}
}
