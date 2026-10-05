package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeStormInput struct {
	Name, Mode                   string
	Owner                        uint16
	X, Y                         uint8
	Seed                         uint32
	FreeStart, FreeCount, Caller int
	Initial                      []nativeHeroPatch
	Links                        []uint16
	Ticks                        int
}
type nativeStormCall struct {
	Kind      string
	Reference uint16
}
type nativeStormTrace struct {
	Update                   int
	Hash                     string
	RNG                      uint32
	RandomDraws, DamageScans int
}
type nativeStormFixture struct {
	CallerWordAfter                    uint16
	Input                              nativeStormInput
	Hash                               string
	Changes                            []nativeHeroChange
	Admitted                           bool
	RandomDraws, Attempts, DamageScans int
	Calls                              []nativeStormCall
	Trace                              []nativeStormTrace
	RNG                                uint32
}

func stormFixtureMemory(input nativeStormInput) *cleanupMemory {
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 0xa8)
		m.putByte(0xf45+index*4, 15)
		m.putByte(0x4f44+index, 0x77)
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
	for owner := uint8(1); owner <= 2; owner++ {
		god := 0xe76a + int(owner)*314
		marker := 0xe740 + int(owner)*14
		m.putWord(god+10, uint16(marker-0x76c0))
		m.putWord(god+0x44, 120)
		m.putByte(marker+12, owner)
		m.putWord(marker+6, uint16(10+owner)<<8|128)
		m.putWord(marker+8, 0x0a80)
	}
	if input.Mode == "tick" {
		for off := range 32 {
			m.putByte(0xc800+off, 0)
		}
		m.putByte(0xc800, 0x36)
		m.putByte(0xc800+12, uint8(input.Owner))
		m.putByte(0xc800+22, 0x2c)
		m.putWord(0xc800+6, uint16(input.X)<<8|128)
		m.putWord(0xc800+8, uint16(input.Y)<<8|128)
		m.putWord(0xc800+10, 0xce4)
		m.putWord(0xc800+24, 200)
	}
	if input.Caller >= 0 && input.Caller+28 <= NativeRuntimeImageEnd {
		m.putWord(input.Caller+26, 0xbeef)
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
	refs := append([]uint16{0x708e, 0x709c}, input.Links...)
	if input.Mode == "tick" {
		refs = append(refs, 0x5140)
	}
	for _, ref := range refs {
		if err := m.insert(NativeRecordReference(ref)); err != nil {
			m.err = err
		}
	}
	return m
}

func stormFixtureHash(m *cleanupMemory) string {
	var image [NativeRuntimeImageEnd]byte
	copy(image[:NativeRecordImageStart], m.lower[:])
	copy(image[NativeRecordImageStart:NativeMagnetImageStart], m.records.Bytes[:])
	copy(image[NativeMagnetImageStart:], m.globals.Bytes[:])
	return fmt.Sprintf("%x", sha256.Sum256(image[:]))
}

// Original CPU observations cover creator admission flags, all terrain codes,
// real tree/follower/hero/town damage, cooldown and finite ending behavior.
func TestStormAgainstOriginalCreatorAndRuntime(t *testing.T) {
	data, err := os.ReadFile("testdata/storm_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeStormFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 710 {
		t.Fatal("native storm catalog incomplete")
	}
	bundle := testBundle(t)
	rules, err := DecodeStormRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeTownCombatRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, action := range bundle.Actions {
		if action.Command == 64 {
			if action.Spell != Storm || action.Handler != 0x17ad4 {
				t.Fatal("storm actioncatalog identity differs")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("native storm command64 missing")
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := stormFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			var commandImage [0xeb90 - NativeRuntimeImageEnd]byte
			if fixture.Input.Caller+26 >= NativeRuntimeImageEnd {
				binary.BigEndian.PutUint16(commandImage[fixture.Input.Caller+26-NativeRuntimeImageEnd:], 0xbeef)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: func(a int, v uint16) error {
				if a >= NativeRuntimeImageEnd && a+2 <= 0xeb90 {
					binary.BigEndian.PutUint16(commandImage[a-NativeRuntimeImageEnd:], v)
					return nil
				}
				return m.write16(a, v)
			}, Write32: m.write32}
			rng, draws, damage := fixture.Input.Seed, 0, 0
			calls := []nativeStormCall{}
			clear := func(ref NativeRecordReference, tile uint8) error {
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: clear})
				return err
			}
			cb := StormCallbacks{Memory: memory, Random: func() uint16 {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				return uint16(rng >> 8 & 0x7fff)
			}, Link: func(ref NativeRecordReference) error {
				calls = append(calls, nativeStormCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}, Unlink: func(ref NativeRecordReference) error {
				calls = append(calls, nativeStormCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}, DestroyTown: func(ref NativeRecordReference) error {
				calls = append(calls, nativeStormCall{Kind: "destroy", Reference: uint16(ref)})
				_, err := combat.Destroy(ref, TownCombatCallbacks{Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m, ref) }, Write: func(ref NativeRecordReference, a FollowerEntryActor) error { return winEntryWrite(m, ref, a) }, ClearFarms: clear, Cleanup: cleanup, EvaluateTown: func(NativeRecordReference) (int, error) { return 0, fmt.Errorf("unexpectedstorm townreform") }})
				return err
			}}
			if fixture.Input.Mode == "create" {
				step, err := rules.Create(fixture.Input.Owner, fixture.Input.X, fixture.Input.Y, fixture.Input.Caller, cb)
				if err != nil {
					t.Fatal(err)
				}
				if step.Admitted != fixture.Admitted || step.Attempts != fixture.Attempts || step.RandomDraws != fixture.RandomDraws {
					t.Fatalf("native storm admission/count differs: %+v wantadmit%v attempts%d draws%d", step, fixture.Admitted, fixture.Attempts, fixture.RandomDraws)
				}
			} else {
				for _, trace := range fixture.Trace {
					step, err := rules.Tick(0x5140, cb)
					if err != nil {
						t.Fatal(err)
					}
					damage += step.DamageScans
					if stormFixtureHash(m) != trace.Hash || rng != trace.RNG || draws != trace.RandomDraws || damage != trace.DamageScans {
						t.Fatalf("native storm update%d image/RNG/damage differs", trace.Update)
					}
				}
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("native callback sequence differs: got%v want%v", calls, fixture.Calls)
			}
			if stormFixtureHash(m) != fixture.Hash {
				for _, change := range fixture.Changes {
					got, _ := m.read8(change.Address)
					if got != change.Value {
						t.Errorf("native byte%x got%x want%x", change.Address, got, change.Value)
					}
				}
				t.Fatal("complete native storm BSS differs")
			}
			callerWord := uint16(0)
			if fixture.Input.Caller+26 >= NativeRuntimeImageEnd {
				callerWord = binary.BigEndian.Uint16(commandImage[fixture.Input.Caller+26-NativeRuntimeImageEnd:])
			} else {
				callerWord = m.word(fixture.Input.Caller + 26)
			}
			if callerWord != fixture.CallerWordAfter {
				t.Fatal("original command/script caller word1a alias differs")
			}
			if rng != fixture.RNG || draws != fixture.RandomDraws || damage != fixture.DamageScans {
				t.Fatal("native storm final RNG/damage counts differ")
			}
		})
	}
}
