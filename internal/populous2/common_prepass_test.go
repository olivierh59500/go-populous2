package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type prepassFixture struct {
	Input struct {
		Name                            string
		Owner, Kind, Tile, State, Flags uint8
		Hero, Scenario, Extra48         uint16
		Population                      int32
	}
	Writes                  []cleanupWrite
	Hash, SourceRaw, GodRaw string
	Sounds                  []struct {
		Argument  uint16
		SourceRaw string
	}
	Callbacks []string
}

func prepassFixtureMemory(t *testing.T, f prepassFixture) *cleanupMemory {
	t.Helper()
	m := &cleanupMemory{}
	c := f.Input
	at := 0x76f4
	for i := range 4096 {
		m.putByte(0xf44+i*4, 0x88)
		m.putByte(0xf44+i*4+1, 15)
	}
	m.putByte(0xf44+(32+32*64)*4+1, c.Tile)
	m.putByte(0x4f44+32+32*64, 7)
	m.putByte(at, c.Kind)
	m.putByte(at+1, 0)
	m.putByte(at+12, c.Owner)
	m.putByte(at+13, c.Flags)
	m.putByte(at+19, 0xaa)
	m.putByte(at+22, c.State)
	m.putByte(at+23, 2)
	m.putByte(at+18, 20)
	m.putWord(at+6, 0x2011)
	m.putWord(at+8, 0x202b)
	m.putWord(at+10, 8)
	m.write32(at+26, uint32(c.Population))
	m.putWord(at+40, c.Hero)
	m.putWord(at+48, c.Extra48)
	for owner := uint8(0); owner < 3; owner++ {
		god := 0xe76a + int(owner)*314
		marker := 0xe740 + int(owner)*14
		ref := NativeRecordReference(uint16(marker - 0x76c0))
		m.putWord(god+68, 30)
		m.putWord(god+8, 52)
		m.putWord(god+10, uint16(ref))
		m.putByte(marker, 20)
		m.putByte(marker+12, owner)
		m.putWord(marker+6, 0x0880)
		m.putWord(marker+8, 0x0980)
		if err := m.insert(ref); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.insert(52); err != nil {
		t.Fatal(err)
	}
	if m.err != nil {
		t.Fatal(m.err)
	}
	m.record = true
	return m
}
func TestCommonPrepassAgainstCompleteOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/common_prepass_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []prepassFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 210 {
		t.Fatal("native commonprepass catalog incomplete")
	}
	r, err := DecodeCommonPrepassRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := prepassFixtureMemory(t, f)
			read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return m.records.ReadFollowerEntry(ref) }
			write := func(ref NativeRecordReference, a FollowerEntryActor) error {
				old, err := read(ref)
				if err != nil {
					return err
				}
				_, err = m.records.PatchFollowerEntry(ref, old, a)
				return err
			}
			mem := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			sounds := []uint16{}
			soundIndex := 0
			cb := CommonPrepassCallbacks{Read: read, Write: write, Tile: func(p NativePackedTile) (uint8, error) {
				return m.byte(0xf44 + (int(uint8(p))+int(uint8(p>>8))*64)*4 + 1), m.err
			}, WriteTile: func(p NativePackedTile, tile uint8) error {
				m.putByte(0xf44+(int(uint8(p))+int(uint8(p>>8))*64)*4+1, tile)
				return m.err
			}, Scenario: func(uint8) (uint16, error) { return f.Input.Scenario, nil }, ClearFarms: func(ref NativeRecordReference, tile uint8) error { return town.ClearFarms(ref, tile, m.town(t)) }, Cleanup: func(ref NativeRecordReference, mode uint16) error {
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: mem, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(ref NativeRecordReference, tile uint8) error { return town.ClearFarms(ref, tile, m.town(t)) }})
				return err
			}, ClearLeader: func(ref NativeRecordReference) error {
				_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: mem, Unlink: m.unlink, Insert: m.insert})
				return err
			}, Unlink: m.unlink, Sound: func(raw uint16) error {
				if soundIndex >= len(f.Sounds) {
					return fmt.Errorf("unexpected native terrain sound")
				}
				s := f.Sounds[soundIndex]
				soundIndex++
				at := cleanupRecordAddress(52) - NativeRecordImageStart
				if raw != s.Argument || hex.EncodeToString(m.records.Bytes[at:at+52]) != s.SourceRaw {
					return fmt.Errorf("native sound boundary record/argument differs")
				}
				sounds = append(sounds, raw)
				return nil
			}}
			step, err := r.Tick(52, cb)
			if err != nil {
				t.Fatal(err)
			}
			if m.err != nil {
				t.Fatal(m.err)
			}
			at := 0x76f4 - NativeRecordImageStart
			if got := hex.EncodeToString(m.records.Bytes[at : at+52]); got != f.SourceRaw {
				t.Fatalf("native source fields differ:\ngot%s\nnative%s", got, f.SourceRaw)
			}
			bytes := append([]byte(nil), m.lower[:]...)
			bytes = append(bytes, m.records.Bytes[:]...)
			bytes = append(bytes, m.globals.Bytes[:]...)
			bytes = append(bytes, make([]byte, 24)...)
			bytes[0xeb2c], bytes[0xeb2d], bytes[0xeb2e], bytes[0xeb2f] = uint8(f.Input.Scenario>>8), uint8(f.Input.Scenario), uint8(f.Input.Scenario>>8), uint8(f.Input.Scenario)
			if fmt.Sprintf("%x", sha256.Sum256(bytes)) != f.Hash {
				t.Fatal("complete native prepass BSS/grid/overlay/deity image differs")
			}
			want := []uint16{}
			for _, s := range f.Sounds {
				want = append(want, s.Argument)
			}
			if !reflect.DeepEqual(sounds, want) {
				t.Fatal("native sound ordering differs")
			}
			if step.OwnerSkipped != (f.Input.Owner == 3) {
				t.Fatal("neutral owner prepass bypass differs")
			}
		})
	}
}
func TestCommonPrepassRejectsUnboundedPointersAndMissingCallbacks(t *testing.T) {
	r, err := DecodeCommonPrepassRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Tick(52, CommonPrepassCallbacks{}); err == nil {
		t.Fatal("missing callbacks accepted")
	}
	if _, err := r.imageWord(65535); err == nil {
		t.Fatal("unbounded plague animation pointer accepted")
	}
}
