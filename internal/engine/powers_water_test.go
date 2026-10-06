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

func TestWhirlpoolNeedsFourPristineWaterTilesAndSharesBudget(t *testing.T) {
	w := &World{random: 4311}
	w.Players[0].Experience[Water] = 255
	before := w.random
	if err := w.CastWhirlpool(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if w.random != before || w.Water.Whirlpools[0].Life != 555 {
		t.Fatal("whirlpool creation consumed RNG or lost experience lifetime")
	}
	for quadrant, d := range whirlpoolFootprint {
		if w.Cell(32+d[0], 32+d[1]).Code != uint8(152+quadrant) {
			t.Fatal("whirlpool quadrant art differs")
		}
	}
	if err := w.CastWhirlpool(0, 32, 32); err == nil {
		t.Fatal("existing animated water accepted another whirlpool")
	}
	if err := w.CastWhirlpool(0, 63, 63); err == nil {
		t.Fatal("out-of-map footprint accepted")
	}
	land := testFlatWorld()
	if err := land.CastWhirlpool(0, 32, 32); err == nil {
		t.Fatal("whirlpool accepted dry land")
	}
}

func TestWhirlpoolExpiryRestoresOnlyItsOwnFrame(t *testing.T) {
	w := &World{}
	if err := w.CastWhirlpool(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	w.Water.Whirlpools[0].Life = 1
	w.paintWater(33, 32, 224)
	w.tickWaterEffects()
	if w.Water.Whirlpools[0].Active || w.effects.Slots[0].Kind != EffectNone {
		t.Fatal("expired whirlpool retained effect slot")
	}
	if w.Cell(32, 32).Code != 0 || w.Cell(33, 32).Code != 224 {
		t.Fatal("whirlpool cleared unrelated permanent terrain")
	}
}

func TestWhirlpoolLowersFirstCoastQuadrantWithoutSpendingMana(t *testing.T) {
	w := &World{random: 4311}
	if err := w.CastWhirlpool(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	// Turn its first quadrant into a raised coastline while retaining three
	// currently stamped water quadrants; the effect only handles this shore.
	for _, p := range [4][2]int{{32, 32}, {33, 32}, {32, 33}, {33, 33}} {
		w.Heights[p[0]+p[1]*CornerSize] = 1
	}
	w.rebuildCells()
	w.Water.Painted[32+32*MapSize] = false
	w.Players[0].Mana = 1000
	w.tickWaterEffects()
	if w.Players[0].Mana != 1000 {
		t.Fatal("natural whirlpool lowering charged a player")
	}
	for _, p := range [4][2]int{{32, 32}, {33, 32}, {32, 33}, {33, 33}} {
		if w.Heights[p[0]+p[1]*CornerSize] != 0 {
			t.Fatal("coastline corner remained raised")
		}
	}
	if w.Cell(32, 32).Code != 156 {
		t.Fatal("lowered shore did not receive current whirlpool frame")
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

func TestPrivateWhirlpoolWaterTraces(t *testing.T) {
	path := os.Getenv("POPULOUS2_WHIRLPOOL_TRACE")
	if path == "" {
		t.Skip("set POPULOUS2_WHIRLPOOL_TRACE for private whirlpool comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type cell struct {
		Index        int
		Height, Tile uint8
		Head         uint16
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name, Mode        string
				X, Y, Occupied    int
				Owner, Experience uint8
				Seed              uint32
				Life, Timer       int16
				Animation         uint16
				InitialCells      []cell
				RawOffset         uint16
				RawValue          uint8
			}
			Accepted       bool
			Slot           int
			InitialActor   struct{ Raw string }
			InitialRNG     uint32
			InitialChanges []cell
			Trace          []struct {
				Tick    int
				Actor   struct{ Raw string }
				RNG     uint32
				Changes []cell
				Lowers  []struct {
					X, Y    int
					Changes []cell
				}
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, reference := range catalog.Cases {
		input := reference.Input
		if input.Owner < 1 || input.Owner > 2 || input.Mode == "tick" || input.RawOffset != 0 {
			continue
		}
		coast := false
		for _, frame := range reference.Trace {
			if len(frame.Lowers) > 0 {
				coast = true
			}
		}
		if coast {
			continue
		}
		t.Run(input.Name, func(t *testing.T) {
			w := &World{random: randomState(input.Seed)}
			w.Players[input.Owner-1].Experience[Water] = input.Experience
			for range input.Occupied {
				w.allocateEffect(EffectFireRain, 0)
			}
			for _, c := range input.InitialCells {
				w.Tiles[c.Index] = Cell{Code: c.Tile}
			}
			err := w.CastWhirlpool(int(input.Owner-1), input.X, input.Y)
			if (err == nil) != reference.Accepted || uint32(w.random) != reference.InitialRNG {
				t.Fatal("whirlpool admission or RNG differs")
			}
			compare := func(rawString string, cells []cell) {
				raw, err := hex.DecodeString(rawString)
				if err != nil || len(raw) != 32 {
					t.Fatal("invalid private whirlpool record")
				}
				e := w.Water.Whirlpools[reference.Slot]
				if e.Active != (raw[12] != 0) {
					t.Fatal("whirlpool lifetime active state differs")
				}
				if e.Active && (e.X != int(raw[6]) || e.Y != int(raw[8]) || e.Life != int(int16(binary.BigEndian.Uint16(raw[24:]))) || e.Delay != int(int16(binary.BigEndian.Uint16(raw[20:]))) || e.Frame != (int(binary.BigEndian.Uint16(raw[10:]))-151)/4) {
					t.Fatalf("whirlpool semantic state differs: %+v", e)
				}
				for _, c := range cells {
					if got := w.Cell(c.Index%MapSize, c.Index/MapSize).Code; got != c.Tile {
						t.Fatalf("whirlpool parcel%d =%d expected%d", c.Index, got, c.Tile)
					}
				}
			}
			if reference.Accepted {
				compare(reference.InitialActor.Raw, reference.InitialChanges)
			}
			next := 0
			for tick := 1; next < len(reference.Trace); tick++ {
				w.tickWaterEffects()
				if expected := reference.Trace[next]; expected.Tick == tick {
					compare(expected.Actor.Raw, expected.Changes)
					if uint32(w.random) != expected.RNG {
						t.Fatal("whirlpool movement random sequence differs")
					}
					next++
				}
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("whirlpool reference catalog supplied no normal cases")
	}
}
