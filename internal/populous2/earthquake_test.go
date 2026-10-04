package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

type quakeLowerFixture struct {
	X, Y                            int
	BeforeAltSHA256, AfterAltSHA256 string
	BeforeDirty, AfterDirty         uint16
	Changes                         []struct {
		Index int
		Bytes [4]uint8
	}
}
type quakeFrameFixture struct {
	Tick                                                 int
	PoolSHA256, GridSHA256, AltSHA256, GlobalAliasSHA256 string
	RNG                                                  uint32
	Dirty, Shake                                         uint16
	ActiveCount                                          int
	Lower                                                []quakeLowerFixture
}
type quakeFixture struct {
	Input struct {
		Name, Mode                   string
		X, Y                         int
		Direction, Owner             uint8
		Strength                     uint16
		Seed                         uint32
		Ticks                        int
		Full, ParentOnly, EdgeParent bool
	}
	Initial, Creation quakeFrameFixture
	CreationAddress   int
	Trace             []quakeFrameFixture
}

func quakeFixtureMemory(f quakeFixture) (*entryFixtureMemory, *legacy.World) {
	m := &entryFixtureMemory{}
	var alt [4225]int
	for y := 0; y <= 64; y++ {
		for x := 0; x <= 64; x++ {
			height := 3
			switch f.Input.Mode {
			case "water":
				height = 0
			case "slope":
				height = 1 + x/8
			case "checker":
				height = 3 + (x+y)%2
			case "hill":
				dx, dy := x-32, y-32
				if dx < 0 {
					dx = -dx
				}
				if dy < 0 {
					dy = -dy
				}
				if dy > dx {
					dx = dy
				}
				height = max(0, 8-dx/4)
			}
			alt[x+y*65] = min(8, height)
		}
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			a := x + y*65
			v := [4]int{alt[a], alt[a+1], alt[a+66], alt[a+65]}
			base := min(v[0], v[1], v[2], v[3])
			shape := uint8(0)
			for bit, h := range v {
				if h > base {
					shape |= 1 << bit
				}
			}
			if shape == 0 && base > 0 {
				base--
				shape = 15
			}
			at := 0xf44 + (x+y*64)*4
			m.bytes[at], m.bytes[at+1] = uint8(base)|0xa8, shape
		}
	}
	for slot := 0; slot < 250; slot++ {
		at := 0xc800 + slot*32
		for offset := 0; offset < 32; offset++ {
			m.bytes[at+offset] = uint8(0x80 + offset)
		}
		m.bytes[at+12] = 0
		if f.Input.Full {
			m.bytes[at+12], m.bytes[at+22] = 1, 0x26
			m.putWord(at+6, 0x2080)
			m.putWord(at+8, 0x2080)
			m.putWord(at+10, 16)
			m.putWord(at+20, 32767)
			m.putWord(at+24, 32767)
		}
	}
	if f.Input.ParentOnly || f.Input.EdgeParent {
		at := 0xc800 + 249*32
		m.bytes[at+12], m.bytes[at+22], m.bytes[at+26], m.bytes[at+18] = 1, 0x22, f.Input.Direction, 4
		m.putWord(at+6, uint16(f.Input.X*256))
		m.putWord(at+8, uint16(f.Input.Y*256))
		m.putWord(at+10, uint16(f.Input.Direction>>1))
		m.putWord(at+20, 1)
		m.putWord(at+24, f.Input.Strength)
	}
	m.putLong(0xeb28, f.Input.Seed)
	m.putWord(0xf42, 1)
	m.putWord(0x5f44, 30)
	m.putWord(0x5f46, 30)
	return m, legacy.WorldFromSnapshot(legacy.WorldSnapshot{Alt: alt, RNG: f.Input.Seed}, legacy.DefaultTerrainRules())
}

