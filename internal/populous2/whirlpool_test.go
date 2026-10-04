package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeWhirlpoolCell struct {
	Index        int
	Height, Tile uint8
	Head         uint16
}

type nativeWhirlpoolInput struct {
	Name, Mode        string
	X, Y, Occupied    int
	Owner, Experience uint8
	Seed              uint32
	Ticks             int
	Life, Timer       int16
	Animation         uint16
	ViewX, ViewY      int
	InitialCells      []nativeWhirlpoolCell
	RawOffset         uint16
	RawValue          uint8
}

type nativeWhirlpoolActor struct {
	Raw string
}

type nativeWhirlpoolLower struct {
	X, Y    int
	Changes []nativeWhirlpoolCell
}

type nativeWhirlpoolTick struct {
	Tick       int
	Actor      nativeWhirlpoolActor
	RNG        uint32
	Changes    []nativeWhirlpoolCell
	GridSHA256 string
	Lowers     []nativeWhirlpoolLower
	SoundCue   bool
}

type nativeWhirlpoolCase struct {
	Input             nativeWhirlpoolInput
	Accepted          bool
	Slot              int
	InitialActor      nativeWhirlpoolActor
	InitialRNG        uint32
	InitialChanges    []nativeWhirlpoolCell
	InitialGridSHA256 string
	Trace             []nativeWhirlpoolTick
}

func nativeWhirlpoolFixtures(t *testing.T) []nativeWhirlpoolCase {
	t.Helper()
	data, err := os.ReadFile("testdata/whirlpool_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeWhirlpoolCase }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 166 {
		t.Fatal("native whirlpool fixture catalog is incomplete")
	}
	return catalog.Cases
}

func whirlpoolRaw(actor NativeEffectActor) string {
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
	return hex.EncodeToString(raw)
}

func whirlpoolLoadActor(t *testing.T, golden nativeWhirlpoolActor) NativeEffectActor {
	t.Helper()
	raw, err := hex.DecodeString(golden.Raw)
	if err != nil || len(raw) != 32 {
		t.Fatal("invalid native whirlpool record")
	}
	actor := NativeEffectActor{Active: raw[12] != 0, Kind: raw[0], X: int16(binary.BigEndian.Uint16(raw[6:])), Y: int16(binary.BigEndian.Uint16(raw[8:])), Animation: int(binary.BigEndian.Uint16(raw[10:])), VX: int16(binary.BigEndian.Uint16(raw[14:])), VY: int16(binary.BigEndian.Uint16(raw[16:])), Speed: raw[18], Timer: int16(binary.BigEndian.Uint16(raw[20:])), State: raw[22], Life: int16(binary.BigEndian.Uint16(raw[24:]))}
	if actor.Active {
		actor.Player = raw[12] - 1
	}
	return actor
}

func whirlpoolApplyCells(grid *[16384]byte, cells []nativeWhirlpoolCell) {
	for _, cell := range cells {
		at := cell.Index * 4
		grid[at], grid[at+1] = cell.Height, cell.Tile
		binary.BigEndian.PutUint16(grid[at+2:], cell.Head)
	}
}

func whirlpoolInitialGrid(input nativeWhirlpoolInput) [16384]byte {
	var grid [16384]byte
	whirlpoolApplyCells(&grid, input.InitialCells)
	if input.RawOffset != 0 {
		grid[input.RawOffset] = input.RawValue
	}
	return grid
}

func whirlpoolGridHash(grid *[16384]byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(grid[:]))
}

func TestWhirlpoolCreationMatchesNativeAdmissionPoolAndTerrain(t *testing.T) {
	rules, err := DecodeWhirlpoolRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	cases := 0
	for _, fixture := range nativeWhirlpoolFixtures(t) {
		input := fixture.Input
		// Native owner 0 and neutral owners are retained in the evidence file.
		// This public creator is used only by Go players 0/1 (native owners 1/2).
		if input.Mode == "tick" || input.Owner < 1 || input.Owner > 2 {
			continue
		}
		cases++
		t.Run(input.Name, func(t *testing.T) {
			var pool [NativeEffectCapacity]NativeEffectActor
			for index := range pool {
				pool[index] = NativeEffectActor{Active: index < input.Occupied, VX: -19, VY: 71}
			}
			beforePool := pool
			grid := whirlpoolInitialGrid(input)
			beforeGrid := grid
			accepted := rules.Create(&pool, int(input.Owner)-1, input.X, input.Y, input.Experience, func(x, y int) uint8 { return grid[(x+y*64)*4+1] }, func(x, y int, tile uint8) { grid[(x+y*64)*4+1] = tile })
			if accepted != fixture.Accepted {
				t.Fatal("creation differs from native admission")
			}
			if got := whirlpoolRaw(pool[fixture.Slot]); got != fixture.InitialActor.Raw {
				t.Fatalf("native allocated/reused record differs\ngot  %s\nwant %s", got, fixture.InitialActor.Raw)
			}
			if whirlpoolGridHash(&grid) != fixture.InitialGridSHA256 || fixture.InitialRNG != input.Seed {
				t.Fatal("native creation terrain or unchanged RNG differs")
			}
			if !accepted && (pool != beforePool || grid != beforeGrid) {
				t.Fatal("rejected creation changed the pool or terrain")
			}
			for index := range pool {
				if index != fixture.Slot && pool[index] != beforePool[index] {
					t.Fatal("creation changed a slot other than the first free one")
				}
			}
		})
	}
	if cases != 134 {
		t.Fatalf("checked %d native creation cases, want 134", cases)
	}
}

