package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeHeroPatch struct {
	Address, Width int
	Value          uint32
}
type nativeHeroInput struct {
	Name, Mode       string
	Initial          []nativeHeroPatch
	DX, DY           int16
	TargetX, TargetY uint8
	Raising          bool
}
type nativeHeroChange struct {
	Address int
	Value   uint8
}
type nativeHeroCall struct {
	Kind   string
	Target uint16
	X, Y   uint8
}
type nativeHeroFixture struct {
	Input               nativeHeroInput
	Hash, Stop          string
	Changes, RaisePatch []nativeHeroChange
	D0, D1, RNG         uint32
	Calls               []nativeHeroCall
}

func heroFixtureMemory(input nativeHeroInput) *cleanupMemory {
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 1)
		m.putByte(0xf45+index*4, 15)
	}
	a := 0x76f4
	m.putByte(a, 2)
	m.putByte(a+12, 1)
	m.putByte(a+13, 2)
	m.putWord(a+6, 0x2087)
	m.putWord(a+8, 0x2093)
	m.putWord(a+10, 0x1c8)
	m.putWord(a+14, 17)
	m.putWord(a+16, 23)
	m.putByte(a+18, 16)
	m.putWord(a+20, 23)
	m.putByte(a+22, 0x24)
	_ = m.write32(a+26, 1000)
	for _, patch := range input.Initial {
		switch patch.Width {
		case 1:
			m.putByte(patch.Address, uint8(patch.Value))
		case 2:
			m.putWord(patch.Address, uint16(patch.Value))
		case 4:
			if err := m.write32(patch.Address, patch.Value); err != nil {
				m.err = err
			}
		}
	}
	if input.Raising {
		m.putWord(0xf12, 1)
	}
	return m
}

// These original CPU fixtures cover complete selector/probe/planner routines
// and stop hero dispatch at its population/redispatch/contact call boundary.
// Direct raising is genuinely executed by the CPU; its recorded external
// callback byte delta is replayed here, without claiming a second terrain port.
func TestFollowerHeroAgainstNativeDecisionsAndHoming(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_hero_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeHeroFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1571 {
		t.Fatal("native hero decision catalog incomplete")
	}
	rules, err := DecodeFollowerHeroRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := heroFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []nativeHeroCall{}
			cb := FollowerHeroCallbacks{Memory: memory, RaiseEnabled: func() bool { return fixture.Input.Raising }, Raise: func(x, y uint8) error {
				calls = append(calls, nativeHeroCall{Kind: "raise", X: x, Y: y})
				for _, patch := range fixture.RaisePatch {
					if err := m.write8(patch.Address, patch.Value); err != nil {
						return err
					}
				}
				return nil
			}, Contact: func(ref, target NativeRecordReference) error {
				if ref != 52 {
					t.Fatal("contact source changed")
				}
				calls = append(calls, nativeHeroCall{Kind: "contact", Target: uint16(target)})
				return nil
			}, Cleanup: func(ref NativeRecordReference, mode uint16) error {
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(NativeRecordReference, uint8) error { return fmt.Errorf("unexpected hero town farm cleanup") }})
				return err
			}}
			switch fixture.Input.Mode {
			case "select":
				target, err := rules.SelectTarget(52, memory)
				if err != nil {
					t.Fatal(err)
				}
				if (target != 0) != (fixture.D0 != 0) {
					t.Fatal("native target selection result differs")
				}
			case "probe":
				result, err := rules.Probe(52, NativePackedTile(m.word(0x76f4+8)&0xff00|m.word(0x76f4+6)>>8), fixture.Input.DX, fixture.Input.DY, memory)
				if err != nil {
					t.Fatal(err)
				}
				if result != int16(uint16(fixture.D0)) {
					t.Fatalf("native movement probe got%d want%d", result, int16(uint16(fixture.D0)))
				}
			case "plan":
				timer, err := rules.Plan(52, fixture.Input.TargetX, fixture.Input.TargetY, cb)
				if err != nil {
					t.Fatal(err)
				}
				if timer != uint16(fixture.D1) {
					t.Fatal("native movement timer differs")
				}
			default:
				step, err := rules.Decide(52, cb)
				if err != nil {
					t.Fatal(err)
				}
				if fixture.Input.Mode == "decision24" {
					if !step.CountPopulation || step.Redispatch {
						t.Fatal("state24 accounting branch differs")
					}
				} else if !step.Redispatch || step.CountPopulation {
					t.Fatal("state26 redispatch branch differs")
				}
				if step.Contact != (fixture.Stop == "contact") {
					t.Fatal("native contact branch differs")
				}
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("native external callbacks differ: got%v want%v", calls, fixture.Calls)
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
				t.Fatalf("complete native BSS differs: got%s want%s", got, fixture.Hash)
			}
			if m.err != nil {
				t.Fatal(m.err)
			}
		})
	}
}

func TestFollowerHeroPlannerZeroSpeedAndSameCell(t *testing.T) {
	rules, err := DecodeFollowerHeroRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	m := heroFixtureMemory(nativeHeroInput{})
	m.putByte(0x76f4+18, 0)
	memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
	cb := FollowerHeroCallbacks{Memory: memory, RaiseEnabled: func() bool { return false }, Raise: func(uint8, uint8) error { t.Fatal("unexpected raise"); return nil }}
	if timer, err := rules.Plan(52, 32, 32, cb); err != nil || timer != 0 {
		t.Fatal("native same-cell contact divides by zero")
	}
	if _, err := rules.Plan(52, 33, 32, cb); err != ErrFollowerZeroSpeed {
		t.Fatal("native zero-speed exception not reported")
	}
	if m.word(0x76f4+14) != 0 || m.word(0x76f4+16) != 0 || m.word(0x76f4+6) != 0x2087 || m.word(0x76f4+8) != 0x2093 {
		t.Fatal("native pre-division velocity writes or uncentered fractions differ")
	}
}

func TestFollowerHeroWallAddressErrorRetainsEarlierWrites(t *testing.T) {
	rules, err := DecodeFollowerHeroRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	m := heroFixtureMemory(nativeHeroInput{})
	_ = m.write32(0x76f4+26, 52769)
	m.putByte(0x5f50, 0x1a)
	m.putByte(0x5f50+12, 2)
	m.putByte(0x5f50+1, 255)
	m.putWord(0xf44+(33+32*64)*4+2, uint16(0xe890))
	memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
	if _, err := rules.Probe(52, 0x2020, 1, 0, memory); err == nil {
		t.Fatal("odd original wall animation address did not report native exception")
	}
	if m.byte(0x5f50) != 0x1c || m.byte(0x76f4+22) != 0x2a || m.word(0x76f4+10) != 0x7cc {
		t.Fatal("native pre-exception writes were rolled back")
	}
}