func assertQuakeFrame(t *testing.T, m *entryFixtureMemory, core *legacy.World, f quakeFrameFixture, shake uint16) {
	t.Helper()
	for _, h := range []struct {
		at, size   int
		want, name string
	}{{0xc800, 8000, f.PoolSHA256, "complete250-slot pool"}, {0xf44, 16384, f.GridSHA256, "complete4096-cell map"}, {0xe740, 42, f.GlobalAliasSHA256, "magnet alias"}} {
		if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[h.at:h.at+h.size])); got != h.want {
			t.Fatalf("native%s differs at update%d: got%s want%s", h.name, f.Tick, got, h.want)
		}
	}
	if nativeDirectHeightHash(core.Alt) != f.AltSHA256 {
		t.Fatalf("native4225-height grid differs at update%d", f.Tick)
	}
	if m.long(0xeb28) != f.RNG || m.word(0xf2e) != f.Dirty || shake != f.Shake {
		t.Fatalf("native RNG/dirty/shake counters differ at update%d: rng%d/%d dirty%d/%d shake%d/%d", f.Tick, m.long(0xeb28), f.RNG, m.word(0xf2e), f.Dirty, shake, f.Shake)
	}
	active := 0
	for at := 0xc800; at < 0xe740; at += 32 {
		if m.bytes[at+12] != 0 {
			active++
		}
	}
	if active != f.ActiveCount {
		t.Fatal("native effect allocation count differs")
	}
}

// These original runs compare all8000 actor bytes, all4096 terrain cells,
// all4225 heights and RNG for every update. Lowering executes the proven Go
// propagation primitive; its original native cell mutations are replayed at
// that external boundary, keeping crack control separate from reconstruction.
func TestEarthquakeAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/earthquake_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []quakeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 34 {
		t.Fatal("native earthquake catalog incomplete")
	}
	rules, err := DecodeEarthquakeRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	updates := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m, core := quakeFixtureMemory(f)
			shake := uint16(0)
			assertQuakeFrame(t, m, core, f.Initial, shake)
			memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
			lowers := f.Creation.Lower
			lowerIndex := 0
			cb := EarthquakeCallbacks{Memory: memory, CameraX: 30, CameraY: 30,
				Tile: func(x, y int) (uint8, error) { return m.bytes[0xf44+(x+y*64)*4+1], nil }, Header: func(x, y int) (uint8, error) { return m.bytes[0xf44+(x+y*64)*4], nil }, SetTile: func(x, y int, tile uint8) error { m.bytes[0xf44+(x+y*64)*4+1] = tile; return nil },
				Dirty: func() error { m.putWord(0xf2e, m.word(0xf2e)+1); return nil }, Shake: func() error { shake++; return nil },
				Random: func() int {
					seed := m.long(0xeb28)
					if seed == 0 {
						seed = 0xbc614e
					}
					seed *= 0xbb40e62d
					m.putLong(0xeb28, seed)
					return int(seed >> 8 & 0x7fff)
				},
				Lower: func(x, y int) error {
					if lowerIndex >= len(lowers) {
						return fmt.Errorf("unexpected native lower%d,%d", x, y)
					}
					native := lowers[lowerIndex]
					lowerIndex++
					if x != native.X || y != native.Y || nativeDirectHeightHash(core.Alt) != native.BeforeAltSHA256 || m.word(0xf2e) != native.BeforeDirty {
						return fmt.Errorf("native lower input differs")
					}
					core.DirectLowerTerrain(x, y)
					if nativeDirectHeightHash(core.Alt) != native.AfterAltSHA256 {
						return fmt.Errorf("Go normative lowering height differs")
					}
					for _, cell := range native.Changes {
						copy(m.bytes[0xf44+cell.Index*4:], cell.Bytes[:])
					}
					m.putWord(0xf2e, native.AfterDirty)
					return nil
				},
			}
			if !f.Input.ParentOnly && !f.Input.EdgeParent {
				created, err := rules.Create(f.Input.Owner, uint8(f.Input.X), uint8(f.Input.Y), f.Input.Direction, f.Input.Strength, 0xeb56, cb)
				if err != nil {
					t.Fatal(err)
				}
				if created.Address != f.CreationAddress {
					t.Fatal("native creator return pointer differs")
				}
			}
			if lowerIndex != len(lowers) {
				t.Fatal("native creation lowering omitted")
			}
			assertQuakeFrame(t, m, core, f.Creation, shake)
			for _, native := range f.Trace {
				updates++
				lowers = native.Lower
				lowerIndex = 0
				if f.Input.ParentOnly || f.Input.EdgeParent {
					if _, err := rules.Tick(0xc800+249*32, cb); err != nil {
						t.Fatal(err)
					}
				} else {
					for at := 0xc800; at < 0xe740; at += 32 {
						if _, err := rules.Tick(at, cb); err != nil {
							t.Fatal(err)
						}
					}
				}
				if lowerIndex != len(lowers) {
					t.Fatal("native runtime lowering omitted")
				}
				assertQuakeFrame(t, m, core, native, shake)
			}
		})
	}
	if updates != 3067 {
		t.Fatalf("native earthquake update count differs: %d", updates)
	}
}