// Lowering callbacks replay changes made by the real $d7f0 routine in the
// isolated 68000. Thus this test checks the controller's call order, vertex
// coordinates, subsequent reads, and full parcel grid without replacing that
// external terrain operation with an invented area-damage approximation.
func TestWhirlpoolControllerMatchesNativeLifetimeMotionCoastlineAndAudio(t *testing.T) {
	rules, err := DecodeWhirlpoolRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	cases, checkpoints, ticks := 0, 0, 0
	for _, fixture := range nativeWhirlpoolFixtures(t) {
		if len(fixture.Trace) == 0 {
			continue
		}
		cases++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			grid := whirlpoolInitialGrid(input)
			whirlpoolApplyCells(&grid, fixture.InitialChanges)
			actor := whirlpoolLoadActor(t, fixture.InitialActor)
			rng := fixture.InitialRNG
			next := 0
			for tick := 1; tick <= fixture.Trace[len(fixture.Trace)-1].Tick; tick++ {
				ticks++
				var golden *nativeWhirlpoolTick
				if fixture.Trace[next].Tick == tick {
					golden = &fixture.Trace[next]
				}
				lowered := 0
				callbacks := WhirlpoolCallbacks{
					ReadTile:     func(x, y int) uint8 { return grid[(x+y*64)*4+1] },
					WriteTile:    func(x, y int, tile uint8) { grid[(x+y*64)*4+1] = tile },
					ReadGridByte: func(offset uint16) uint8 { return grid[offset] },
					LowerVertex: func(x, y int) {
						if golden == nil || lowered >= len(golden.Lowers) {
							t.Fatalf("unexpected lowering at tick %d vertex(%d,%d)", tick, x, y)
						}
						expected := golden.Lowers[lowered]
						if expected.X != x || expected.Y != y {
							t.Fatalf("lowering %d uses vertex(%d,%d), native(%d,%d)", lowered, x, y, expected.X, expected.Y)
						}
						whirlpoolApplyCells(&grid, expected.Changes)
						lowered++
					},
					Random: func() int {
						if rng == 0 {
							rng = 0xbc614e
						}
						rng *= 0xbb40e62d
						return int(rng >> 8 & 0x7fff)
					},
					ViewX: input.ViewX, ViewY: input.ViewY,
				}
				step, err := rules.Tick(&actor, callbacks)
				if err != nil {
					t.Fatal(err)
				}
				if golden != nil {
					checkpoints++
					if whirlpoolRaw(actor) != golden.Actor.Raw || whirlpoolGridHash(&grid) != golden.GridSHA256 || rng != golden.RNG {
						t.Fatalf("tick %d native actor/grid/RNG differs\ngot actor %s\nwant actor %s\ngot grid %s\nwant grid %s\nrng %x want %x", tick, whirlpoolRaw(actor), golden.Actor.Raw, whirlpoolGridHash(&grid), golden.GridSHA256, rng, golden.RNG)
					}
					if lowered != len(golden.Lowers) {
						t.Fatalf("tick %d omitted a native coastline lowering", tick)
					}
					if (step.SoundCue == 125) != golden.SoundCue || (!golden.SoundCue && step.SoundCue != -1) || step.Finished == actor.Active {
						t.Fatalf("tick %d native audible view or expiry result differs", tick)
					}
					next++
				}
			}
		})
	}
	if cases != 38 || checkpoints != 308 || ticks != 3599 {
		t.Fatalf("native coverage cases/checkpoints/ticks = %d/%d/%d, expected38/308/3599", cases, checkpoints, ticks)
	}
}

func TestWhirlpoolDecodedTerrainArtworkAndCornerWindow(t *testing.T) {
	bundle := testBundle(t)
	rules, err := DecodeWhirlpoolRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if rules.BaseLife != 300 || rules.Speed != 16 || rules.SoundCue != 125 || rules.Footprint != [4]uint16{0, 1, 0x101, 0x100} {
		t.Fatal("native whirlpool parameters differ")
	}
	for landscape, tiles := range bundle.Tiles {
		for tile := 0x98; tile <= 0xa7; tile++ {
			if tile >= len(tiles) || tiles[tile] == nil || rules.TileProperties[tile]&8 == 0 {
				t.Fatalf("native whirlpool terrain tile%x missing in landscape%d", tile, landscape)
			}
		}
	}
	if rules.CornerWords[16] != 0xfeff || rules.CornerWords[17] != 0xff00 {
		t.Fatal("native edge retries lost words from the following release table")
	}
}
