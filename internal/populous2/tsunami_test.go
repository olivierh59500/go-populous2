package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	legacy "go-populous2/internal/legacy"
)

type nativeTsunamiInput struct {
	Name, Mode                   string
	Owner                        uint16
	X, Y                         uint8
	Parent, FreeStart, FreeCount int
	Direction                    uint16
	Alt                          []uint8
	Initial                      []nativeHeroPatch
	Links                        []uint16
	Ticks                        int
	Prepass                      uint16
}
type nativeTsunamiCall struct {
	Kind      string
	Reference uint16
	X, Y      uint8
}
type nativeTsunamiTrace struct {
	Update                            int
	Hash, PoolHash, GridHash, AltHash string
}
type nativeTsunamiFixture struct {
	Input         nativeTsunamiInput
	Hash, AltHash string
	Changes       []nativeHeroChange
	Calls         []nativeTsunamiCall
	Trace         []nativeTsunamiTrace
	RNG           uint32
}

func tsunamiFixtureMemory(input nativeTsunamiInput) *cleanupMemory {
	m := &cleanupMemory{}
	for y := range 64 {
		for x := range 64 {
			i := x + y*65
			h := [4]uint8{input.Alt[i], input.Alt[i+1], input.Alt[i+66], input.Alt[i+65]}
			low := min(h[0], h[1], h[2], h[3])
			shape := uint8(0)
			for k, v := range h {
				if v > low {
					shape |= 1 << k
				}
			}
			if shape == 0 && low > 0 {
				low--
				shape = 15
			}
			a := 0xf44 + (x+y*64)*4
			m.putByte(a, 0xa8|low)
			m.putByte(a+1, shape)
			m.putByte(0x4f44+x+y*64, 0x77)
		}
	}
	for index := range 250 {
		a := 0xc800 + index*32
		for off := range 32 {
			m.putByte(a+off, uint8(index*32+off+7))
		}
		owner := uint8(1)
		if index >= input.FreeStart && index < input.FreeStart+input.FreeCount {
			owner = 0
		}
		m.putByte(a+12, owner)
	}
	parent := 0xc800 + input.Parent*32
	if input.Mode != "create" {
		for off := range 32 {
			m.putByte(parent+off, 0)
		}
		animations := [4]uint16{0xc60, 0xc84, 0xca8, 0xc3c}
		vectors := [4][2]int16{{0, -32}, {32, 0}, {0, 32}, {-32, 0}}
		direction := input.Direction / 4
		m.putByte(parent, 0x32)
		m.putByte(parent+12, uint8(input.Owner))
		m.putByte(parent+22, 0x2a)
		m.putWord(parent+6, uint16(input.X)<<8|128)
		m.putWord(parent+8, uint16(input.Y)<<8|128)
		m.putWord(parent+10, animations[direction])
		m.putWord(parent+14, uint16(vectors[direction][0]))
		m.putWord(parent+16, uint16(vectors[direction][1]))
		m.putWord(parent+24, 200)
		m.putWord(parent+26, input.Direction)
	}
	for _, p := range input.Initial {
		switch p.Width {
		case 1:
			m.putByte(p.Address, uint8(p.Value))
		case 2:
			m.putWord(p.Address, uint16(p.Value))
		case 4:
			if err := m.write32(p.Address, p.Value); err != nil {
				m.err = err
			}
		}
	}
	refs := append([]uint16{}, input.Links...)
	if input.Mode != "create" {
		refs = append(refs, uint16(0x5140+input.Parent*32))
	}
	for _, ref := range refs {
		if err := m.insert(NativeRecordReference(ref)); err != nil {
			m.err = err
		}
	}
	return m
}

func tsunamiRawHeightHash(m *cleanupMemory, raster []byte) string {
	var heights [4225]byte
	for y := 0; y <= 64; y++ {
		for x := 0; x <= 64; x++ {
			xx, yy, bit := x, y, uint(0)
			if xx == 64 {
				xx--
				bit = 1
			}
			if yy == 64 {
				yy--
				bit = 3
			}
			if x == 64 && y == 64 {
				bit = 2
			}
			a := 0xf44 + (xx+yy*64)*4
			heights[x+y*65] = (m.byte(a) & 7) + (raster[int(m.byte(a+1))] >> bit & 1)
		}
	}
	return fmt.Sprintf("%x", sha256.Sum256(heights[:]))
}

