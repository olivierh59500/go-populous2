package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativePrimitiveInput struct {
	Name, Kind          string
	Owner               uint16
	X, Y                uint8
	Seed                uint32
	FXFree, SceneryFree int
	Initial             []nativeHeroPatch
}
type nativePrimitiveFixture struct {
	Input          nativePrimitiveInput
	Hash           string
	Changes        []nativeHeroChange
	D0, RNG        uint32
	RandomDraws    int
	Links          []uint16
	CycleReference uint16
}

func primitiveFixtureMemory(input nativePrimitiveInput) *cleanupMemory {
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 0xa8)
		m.putByte(0xf45+index*4, 15)
	}
	for address := 0x6bd0; address < 0x76c0; address += 14 {
		for offset := range 14 {
			m.putByte(address+offset, uint8((address-0x6bd0+offset)*13+7))
		}
		m.putByte(address+12, 1)
	}
	for address := 0xc800; address < 0xe740; address += 32 {
		for offset := range 32 {
			m.putByte(address+offset, uint8((address-0xc800+offset)*13+7))
		}
		m.putByte(address+12, 1)
	}
	if input.FXFree >= 0 {
		m.putByte(0xc800+input.FXFree*32+12, 0)
	}
	if input.SceneryFree >= 0 {
		m.putByte(0x6bd0+input.SceneryFree*14+12, 0)
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
	return m
}

// Raw creator cases compare every byte of retained pools, profiles and grid.
// Neutral owner3, native-word XP aliases, full pools, border jitter, existing
// tree cycles and first-free-slot reuse execute against the actual binary.
func TestNativePrimitiveCreatorsAgainstOriginalRoutines(t *testing.T) {
	data, err := os.ReadFile("testdata/primitive_creators_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativePrimitiveFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 383 {
		t.Fatal("native primitive creator catalog incomplete")
	}
	rules, err := DecodeNativePrimitiveCreatorRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := primitiveFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			rng, draws := fixture.Input.Seed, 0
			links := []uint16{}
			cb := NativePrimitiveCreatorCallbacks{Memory: memory, Random: func() uint16 {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				return uint16(rng >> 8 & 0x7fff)
			}, Link: func(ref NativeRecordReference) error { links = append(links, uint16(ref)); return m.insert(ref) }}
			var step NativePrimitiveCreation
			var err error
			switch fixture.Input.Kind {
			case "whirlwind":
				step, err = rules.CreateWhirlwind(fixture.Input.Owner, fixture.Input.X, fixture.Input.Y, cb)
			case "fire":
				step, err = rules.CreateFireColumn(fixture.Input.Owner, fixture.Input.X, fixture.Input.Y, cb)
			case "tree":
				step, err = rules.PlantTree(fixture.Input.X, fixture.Input.Y, cb)
			default:
				t.Fatal("unknown native creator")
			}
			if err != nil {
				t.Fatal(err)
			}
			if step.Created != (len(fixture.Links) != 0) || step.Cycled != (fixture.CycleReference != 0) || step.RandomDraws != fixture.RandomDraws || draws != fixture.RandomDraws {
				t.Fatal("native allocation, cycle or RNG admission differs")
			}
			if step.Created && uint16(step.Reference) != fixture.Links[0] || step.Cycled && uint16(step.Reference) != fixture.CycleReference {
				t.Fatal("native selected raw slot differs")
			}
			if rng != fixture.RNG || !reflect.DeepEqual(links, fixture.Links) {
				t.Fatal("native RNG or primitive graph links differ")
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
				t.Fatalf("complete native creator BSS differs: got%s want%s", got, fixture.Hash)
			}
			if m.err != nil {
				t.Fatal(m.err)
			}
		})
	}
}
