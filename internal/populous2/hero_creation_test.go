package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type heroCreationInput struct {
	Name                                               string
	Hero, Owner, Flags, Kind, Stage, Speed, Experience uint8
	Population                                         uint32
	Missing, Direct                                    bool
}

type heroCreationCall struct {
	Kind       string
	Ref, Value uint16
}

type heroCreationFixture struct {
	Input   heroCreationInput
	Hash    string
	Changes []nativeHeroChange
	Calls   []heroCreationCall
	D0, RNG uint32
}

func heroCreationFixtureMemory(c heroCreationInput) *cleanupMemory {
	m := &cleanupMemory{}
	for j := range 4096 {
		m.putByte(0xf44+j*4, 0xa8)
		m.putByte(0xf45+j*4, 15)
		m.putByte(0x4f44+j, 0x77)
	}
	a := 0x76f4
	for j := range 52 {
		m.putByte(a+j, uint8(j*13+7))
	}
	m.putWord(a+2, 0)
	m.putWord(a+4, 0)
	m.putByte(a, c.Kind)
	m.putByte(a+1, c.Stage)
	m.putByte(a+12, c.Owner)
	m.putByte(a+13, c.Flags)
	m.putWord(a+6, 0x2087)
	m.putWord(a+8, 0x2193)
	m.putByte(a+18, c.Speed)
	_ = m.write32(a+26, c.Population)
	god := 0xe76a + int(c.Owner)*314
	marker := 0xe740 + int(c.Owner)*14
	_ = m.write32(god, 0x11223344)
	if !c.Missing {
		m.putWord(god+8, 52)
	}
	m.putWord(god+10, uint16(marker-0x76c0))
	m.putByte(god+0x52+int(c.Hero), c.Experience)
	m.putByte(marker, 20)
	m.putByte(marker+12, c.Owner)
	m.putWord(marker+6, 0x0880)
	m.putWord(marker+8, 0x0980)
	_ = m.insert(NativeRecordReference(marker - 0x76c0))
	_ = m.insert(52)
	if c.Kind == 4 {
		tile := uint8(47)
		if c.Owner == 2 {
			tile = 63
		}
		for y := 30; y <= 36; y++ {
			for x := 29; x <= 35; x++ {
				m.putByte(0xf45+(x+y*64)*4, tile)
			}
		}
	}
	return m
}

// These comparisons execute complete original creation with all six heroes,
// both owners, every town stage, leader flags, speed saturation, zero/overflow
// population and direct Adonis conversion. No simulation boundary is stubbed.
func TestHeroCreationAgainstCompleteNativeRoutine(t *testing.T) {
	data, err := os.ReadFile("testdata/hero_creation_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []heroCreationFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1104 {
		t.Fatal("native hero creation catalog incomplete")
	}
	bundle := testBundle(t)
	town, err := DecodeNativeTownEvaluator(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeHeroRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := heroCreationFixtureMemory(fixture.Input)
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []heroCreationCall{}
			cb := HeroCreationCallbacks{Memory: memory, ClearLeader: func(ref NativeRecordReference) error {
				calls = append(calls, heroCreationCall{Kind: "leader", Ref: uint16(ref)})
				_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert})
				return err
			}, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
				calls = append(calls, heroCreationCall{Kind: "farms", Ref: uint16(ref), Value: uint16(tile)})
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}, Sound: func(value uint16) error {
				calls = append(calls, heroCreationCall{Kind: "sound", Value: value})
				return nil
			}}
			id := heroIDs[fixture.Input.Hero]
			created := false
			if fixture.Input.Direct {
				err = rules.Convert(52, id, cb)
				created = err == nil
			} else {
				var step HeroCreationStep
				step, err = rules.Create(id, uint16(fixture.Input.Owner), cb)
				created = step.Created
			}
			if err != nil || m.err != nil {
				t.Fatalf("native creation failed: %v/%v", err, m.err)
			}
			if created != (fixture.D0 == 1) || fixture.RNG != 4311 {
				t.Fatal("native admission or RNG differs")
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("native callback order differs: got%v want%v", calls, fixture.Calls)
			}
			var image [NativeRuntimeImageEnd]byte
			copy(image[:NativeRecordImageStart], m.lower[:])
			copy(image[NativeRecordImageStart:NativeMagnetImageStart], m.records.Bytes[:])
			copy(image[NativeMagnetImageStart:], m.globals.Bytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(image[:])); got != fixture.Hash {
				for _, c := range fixture.Changes {
					if image[c.Address] != c.Value {
						t.Errorf("native byte%x got%x want%x", c.Address, image[c.Address], c.Value)
					}
				}
				t.Fatalf("complete native BSS differs: got%s want%s", got, fixture.Hash)
			}
		})
	}
}
