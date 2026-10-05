package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type prepassFrameFixture struct {
	prepassFixture
	InputD, Registers [8]uint32
}

func nativePrepassFrameFixtures(t *testing.T) []prepassFrameFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/prepass_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []prepassFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 630 {
		t.Fatal("native full-register prepass corpus incomplete")
	}
	return catalog.Cases
}

func TestCommonPrepassFullFrameAgainstOriginalCPU(t *testing.T) {
	fixtures := nativePrepassFrameFixtures(t)
	r, err := DecodeCommonPrepassRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for index, fixture := range fixtures {
		m := prepassFixtureMemory(t, fixture.prepassFixture)
		frame := NativeFrameRegisterContext{D: fixture.InputD, AddressBase: 0x200000}
		read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return m.records.ReadFollowerEntry(ref) }
		write := func(ref NativeRecordReference, a FollowerEntryActor) error {
			old, err := read(ref)
			if err != nil {
				return err
			}
			_, err = m.records.PatchFollowerEntry(ref, old, a)
			return err
		}
		memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
		farms := func(ref NativeRecordReference, tile uint8) error { return town.ClearFarms(ref, tile, m.town(t)) }
		cb := CommonPrepassCallbacks{
			Frame: &frame, Read: read, Write: write,
			Tile: func(tile NativePackedTile) (uint8, error) {
				return m.byte(0xf45 + (int(uint8(tile))+int(uint8(tile>>8))*64)*4), m.err
			},
			WriteTile: func(tile NativePackedTile, value uint8) error {
				m.putByte(0xf45+(int(uint8(tile))+int(uint8(tile>>8))*64)*4, value)
				return m.err
			},
			Scenario:   func(uint8) (uint16, error) { return fixture.Input.Scenario, nil },
			ClearFarms: farms,
			CleanupFrame: func(ref NativeRecordReference, _ uint16, context *NativeFrameRegisterContext) error {
				_, err := CleanupFollowerWithFrame(ref, context, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: farms})
				return err
			},
			ClearLeaderFrame: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
				_, err := ClearFollowerLeaderWithFrame(ref, context, FollowerLeaderCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert})
				return err
			},
			Unlink: m.unlink, Sound: func(uint16) error { return nil },
		}
		if _, err := r.Tick(52, cb); err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		if frame.D != fixture.Registers {
			t.Fatalf("prepass%s seed%d full registers differ: %x / %x", fixture.Input.Name, index%3, frame.D, fixture.Registers)
		}
		bytes := append([]byte(nil), m.lower[:]...)
		bytes = append(bytes, m.records.Bytes[:]...)
		bytes = append(bytes, m.globals.Bytes[:]...)
		bytes = append(bytes, make([]byte, 24)...)
		bytes[0xeb2c], bytes[0xeb2d], bytes[0xeb2e], bytes[0xeb2f] = uint8(fixture.Input.Scenario>>8), uint8(fixture.Input.Scenario), uint8(fixture.Input.Scenario>>8), uint8(fixture.Input.Scenario)
		if got := fmt.Sprintf("%x", sha256.Sum256(bytes)); got != fixture.Hash {
			t.Fatalf("prepass%s seed%d BSS differs", fixture.Input.Name, index%3)
		}
	}
}

func TestWorldCommonPrepassFrameAgainstOriginalCPU(t *testing.T) {
	for index, fixture := range nativePrepassFrameFixtures(t) {
		m := prepassFixtureMemory(t, fixture.prepassFixture)
		raw := cleanupFixtureBytes(m)
		raw = append(raw, make([]byte, 0xeb90-len(raw))...)
		binary.BigEndian.PutUint16(raw[0xeb2c:], fixture.Input.Scenario)
		binary.BigEndian.PutUint16(raw[0xeb2e:], fixture.Input.Scenario)
		w := installNativeFixtureWorld(t, raw, nil)
		frame := NativeFrameRegisterContext{D: fixture.InputD, AddressBase: 0x200000}
		w.nativeCallDepth++
		_, err := w.CommonPrepass.Tick(52, w.nativeCommonPrepassFrameCallbacks(&frame))
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		if frame.D != fixture.Registers {
			t.Fatalf("World prepass%s seed%d registers differ: %x / %x", fixture.Input.Name, index%3, frame.D, fixture.Registers)
		}
		all := nativeFixtureWorldImage(w, raw)
		if got := fmt.Sprintf("%x", sha256.Sum256(all[:0xeb30])); got != fixture.Hash {
			t.Fatalf("World prepass%s seed%d BSS differs: %s / %s", fixture.Input.Name, index%3, got, fixture.Hash)
		}
	}
}
