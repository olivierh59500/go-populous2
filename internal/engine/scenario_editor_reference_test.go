package engine

import (
	"encoding/binary"
	"os"
	"testing"

	"go-populous2/internal/populous2"
)

func TestPrivateNeutralEditorGroundEffectsMatchOriginalSamplesAndRNG(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original reference directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ScenarioEventKind{ScenarioBaptism, ScenarioSwamp, ScenarioFlowers} {
		for _, seed := range []uint32{1, 4311, 0x123456, 0xffffabcd} {
			w := testFlatWorld()
			w.Editor = false
			w.random = randomState(seed)
			raw := make([]byte, 0x11280)
			for cell := 0; cell < MapSize*MapSize; cell++ {
				c := w.Cell(cell%64, cell/64)
				raw[0xf44+cell*4] = c.BaseAltitude
				raw[0xf45+cell*4] = c.Code
			}
			memory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil }, Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { binary.BigEndian.PutUint16(raw[at:], v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil }}
			random := randomState(seed)
			rng := func() uint16 { return random.next() }
			if kind == ScenarioFlowers {
				_, err = source.RenewNative.Create(3, 32, 32, populous2.RenewNativeCallbacks{Memory: memory, Random: rng})
			} else {
				id := populous2.Baptism
				if kind == ScenarioSwamp {
					id = populous2.Swamp
				}
				_, err = source.NativeGround.Create(id, 3, 32, 32, populous2.NativeGroundCallbacks{Memory: memory, Random: rng})
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := w.executeScenarioEvent(ScenarioEvent{Kind: kind, X: 32, Y: 32}); err != nil {
				t.Fatal(err)
			}
			if w.random != random {
				t.Fatal("neutral editor sampling RNG differs", kind, seed, w.random, random)
			}
			for y := 0; y < MapSize; y++ {
				for x := 0; x < MapSize; x++ {
					if code := w.Cell(x, y).Code; code != raw[0xf45+(x+y*64)*4] {
						t.Fatalf("neutral editor ground differs kind%d seed%d at%d,%d: %d/%d", kind, seed, x, y, code, raw[0xf45+(x+y*64)*4])
					}
				}
			}
		}
	}
}

func TestPrivateNeutralEditorWhirlpoolCreatorMatchesOriginal(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original reference directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	w := testFlatWorld()
	w.Editor = false
	var water [CornerSize * CornerSize]uint8
	if err := w.EditorSetTerrain(water); err != nil {
		t.Fatal(err)
	}
	var pool [populous2.NativeEffectCapacity]populous2.NativeEffectActor
	grid := [MapSize * MapSize]uint8{}
	ok := source.Whirlpools.Create(&pool, 2, 30, 30, 255, func(x, y int) uint8 { return grid[x+y*64] }, func(x, y int, c uint8) { grid[x+y*64] = c })
	if !ok {
		t.Fatal("original neutral whirlpool fixture rejected")
	}
	if err := w.executeScenarioEvent(ScenarioEvent{Kind: ScenarioWhirlpool, X: 30, Y: 30}); err != nil {
		t.Fatal(err)
	}
	if w.Water.Whirlpools[0].Life != int(pool[0].Life) || w.Water.Whirlpools[0].Owner != 2 {
		t.Fatal("neutral whirlpool used a player experience tier")
	}
	for cell, c := range grid {
		if w.Cell(cell%64, cell/64).Code != c {
			t.Fatal("neutral whirlpool original footprint differs")
		}
	}
}

func TestPrivateNeutralTidalAndWindRecordsMatchOriginal(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original reference directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ScenarioEventKind{ScenarioTsunami, ScenarioWind} {
		raw := make([]byte, 0x11280)
		memory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil }, Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { binary.BigEndian.PutUint16(raw[at:], v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil }}
		w := testFlatWorld()
		w.Editor = false
		var water [CornerSize * CornerSize]uint8
		if err := w.EditorSetTerrain(water); err != nil {
			t.Fatal(err)
		}
		event := ScenarioEvent{Kind: kind, X: 30, Y: 30, Direction: 1}
		if kind == ScenarioTsunami {
			_, err = source.TsunamiRules.Create(3, 30, 30, populous2.TsunamiCallbacks{Memory: memory, Link: func(populous2.NativeRecordReference) error { return nil }})
		} else {
			_, err = source.HurricaneRules.Create(3, 30, 30, 2, populous2.HurricaneCallbacks{Memory: memory})
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := w.executeScenarioEvent(event); err != nil {
			t.Fatal(err)
		}
		if kind == ScenarioWind {
			e := w.Wind[0]
			if int(raw[0xc806]) != e.X || int(raw[0xc808]) != e.Y || int(binary.BigEndian.Uint16(raw[0xc818:])) != e.Life || int(binary.BigEndian.Uint16(raw[0xc81a:]))/2 != e.Direction || e.Owner != 2 {
				t.Fatal("neutral wind creator differs from source record")
			}
		} else {
			for id := 0; id < 4; id++ {
				e := w.Water.Waves[id]
				at := 0xc800 + id*32
				if !e.Active || e.Owner != 2 || e.X != int(binary.BigEndian.Uint16(raw[at+6:])) || e.Y != int(binary.BigEndian.Uint16(raw[at+8:])) {
					t.Fatal("neutral tidal creator differs from source", id, e)
				}
			}
		}
	}
}
