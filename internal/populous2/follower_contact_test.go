package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeContactInput struct {
	Name           string
	Source, Target uint16
	Initial        []nativeHeroPatch
}
type nativeContactCall struct {
	Kind       string
	Ref, Value uint16
}
type nativeContactFixture struct {
	Input                            nativeContactInput
	Hash                             string
	Changes                          []nativeHeroChange
	Calls                            []nativeContactCall
	EffectiveSource, EffectiveTarget uint16
	D0, A0, A1, RNG                  uint32
}

func contactFixtureMemory(input nativeContactInput) *cleanupMemory {
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 0xa8)
		m.putByte(0xf45+index*4, 15)
		m.putByte(0x4f44+index, 0x77)
	}
	for index := 1; index <= 4; index++ {
		a := 0x76c0 + index*52
		m.putByte(a, 2)
		m.putByte(a+12, uint8(1+(index+1)%2))
		m.putWord(a+6, 0x2087)
		m.putWord(a+8, 0x2093)
		m.putWord(a+10, 0x1c8)
		m.putWord(a+14, 17)
		m.putWord(a+16, 23)
		m.putByte(a+18, 16)
		m.putByte(a+19, 9)
		m.putWord(a+20, 23)
		m.putByte(a+22, 0x26)
		_ = m.write32(a+26, 1000)
		m.putWord(a+34, 156)
		m.putWord(a+36, 208)
		m.putWord(a+46, 123)
		m.putWord(a+50, 4)
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
	for index := 1; index <= 2; index++ {
		a := 0x76c0 + index*52
		if m.byte(a) == 4 {
			tile := uint8(47)
			if m.byte(a+12) == 2 {
				tile = 63
			}
			for y := 29; y <= 35; y++ {
				for x := 29; x <= 35; x++ {
					m.putByte(0xf45+(x+y*64)*4, tile)
				}
			}
		}
	}
	return m
}

// Complete native contact comparisons include all six hero types, swapped
// attackers, all19 town stages, same-record aliases, and raw captive/backlink
// addresses. Actual49-cell town farm cleanup runs on both sides of the proof.
func TestFollowerContactAgainstCompleteNativeRoutine(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_contact_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeContactFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 292 {
		t.Fatal("native contact catalog incomplete")
	}
	town, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := contactFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []nativeContactCall{}
			cb := FollowerContactCallbacks{Memory: memory, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
				calls = append(calls, nativeContactCall{Kind: "clear", Ref: uint16(ref), Value: uint16(tile)})
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}, Sound: func(value uint16) error {
				calls = append(calls, nativeContactCall{Kind: "sound", Value: value})
				return nil
			}}
			step, err := PrepareFollowerContact(NativeRecordReference(fixture.Input.Source), NativeRecordReference(fixture.Input.Target), cb)
			if err != nil {
				t.Fatal(err)
			}
			captured := false
			for _, call := range fixture.Calls {
				captured = captured || call.Kind == "sound"
			}
			if uint16(step.Aggressor) != fixture.EffectiveSource || uint16(step.Defender) != fixture.EffectiveTarget || step.Swapped != (fixture.EffectiveSource != fixture.Input.Source) || step.Captured != captured {
				t.Fatal("native participant swap or capture branch differs")
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("native callback order differs: got%v want%v", calls, fixture.Calls)
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
				t.Fatalf("complete native contact BSS differs: got%s want%s", got, fixture.Hash)
			}
			if fixture.D0 != 0xa55a1122 || fixture.A0 != uint32(cleanupRecordAddress(NativeRecordReference(fixture.Input.Source))) || fixture.A1 != uint32(cleanupRecordAddress(NativeRecordReference(fixture.Input.Target))) || fixture.RNG != 4311 {
				t.Fatal("original contact register preservation or RNG fixture differs")
			}
			if m.err != nil {
				t.Fatal(m.err)
			}
		})
	}
}
