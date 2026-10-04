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

type neutralComposedInput struct {
	Name     string
	Selector uint16
	X, Y     uint16
	VX, VY   int16
	Seed     uint32
	Alt      []uint8
	Initial  []nativeHeroPatch
	Links    []uint16
	BusyFX   int
}
type neutralComposedCall struct {
	Kind        string
	Reference   uint16
	X, Y        uint8
	Owner, Mode uint16
}
type neutralComposedFixture struct {
	Input         neutralComposedInput
	Hash, AltHash string
	Changes       []nativeHeroChange
	Calls         []neutralComposedCall
	RandomDraws   int
	RNG           uint32
}

func neutralComposedMemory(input neutralComposedInput) *cleanupMemory {
	m := &cleanupMemory{}
	for y := range 64 {
		for x := range 64 {
			i := x + y*65
			h := [4]uint8{input.Alt[i], input.Alt[i+1], input.Alt[i+66], input.Alt[i+65]}
			low := min(h[0], h[1], h[2], h[3])
			shape := uint8(0)
			for index, v := range h {
				if v > low {
					shape |= 1 << index
				}
			}
			if shape == 0 && low > 0 {
				low--
				shape = 15
			}
			at := 0xf44 + (x+y*64)*4
			m.putByte(at, 0xa8|low)
			m.putByte(at+1, shape)
			m.putByte(0x4f44+x+y*64, 0x77)
		}
	}
	for index := range 250 {
		a := 0xc800 + index*32
		for off := range 32 {
			m.putByte(a+off, uint8(index*32+off+7))
		}
		m.putByte(a+12, 0)
		if index < input.BusyFX {
			m.putByte(a+12, 1)
		}
	}
	starts := [7]uint16{0x2cc, 0x2cc, 0x53c, 0xa98, 0xab4, 0x2bfc, 0x2c18}
	a := 0x76f4
	m.putByte(a, 0x3c)
	m.putByte(a+12, 3)
	m.putByte(a+22, 0x44)
	m.putWord(a+40, input.Selector)
	m.putWord(a+6, input.X)
	m.putWord(a+8, input.Y)
	m.putWord(a+14, uint16(input.VX))
	m.putWord(a+16, uint16(input.VY))
	m.putWord(a+10, starts[input.Selector/2])
	for owner := uint8(1); owner <= 2; owner++ {
		god := 0xe76a + int(owner)*314
		marker := 0xe740 + int(owner)*14
		m.putWord(god+10, uint16(marker-0x76c0))
		m.putWord(god+0x44, 120)
		m.putWord(god+0x46, 3)
		m.putByte(marker+12, owner)
		m.putWord(marker+6, uint16(10+int(owner))<<8|128)
		m.putWord(marker+8, 0x0a80)
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
	for _, ref := range append([]uint16{0x708e, 0x709c, 52}, input.Links...) {
		if err := m.insert(NativeRecordReference(ref)); err != nil {
			m.err = err
		}
	}
	return m
}

func neutralComposedMove(m *cleanupMemory, ref NativeRecordReference, x, y uint16) error {
	var grid NativeOccupancyState
	for index := range grid.Cells {
		at := 0xf44 + index*4
		grid.Cells[index] = NativeOccupancyCell{Header: m.byte(at), Tile: m.byte(at + 1), Head: NativeRecordReference(m.word(at + 2))}
	}
	if _, err := grid.Move(ref, x, y, m.runtime().RecordAccess()); err != nil {
		return err
	}
	for index, cell := range grid.Cells {
		at := 0xf44 + index*4
		m.putByte(at, cell.Header)
		m.putByte(at+1, cell.Tile)
		m.putWord(at+2, uint16(cell.Head))
	}
	return m.err
}

// Composed calls execute actual primitive creators and complete retained
// cleanup/farm composition. Direct lowering runs the inherited propagation
// hook; every height and touched raw parcel is compared with the original CPU.
func TestNativeNeutralRuntimeWithRealPrimitivesAndVictimChains(t *testing.T) {
	data, err := os.ReadFile("testdata/neutral_runtime_composed_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []neutralComposedFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 321 {
		t.Fatal("composed neutral catalog incomplete")
	}
	bundle := testBundle(t)
	neutral, err := DecodeNativeNeutralRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	creators, err := DecodeNativePrimitiveCreatorRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := neutralComposedMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			var alt [legacy.EndWidth * legacy.EndWidth]int
			for index, height := range fixture.Input.Alt {
				alt[index] = int(height)
			}
			core := legacy.WorldFromSnapshot(legacy.WorldSnapshot{Alt: alt, RNG: fixture.Input.Seed}, legacy.DefaultTerrainRules())
			geometry := &World{Core: core}
			for index := range 4096 {
				tile := m.byte(0xf45 + index*4)
				if tile&0xf0 == 0xe0 {
					geometry.Marks[index] = Mark{Spell: Basalt, Life: 1, Persistent: true, NativeTile: tile}
				}
			}
			rng, draws := fixture.Input.Seed, 0
			random := func() int {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				return int(rng >> 8 & 0x7fff)
			}
			primitive := NativePrimitiveCreatorCallbacks{Memory: memory, Random: func() uint16 { return uint16(random()) }, Link: m.insert}
			calls := []neutralComposedCall{}
			packedCell := func(tile NativePackedTile) int { return 0xf44 + int(uint8(tile))*4 + int(uint8(uint16(tile)>>8))*256 }
			cb := NativeNeutralActorCallbacks{Memory: memory, Move: func(ref NativeRecordReference, x, y uint16) error { return neutralComposedMove(m, ref, x, y) }, Unlink: m.unlink, Head: func(tile NativePackedTile) (NativeRecordReference, error) {
				value, err := m.read16(packedCell(tile) + 2)
				return NativeRecordReference(value), err
			}, Tile: func(tile NativePackedTile) (uint8, error) { return m.read8(packedCell(tile) + 1) }, WriteTile: func(tile NativePackedTile, value uint8) error { return m.write8(packedCell(tile)+1, value) }, Random: random,
				CreateWhirlwind: func(x, y uint8, owner uint16) error {
					calls = append(calls, neutralComposedCall{Kind: "whirlwind", X: x, Y: y, Owner: owner})
					_, err := creators.CreateWhirlwind(owner, x, y, primitive)
					return err
				},
				PlantTree: func(x, y uint8, _ uint16) error {
					calls = append(calls, neutralComposedCall{Kind: "tree", X: x, Y: y})
					_, err := creators.PlantTree(x, y, primitive)
					return err
				},
				CreateFireColumn: func(x, y uint8, owner uint16) error {
					calls = append(calls, neutralComposedCall{Kind: "fire", X: x, Y: y, Owner: owner})
					_, err := creators.CreateFireColumn(owner, x, y, primitive)
					return err
				},
				Cleanup: func(ref NativeRecordReference, mode uint16) error {
					calls = append(calls, neutralComposedCall{Kind: "cleanup", Reference: uint16(ref), Mode: mode})
					_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
						return town.ClearFarms(ref, tile, winTownCallbacks(m))
					}})
					return err
				},
				Lower: func(x, y uint8) error {
					calls = append(calls, neutralComposedCall{Kind: "lower", X: x, Y: y})
					before := core.Alt
					core.DirectLowerTerrain(int(x), int(y))
					geometry.clearChangedGround(before)
					changed := 0
					for index, height := range core.Alt {
						changed += before[index] - height
					}
					// $d012 increments work even for the direct height0 no-op; each
					// successful recursive unit contributes one to both work and DD2.
					m.putWord(0xdd2, uint16(changed))
					m.putWord(0xf2e, m.word(0xf2e)+uint16(max(changed, 1)))
					for yy := range 64 {
						for xx := range 64 {
							i := xx + yy*65
							if before[i] == core.Alt[i] && before[i+1] == core.Alt[i+1] && before[i+65] == core.Alt[i+65] && before[i+66] == core.Alt[i+66] {
								continue
							}
							cell := geometry.TerrainCell(xx, yy)
							at := 0xf44 + (xx+yy*64)*4
							old := neutral.Raster[m.byte(at+1)]
							m.putByte(at, uint8(cell.BaseAltitude))
							m.putByte(at+1, old&0xf0|cell.Shape)
						}
					}
					return m.err
				},
			}
			if _, err := neutral.Tick(52, cb); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("actual nested native callbacks differ: got%v want%v", calls, fixture.Calls)
			}
			if rng != fixture.RNG || draws != fixture.RandomDraws {
				t.Fatal("composed primitive RNG consumption differs")
			}
			var image [NativeRuntimeImageEnd]byte
			copy(image[:NativeRecordImageStart], m.lower[:])
			copy(image[NativeRecordImageStart:NativeMagnetImageStart], m.records.Bytes[:])
			copy(image[NativeMagnetImageStart:], m.globals.Bytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(image[:])); got != fixture.Hash {
				for _, change := range fixture.Changes {
					if image[change.Address] != change.Value {
						t.Errorf("native byte%x got%x want%x", change.Address, image[change.Address], change.Value)
					}
				}
				t.Fatalf("complete composed native BSS differs: got%s want%s", got, fixture.Hash)
			}
			if nativeDirectHeightHash(core.Alt) != fixture.AltHash {
				t.Fatal("complete native4225-height grid differs")
			}
		})
	}
}
