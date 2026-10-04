package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeCaptiveInput struct {
	Name    string
	Initial []nativeHeroPatch
}
type nativeCaptiveFixture struct {
	Input            nativeCaptiveInput
	Hash, Stop       string
	Changes          []nativeHeroChange
	Calls            []nativeMagnetCall
	TargetX, TargetY uint8
	RNG              uint32
}

func captiveFixtureMemory(input nativeCaptiveInput) *cleanupMemory {
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 1)
		m.putByte(0xf45+index*4, 15)
	}
	for index := 1; index <= 4; index++ {
		a := 0x76c0 + index*52
		m.putByte(a, 2)
		m.putByte(a+12, 1)
		m.putByte(a+13, 8)
		m.putWord(a+6, 0x2087)
		m.putWord(a+8, 0x2093)
		m.putWord(a+14, 17)
		m.putWord(a+16, 23)
		m.putByte(a+18, 16)
		m.putWord(a+20, 23)
		m.putByte(a+22, 0x34)
		_ = m.write32(a+26, 1000)
	}
	m.putByte(0x7728+12, 2)
	m.putByte(0x7728+13, 2)
	m.putWord(0x7728+6, 0x2241)
	m.putWord(0x7728+8, 0x2157)
	m.putWord(0x7728+40, 10)
	m.putWord(0x7728+42, 52)
	m.putWord(0x76f4+42, 0x35c)
	m.putWord(0x76f4+44, 104)
	_ = m.write32(0xe8a4+20, 3)
	_ = m.write32(0xe9de+20, 3)
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

// Original CPU cases retain complete follower/global memory, including the
// overlapping35c record view left by Helen's sound argument, raw linked-list
// repair and same-cell timer0 movement. No inherited knight path is involved.
func TestFollowerCaptiveAgainstNativeHandler(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_captive_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeCaptiveFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 446 {
		t.Fatal("native captive catalog incomplete")
	}
	hero, err := DecodeFollowerHeroRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	rules := FollowerCaptiveRules{Hero: hero}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := captiveFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []nativeMagnetCall{}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				calls = append(calls, nativeMagnetCall{Kind: "cleanup", Source: uint16(ref), Value: mode})
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(NativeRecordReference, uint8) error { return fmt.Errorf("unexpected captive town farms") }})
				return err
			}
			cb := FollowerCaptiveCallbacks{Hero: FollowerHeroCallbacks{Memory: memory, RaiseEnabled: func() bool { return false }, Raise: func(uint8, uint8) error { return fmt.Errorf("unexpected direct raise") }}, Attrition: FollowerAttritionCallbacks{Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m, ref) }, Write: func(ref NativeRecordReference, a FollowerEntryActor) error { return winEntryWrite(m, ref, a) }, Cleanup: cleanup}}
			step, err := rules.Tick(52, cb)
			if err != nil {
				t.Fatal(err)
			}
			stop := "count"
			if step.Redispatch {
				stop = "redispatch"
			}
			if stop != fixture.Stop || step.CountPopulation != (fixture.Stop == "count") {
				t.Fatal("native captive accounting/redispatch branch differs")
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("native captive callbacks got%v want%v", calls, fixture.Calls)
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
				t.Fatalf("complete native captive BSS differs: got%s want%s", got, fixture.Hash)
			}
			if fixture.RNG != 4311 || m.err != nil {
				t.Fatal("native captive RNG or memory differs")
			}
		})
	}
}