// Complete native tidal-wave comparisons include source-position fractions,
// first-free reuse, earlier/later children, barriers, direct shore lowering
// and real Helen/hero water prepass. No directed radius damage is substituted.
func TestTsunamiAgainstNativeCreatorRuntimeAndOrderedPool(t *testing.T) {
	data, err := os.ReadFile("testdata/tsunami_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeTsunamiFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 914 {
		t.Fatal("native tsunami catalog incomplete")
	}
	bundle := testBundle(t)
	rules, err := DecodeTsunamiRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	prepass, err := DecodeCommonPrepassRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, action := range bundle.Actions {
		if action.Command == 56 {
			found = true
			if action.Spell != Tsunami || action.Handler != 0x17a7c {
				t.Fatal("native tidal-wave catalog identity differs")
			}
		}
	}
	if !found {
		t.Fatal("tidal-wave command56 missing")
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := tsunamiFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			var alt [legacy.EndWidth * legacy.EndWidth]int
			for index, height := range fixture.Input.Alt {
				alt[index] = int(height)
			}
			core := legacy.WorldFromSnapshot(legacy.WorldSnapshot{Alt: alt, RNG: 4311}, legacy.DefaultTerrainRules())
			geometry := &World{Core: core}
			for index := range 4096 {
				tile := m.byte(0xf45 + index*4)
				if tile&0xf0 == 0xe0 {
					geometry.Marks[index] = Mark{Spell: Basalt, Life: 1, Persistent: true, NativeTile: tile}
				}
			}
			calls := []nativeTsunamiCall{}
			cb := TsunamiCallbacks{Memory: memory, Link: func(ref NativeRecordReference) error {
				calls = append(calls, nativeTsunamiCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}, Unlink: func(ref NativeRecordReference) error {
				calls = append(calls, nativeTsunamiCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}, Move: func(ref NativeRecordReference, x, y uint16) (bool, error) {
				a := cleanupRecordAddress(ref)
				old := m.word(a+8)&0xff00 | uint16(m.byte(a+6))
				changed := old != (y&0xff00 | x>>8)
				return changed, neutralComposedMove(m, ref, x, y)
			}, Lower: func(x, y uint8) error {
				calls = append(calls, nativeTsunamiCall{Kind: "lower", X: x, Y: y})
				before := core.Alt
				core.DirectLowerTerrain(int(x), int(y))
				geometry.clearChangedGround(before)
				changed := 0
				for index, height := range core.Alt {
					changed += before[index] - height
				}
				m.putWord(0xdd2, uint16(changed))
				m.putWord(0xf2e, m.word(0xf2e)+uint16(max(changed, 1)))
				for yy := range 64 {
					for xx := range 64 {
						i := xx + yy*65
						if before[i] == core.Alt[i] && before[i+1] == core.Alt[i+1] && before[i+65] == core.Alt[i+65] && before[i+66] == core.Alt[i+66] {
							continue
						}
						cell := geometry.TerrainCell(xx, yy)
						a := 0xf44 + (xx+yy*64)*4
						raster := bundle.Executable.Hunks[0].Data[0x33512+int(m.byte(a+1))]
						m.putByte(a, uint8(cell.BaseAltitude))
						m.putByte(a+1, raster&0xf0|cell.Shape)
					}
				}
				return m.err
			}}
			if fixture.Input.Mode == "create" {
				if _, err := rules.Create(fixture.Input.Owner, fixture.Input.X, fixture.Input.Y, cb); err != nil {
					t.Fatal(err)
				}
			} else {
				parent := NativeRecordReference(0x5140 + fixture.Input.Parent*32)
				for _, trace := range fixture.Trace {
					if fixture.Input.Mode == "pass" {
						_ = m.write32(0xf40, uint32(trace.Update))
						for index := range 250 {
							a := 0xc800 + index*32
							if m.byte(a+12) == 0 {
								continue
							}
							if _, err := rules.Tick(NativeRecordReference(0x5140+index*32), cb); err != nil {
								t.Fatal(err)
							}
						}
					} else {
						if _, err := rules.Tick(parent, cb); err != nil {
							t.Fatal(err)
						}
					}
					lowered := false
					for _, call := range calls {
						lowered = lowered || call.Kind == "lower"
					}
					if lowered && nativeDirectHeightHash(core.Alt) != trace.AltHash {
						t.Fatal("actualCore direct lowering differs fromnative4225 heights")
					}
					if stormFixtureHash(m) != trace.Hash || tsunamiRawHeightHash(m, bundle.Executable.Hunks[0].Data[0x33512:0x33612]) != trace.AltHash {
						t.Fatalf("native wave pass%d retainedBSS/height differs", trace.Update)
					}
					if fmt.Sprintf("%x", sha256.Sum256(m.records.Bytes[0xc800-NativeRecordImageStart:0xe740-NativeRecordImageStart])) != trace.PoolHash || fmt.Sprintf("%x", sha256.Sum256(m.lower[0xf44:0x4f44])) != trace.GridHash {
						t.Fatal("complete wave pool/grid hash differs")
					}
				}
				if fixture.Input.Prepass != 0 {
					ref := NativeRecordReference(fixture.Input.Prepass)
					clear := func(ref NativeRecordReference, tile uint8) error {
						return town.ClearFarms(ref, tile, winTownCallbacks(m))
					}
					_, err := prepass.Tick(ref, CommonPrepassCallbacks{Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m, ref) }, Write: func(ref NativeRecordReference, a FollowerEntryActor) error { return winEntryWrite(m, ref, a) }, Tile: func(tile NativePackedTile) (uint8, error) { return m.read8(tsunamiGrid(uint16(tile)) + 1) }, WriteTile: func(tile NativePackedTile, value uint8) error { return m.write8(tsunamiGrid(uint16(tile))+1, value) }, Scenario: func(uint8) (uint16, error) { return 0, nil }, ClearFarms: clear, Cleanup: func(ref NativeRecordReference, mode uint16) error {
						_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: clear})
						return err
					}, Unlink: m.unlink, ClearLeader: func(NativeRecordReference) error { return fmt.Errorf("unexpected wave leader callback") }, Sound: func(uint16) error { return nil }})
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("native wave callbacks differ: got%v want%v", calls, fixture.Calls)
			}
			if stormFixtureHash(m) != fixture.Hash {
				for _, change := range fixture.Changes {
					got, _ := m.read8(change.Address)
					if got != change.Value {
						t.Errorf("native byte%x got%x want%x", change.Address, got, change.Value)
					}
				}
				t.Fatal("complete native wave BSS differs")
			}
			if tsunamiRawHeightHash(m, bundle.Executable.Hunks[0].Data[0x33512:0x33612]) != fixture.AltHash || fixture.RNG != 4311 || core.Snapshot().RNG != 4311 {
				t.Fatal("wave changed native height or no-RNG invariant")
			}
		})
	}
}
