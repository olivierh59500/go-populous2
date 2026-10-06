package engine

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestBasaltAdmissionKeepsResourcesOnFailure(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	before := w.random
	if w.CreateBasalt(0, 32, 32, 0, 100) || w.random != before || w.Water.Painted[32+32*MapSize] {
		t.Fatal("land incorrectly received basalt or rejection consumed randomness")
	}
	water := &World{random: 4311}
	for range EffectCapacity {
		water.allocateEffect(EffectFireColumn, 0)
	}
	before = water.random
	if water.CreateBasalt(0, 32, 32, 0, 100) || water.random != before || water.Water.Painted[32+32*MapSize] {
		t.Fatal("exhausted effect budget changed terrain or randomness")
	}
}

func TestBasaltPropagatesWithRemainingLifeAndLeavesTerrain(t *testing.T) {
	w := &World{random: 4311}
	if !w.CreateBasalt(0, 32, 32, 1, 100) {
		t.Fatal("water rejected basalt")
	}
	id := 0
	delay := w.Water.Basalt[id].Delay
	for range delay - 1 {
		w.tickWaterEffects()
	}
	if !w.Water.Basalt[id].Active || w.Water.Painted[33+32*MapSize] {
		t.Fatal("basalt propagated before delay")
	}
	w.tickWaterEffects()
	if w.Water.Basalt[id].Active || !w.Water.Basalt[1].Active || w.Water.Basalt[1].Life != 100-delay-1 {
		t.Fatalf("parent/child ordering differs: %+v", w.Water.Basalt[1])
	}
	if code, ok := w.Water.TileCode(32, 32); !ok || code != 224 {
		t.Fatal("controller removal discarded basalt ground")
	}
	w.ClearWaterTerrain(32, 32)
	if _, ok := w.Water.TileCode(32, 32); !ok {
		t.Fatal("landscape sculpting discarded persistent basalt")
	}
	for range 150 {
		w.tickWaterEffects()
	}
	for _, e := range w.Water.Basalt {
		if e.Active {
			t.Fatal("basalt exceeded inherited life")
		}
	}
}

func TestBasaltParentReservesBudgetUntilChildAdmission(t *testing.T) {
	w := &World{random: 4311}
	for range EffectCapacity - 1 {
		w.allocateEffect(EffectFireRain, 0)
	}
	if !w.CreateBasalt(0, 32, 32, 1, 100) {
		t.Fatal("final slot did not accept parent")
	}
	id := EffectCapacity - 1
	w.Water.Basalt[id].Delay = 1
	w.tickWaterEffects()
	if w.Water.Basalt[id].Active || w.Water.Painted[33+32*MapSize] {
		t.Fatal("child incorrectly reused still-live parent slot")
	}
	if next := w.allocateEffect(EffectFungus, 0); next != id {
		t.Fatal("parent did not release its shared slot")
	}
}

func TestPrivateBasaltReferenceTraces(t *testing.T) {
	path := os.Getenv("POPULOUS2_BASALT_TRACE")
	if path == "" {
		t.Skip("set POPULOUS2_BASALT_TRACE for private basalt comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type cell struct {
		Index        int
		Header, Tile uint8
		Head         uint16
	}
	type actor struct {
		Index int
		Raw   string
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name, Mode   string
				X, Y         int
				Owner        uint8
				Life         int16
				Direction    uint16
				Occupied     int
				Seed         uint32
				Tile, Header uint8
				Follower     bool
				Ticks        int
				Obstacles    []cell
			}
			Accepted      bool
			InitialActors []actor
			InitialCells  []cell
			InitialRNG    uint32
			Trace         []struct {
				Tick   int
				Actors []actor
				Cells  []cell
				RNG    uint32
			}
		}
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, reference := range catalog.Cases {
		input := reference.Input
		if input.Owner < 1 || input.Owner > 2 || input.Direction > 6 || input.Direction%2 != 0 {
			continue
		}
		t.Run(input.Name, func(t *testing.T) {
			w := &World{random: randomState(input.Seed)}
			for i := range input.Occupied {
				w.allocateEffect(EffectFireRain, 0)
				_ = i
			}
			setCell := func(c cell) {
				w.Tiles[c.Index] = Cell{Code: c.Tile}
				if c.Head != 0 {
					w.Occupants[c.Index] = 1
				}
			}
			if inside(input.X, input.Y) {
				setCell(cell{Index: input.X + input.Y*MapSize, Tile: input.Tile})
			}
			for _, c := range input.Obstacles {
				setCell(c)
			}
			got := w.CreateBasalt(input.Owner-1, input.X, input.Y, int(input.Direction/2), int(input.Life))
			if got != reference.Accepted || uint32(w.random) != reference.InitialRNG {
				t.Fatalf("creation acceptance/RNG differs got%v/%d expected%v/%d", got, w.random, reference.Accepted, reference.InitialRNG)
			}
			compare := func(actors []actor, cells []cell) {
				for _, a := range actors {
					raw, err := hex.DecodeString(a.Raw)
					if err != nil || len(raw) != 32 {
						t.Fatal("invalid private basalt record")
					}
					e := w.Water.Basalt[a.Index]
					if e.Active != (raw[12] != 0) {
						t.Fatal("basalt active state differs")
					}
					if !e.Active {
						continue
					}
					if e.X != int(raw[6]) || e.Y != int(raw[8]) || e.Life != int(int16(binary.BigEndian.Uint16(raw[24:]))) || e.Delay != int(int16(binary.BigEndian.Uint16(raw[20:]))) || e.Frame != int(binary.BigEndian.Uint16(raw[10:])-0x5ec)/4 {
						t.Fatalf("basalt semantic state differs: %+v", e)
					}
				}
				for _, c := range cells {
					if w.Cell(c.Index%MapSize, c.Index/MapSize).Code != c.Tile {
						t.Fatalf("basalt terrain differs parcel%d", c.Index)
					}
				}
			}
			compare(reference.InitialActors, reference.InitialCells)
			next := 0
			for tick := 1; next < len(reference.Trace); tick++ {
				w.tickWaterEffects()
				if expected := reference.Trace[next]; expected.Tick == tick {
					compare(expected.Actors, expected.Cells)
					if uint32(w.random) != expected.RNG {
						t.Fatal("propagation random state differs")
					}
					next++
				}
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("private basalt catalog provided no valid reference cases")
	}
}
