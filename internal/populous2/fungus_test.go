package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type fungusTraceActor struct {
	Index int
	Raw   string
}

type fungusTraceSnapshot struct {
	Tick    int
	Actors  []fungusTraceActor
	Pending [2]uint16
	Tiles   []uint8
}

type fungusTraceCase struct {
	Name       string
	Experience uint8
	Initial    []struct {
		X, Y int
		Tile uint8
	}
	ActorInitial []fungusTraceActor
	Actions      []struct{ Tick, Player, X, Y int }
	Snapshots    []fungusTraceSnapshot
}

func fungusTestRules(t *testing.T) FungusRules {
	t.Helper()
	rules, err := DecodeFungusRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func fungusTestTiles() [4096]uint8 {
	var tiles [4096]uint8
	for i := range tiles {
		tiles[i] = 15
	}
	return tiles
}

func fungusRaw(actor NativeEffectActor, bounds FungusBounds) []byte {
	raw := make([]byte, 32)
	raw[0] = actor.Kind
	binary.BigEndian.PutUint16(raw[6:], uint16(actor.X))
	binary.BigEndian.PutUint16(raw[8:], uint16(actor.Y))
	binary.BigEndian.PutUint16(raw[10:], uint16(actor.Animation))
	if actor.Active {
		raw[12] = actor.Player + 1
	}
	binary.BigEndian.PutUint16(raw[14:], uint16(actor.VX))
	binary.BigEndian.PutUint16(raw[16:], uint16(actor.VY))
	raw[18] = actor.Speed
	binary.BigEndian.PutUint16(raw[20:], uint16(actor.Timer))
	raw[22] = actor.State
	binary.BigEndian.PutUint16(raw[24:], uint16(actor.Life))
	raw[26], raw[27], raw[28], raw[29] = bounds.DX, bounds.DY, bounds.WorkDX, bounds.WorkDY
	return raw
}

func fungusLoadRaw(raw []byte) (NativeEffectActor, FungusBounds) {
	actor := NativeEffectActor{Kind: raw[0], Active: raw[12] != 0, X: int16(binary.BigEndian.Uint16(raw[6:])), Y: int16(binary.BigEndian.Uint16(raw[8:])), Animation: int(binary.BigEndian.Uint16(raw[10:])), VX: int16(binary.BigEndian.Uint16(raw[14:])), VY: int16(binary.BigEndian.Uint16(raw[16:])), Speed: raw[18], Timer: int16(binary.BigEndian.Uint16(raw[20:])), State: raw[22], Life: int16(binary.BigEndian.Uint16(raw[24:]))}
	if actor.Active {
		actor.Player = raw[12] - 1
	}
	return actor, FungusBounds{DX: raw[26], DY: raw[27], WorkDX: raw[28], WorkDY: raw[29]}
}

// These are relocated 68000 runs of $15fda/$1482e, not predictions generated
// by this Go engine. The full map and native controller bytes are compared.
func TestFungusAgainstOriginal68000Traces(t *testing.T) {
	data, err := os.ReadFile("testdata/fungus_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Cases []fungusTraceCase }
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	rules := fungusTestRules(t)
	for _, reference := range fixture.Cases {
		t.Run(reference.Name, func(t *testing.T) {
			tiles := fungusTestTiles()
			read := func(index int) uint8 { return tiles[index] }
			write := func(index int, tile uint8) { tiles[index] = tile }
			var pool [NativeEffectCapacity]NativeEffectActor
			var state FungusState
			for _, point := range reference.Initial {
				tiles[point.X+point.Y*64] = point.Tile
			}
			for _, initial := range reference.ActorInitial {
				raw, err := hex.DecodeString(initial.Raw)
				if err != nil || len(raw) != 32 {
					t.Fatalf("invalid native initial record: %v", err)
				}
				pool[initial.Index], state.Bounds[initial.Index] = fungusLoadRaw(raw)
			}
			next := 0
			for tick := 0; next < len(reference.Snapshots); tick++ {
				for _, action := range reference.Actions {
					if action.Tick == tick {
						rules.Create(&pool, &state, action.Player, action.X, action.Y, reference.Experience, read, write)
					}
				}
				if expected := reference.Snapshots[next]; expected.Tick == tick {
					if state.Pending != expected.Pending {
						t.Fatalf("tick %d pending %v, native %v", tick, state.Pending, expected.Pending)
					}
					if !bytes.Equal(tiles[:], expected.Tiles) {
						for index, tile := range expected.Tiles {
							if tiles[index] != tile {
								t.Fatalf("tick %d tile %d,%d=%d, native %d", tick, index%64, index/64, tiles[index], tile)
							}
						}
					}
					count := 0
					for _, actor := range pool {
						if actor.Kind == FungusActorKind {
							count++
						}
					}
					if count != len(expected.Actors) {
						t.Fatalf("tick %d actor count %d, native %d", tick, count, len(expected.Actors))
					}
					for _, actor := range expected.Actors {
						raw, err := hex.DecodeString(actor.Raw)
						if err != nil || len(raw) != 32 {
							t.Fatalf("invalid native snapshot record: %v", err)
						}
						if got := fungusRaw(pool[actor.Index], state.Bounds[actor.Index]); !bytes.Equal(got, raw) {
							t.Fatalf("tick %d slot %d record %x, native %x", tick, actor.Index, got, raw)
						}
					}
					next++
				}
				if next < len(reference.Snapshots) {
					for index := range pool {
						rules.Tick(&pool[index], &state, index, read, write)
					}
				}
			}
		})
	}
}

func TestFungusAllocationRecastAndSharedPool(t *testing.T) {
	rules := fungusTestRules(t)
	tiles := fungusTestTiles()
	read := func(index int) uint8 { return tiles[index] }
	write := func(index int, tile uint8) { tiles[index] = tile }
	var pool [NativeEffectCapacity]NativeEffectActor
	var state FungusState
	pool[0] = NativeEffectActor{Active: true, Kind: 0x22}
	pool[1] = NativeEffectActor{VX: 9, VY: -9, Life: 33, Animation: 0x660}
	first := rules.Create(&pool, &state, 0, 32, 32, 0, read, write)
	if !first.Planted || first.Reused || first.Slot != 1 || state.Pending[0] != 2 {
		t.Fatalf("first-free allocation: %+v pending %v", first, state.Pending)
	}
	if pool[1].VX != 9 || pool[1].VY != -9 || pool[1].Life != 33 || pool[1].Animation != 0x660 {
		t.Fatal("controller creation cleared unrelated recycled bytes")
	}
	for range 20 {
		rules.Tick(&pool[1], &state, 1, read, write)
	}
	timer, period := pool[1].Timer, pool[1].Speed
	recast := rules.Create(&pool, &state, 0, 30, 32, 255, read, write)
	if !recast.Reused || recast.Slot != 1 || pool[1].Timer != timer || pool[1].Speed != period {
		t.Fatal("recast did not retain collecting wait/period")
	}
	for range 80 {
		rules.Tick(&pool[1], &state, 1, read, write)
	}
	if state.Pending[0] != 0 || pool[1].State != FungusEvolving {
		t.Fatal("collecting pointer did not clear at evolution")
	}
	next := rules.Create(&pool, &state, 0, 42, 42, 255, read, write)
	if next.Slot != 2 || next.Reused || state.Pending[0] != 3 || pool[2].Speed != 3 {
		t.Fatal("additional evolving/collecting controllers cannot coexist")
	}
	other := rules.Create(&pool, &state, 1, 50, 50, 0, read, write)
	if other.Slot != 3 || state.Pending[1] != 4 {
		t.Fatal("other side did not get independent pending reference")
	}
	for index := range pool {
		pool[index].Active = true
	}
	state.Pending[1] = 0
	before := pool
	full := rules.Create(&pool, &state, 1, 55, 55, 0, read, write)
	if !full.Planted || full.Slot != -1 || tiles[55+55*64] != 145 || pool != before || state.Pending[1] != 0 {
		t.Fatal("full pool lost native plant-before-allocation behavior")
	}
}

func TestFungusBoundaryQuirkIsSafelyBounded(t *testing.T) {
	rules := fungusTestRules(t)
	tiles := fungusTestTiles()
	for _, x := range []int{9, 10, 11} {
		tiles[x+62*64] = 149
	}
	read := func(index int) uint8 {
		if index < 0 || index >= len(tiles) {
			t.Fatalf("unbounded read %d", index)
		}
		return tiles[index]
	}
	write := func(index int, tile uint8) {
		if index < 0 || index >= len(tiles) {
			t.Fatalf("unbounded write %d", index)
		}
		tiles[index] = tile
	}
	actor := NativeEffectActor{Active: true, Kind: FungusActorKind, X: packFungusCoordinates(8, 8), Y: packFungusCoordinates(60, 60), Speed: 10, State: FungusEvolving}
	var state FungusState
	state.Bounds[0] = FungusBounds{DX: 4, DY: 3, WorkDX: 4, WorkDY: 3}
	step := rules.Tick(&actor, &state, 0, read, write)
	if !step.Generated || state.Bounds[0].DY != 54 {
		t.Fatalf("original bottom clamp lost: %+v %+v", step, state.Bounds[0])
	}
	outside := step.OutsideReads
	for range 22 {
		outside += rules.Tick(&actor, &state, 0, read, write).OutsideReads
	}
	if outside == 0 {
		t.Fatal("native out-of-map rectangle was not reported")
	}
}

func TestFungusBytePhasePreservesTimerHighByteAndOtherActors(t *testing.T) {
	rules := fungusTestRules(t)
	tiles := fungusTestTiles()
	read := func(index int) uint8 { return tiles[index] }
	write := func(index int, tile uint8) { tiles[index] = tile }
	var state FungusState
	actor := NativeEffectActor{Active: true, Kind: FungusActorKind, X: packFungusCoordinates(30, 30), Y: packFungusCoordinates(30, 30), Speed: 10, State: FungusEvolving, Timer: 0x1205}
	state.Bounds[0] = FungusBounds{DX: 2, DY: 2, WorkDX: 2, WorkDY: 2}
	rules.Tick(&actor, &state, 0, read, write)
	if actor.Timer != 0x1204 {
		t.Fatalf("phase changed high byte: %04x", uint16(actor.Timer))
	}
	other := NativeEffectActor{Active: true, Kind: 0x22, Timer: 10, Life: 100, State: 4}
	before := other
	rules.Tick(&other, &state, 1, read, write)
	if other != before {
		t.Fatal("fungus controller touched another shared actor")
	}
}
