package populous2

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestNativeFollowerCodecMatchesOriginalEntryRecords(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_entry_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []entryFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Fixtures) != 72 {
		t.Fatalf("invalid native entry catalog: %v", err)
	}
	checked := 0
	for _, fixture := range catalog.Fixtures {
		for _, record := range fixture.Records {
			raw, err := hex.DecodeString(record.Raw)
			if err != nil || len(raw) != 52 {
				continue
			}
			ref := NativeRecordReference(record.Reference)
			if slot, ok := LocateNativeRecordImageSlot(ref); !ok || slot.Location.Pool != NativeFollowerPool {
				continue
			}
			var image NativeRecordImage
			if _, err := image.Patch(ref, 0, raw); err != nil {
				t.Fatal(err)
			}
			got, err := image.ReadFollowerEntry(ref)
			if err != nil {
				t.Fatal(err)
			}
			var memory entryFixtureMemory
			at, err := entryAddress(ref, 0, 52)
			if err != nil {
				t.Fatal(err)
			}
			copy(memory.bytes[at:at+52], raw)
			want, err := memory.read(ref)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native follower fields differ: got%+v want%+v", got, want)
			}
			before := image.Bytes
			patches, err := image.PatchFollowerEntry(ref, got, got)
			if err != nil || len(patches) != 0 || image.Bytes != before {
				t.Fatal("unchanged actor regenerated native bytes")
			}
			changed := got
			changed.Motion.Timer--
			changed.Weapon ^= 0xff
			if _, err := image.PatchFollowerEntry(ref, got, changed); err != nil {
				t.Fatal(err)
			}
			expected := append([]byte(nil), raw...)
			expected[20], expected[21] = byte(uint16(changed.Motion.Timer)>>8), byte(changed.Motion.Timer)
			expected[25] = changed.Weapon
			start := slotOffset(ref)
			if !bytes.Equal(image.Bytes[start:start+52], expected) {
				t.Fatal("native field patch changed retained/unmodeled bytes")
			}
			checked++
		}
	}
	if checked < 72 {
		t.Fatalf("native codec checked only%d records", checked)
	}
}

func slotOffset(ref NativeRecordReference) int {
	return 0x76c0 + int(int16(ref)) - NativeRecordImageStart
}
