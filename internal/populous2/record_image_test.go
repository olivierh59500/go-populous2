package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeRecordImageAgainstOriginalInstructionAliases(t *testing.T) {
	data, err := os.ReadFile("testdata/record_image_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Multiplier, Add int
		Cases           []struct {
			Name, Kind                string
			Reference                 NativeRecordReference
			Offset, Width, BSSAddress int
			Value, Observed           uint32
			BeforeSHA256, AfterSHA256 string
			ChangedOffsets            []int
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 11 || catalog.Multiplier != 37 || catalog.Add != 11 {
		t.Fatal("native record-image fixture catalog is incomplete")
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			var image NativeRecordImage
			for index := range image.Bytes {
				image.Bytes[index] = uint8(index*catalog.Multiplier + catalog.Add)
			}
			before := image
			if fmt.Sprintf("%x", sha256.Sum256(image.Bytes[:])) != fixture.BeforeSHA256 {
				t.Fatal("native numeric starting image differs")
			}
			if fixture.Kind == "read" {
				var got uint32
				var err error
				switch fixture.Width {
				case 1:
					var value uint8
					value, err = image.Read8(fixture.Reference, fixture.Offset)
					got = uint32(value)
				case 2:
					var value uint16
					value, err = image.Read16(fixture.Reference, fixture.Offset)
					got = uint32(value)
				case 4:
					got, err = image.Read32(fixture.Reference, fixture.Offset)
				default:
					t.Fatal("unexpected native read width")
				}
				if err != nil || got != fixture.Observed || image != before {
					t.Fatalf("native raw read differs: got%08x native%08x error%v", got, fixture.Observed, err)
				}
			} else {
				var patch NativeRecordImagePatch
				var err error
				switch fixture.Width {
				case 1:
					patch, err = image.Write8(fixture.Reference, fixture.Offset, uint8(fixture.Value))
				case 2:
					patch, err = image.Write16(fixture.Reference, fixture.Offset, uint16(fixture.Value))
				case 4:
					patch, err = image.Write32(fixture.Reference, fixture.Offset, fixture.Value)
				default:
					t.Fatal("unexpected native write width")
				}
				if err != nil || patch.BSSOffset != fixture.BSSAddress || patch.Offset != fixture.BSSAddress-NativeRecordImageStart || patch.Length != fixture.Width || !patch.Changed {
					t.Fatalf("native assigned range differs: %+v %v", patch, err)
				}
				changed := []int{}
				for index, value := range image.Bytes {
					if value != before.Bytes[index] {
						changed = append(changed, index)
					}
				}
				if !reflect.DeepEqual(changed, fixture.ChangedOffsets) {
					t.Fatal("native write cleared unrelated or retained bytes")
				}
				if fixture.Name == "Helen35c-write-hits-follower17-speed" {
					if len(patch.Slots) != 1 || patch.Slots[0].Location.Pool != NativeFollowerPool || patch.Slots[0].Location.Index != 17 || patch.BSSOffset-patch.Slots[0].Location.BSSOffset != 18 {
						t.Fatal("unaligned Helen reference did not identify follower17 speed/byte19")
					}
				}
				if fixture.Name == "long-patch-crosses-wall-scenery-boundary" {
					if len(patch.Slots) != 2 || patch.Slots[0].Location.Pool != NativeWallPool || patch.Slots[0].Location.Index != 199 || patch.Slots[1].Location.Pool != NativeSceneryPool || patch.Slots[1].Location.Index != 0 {
						t.Fatal("cross-pool write omitted an affected typed slot")
					}
				}
			}
			if fmt.Sprintf("%x", sha256.Sum256(image.Bytes[:])) != fixture.AfterSHA256 {
				t.Fatal("complete native record image differs after instruction")
			}
		})
	}
}

func TestNativeRecordImagePhysicalSlotsAndReservedRecord(t *testing.T) {
	count := 0
	for _, pool := range []struct {
		kind                 NativeRecordPool
		start, stride, count int
	}{
		{NativeWallPool, 0x5f50, 16, 200},
		{NativeSceneryPool, 0x6bd0, 14, 200},
		{NativeFollowerPool, 0x76c0, 52, 400},
		{NativeEffectPool, 0xc800, 32, 250},
	} {
		for index := range pool.count {
			address := pool.start + index*pool.stride
			reference := NativeRecordReference(uint16(address - 0x76c0))
			slot, ok := LocateNativeRecordImageSlot(reference)
			if !ok || slot.Reference != reference || slot.Location.Pool != pool.kind || slot.Location.Index != index || slot.Location.BSSOffset != address || slot.Location.Stride != pool.stride || slot.Reserved != (pool.kind == NativeFollowerPool && index == 0) {
				t.Fatalf("native physical slot differs: %+v", slot)
			}
			if _, ok := LocateNativeRecordImageSlot(reference + 1); ok {
				t.Fatal("unaligned entity reference accepted")
			}
			count++
		}
	}
	if count != 1050 || NativeRecordImageSize != 34800 {
		t.Fatal("physical reserved slot or pool bytes missing")
	}
	if _, ok := LocateNativeRecordImageSlot(0x35c); ok {
		t.Fatal("raw Helen alias incorrectly became an aligned follower")
	}
}

func TestNativeRecordImageBoundsPatchOverlapAndRoundtrip(t *testing.T) {
	var image NativeRecordImage
	for index := range image.Bytes {
		image.Bytes[index] = uint8(index)
	}
	before := image
	for _, test := range []struct {
		ref    NativeRecordReference
		offset int
	}{{0, -100000}, {0, int(^uint(0) >> 1)}, {0, -int(^uint(0)>>1) - 1}, {0x8000, 0}, {0x7080, 0}} {
		if _, err := image.Read8(test.ref, test.offset); err == nil {
			t.Fatal("out-of-image byte read accepted")
		}
		if _, err := image.Write32(test.ref, test.offset, 0); err == nil || image != before {
			t.Fatal("invalid patch partially changed the image")
		}
	}
	// Arbitrary aliases and negative offsets are not entity lookups. Odd raw
	// addresses remain supported as byte-image operations, as documented.
	if value, err := image.Read16(1, 0); err != nil || value != uint16(image.Bytes[0x1771])<<8|uint16(image.Bytes[0x1772]) {
		t.Fatal("bounded unaligned byte image read rejected")
	}
	if value, err := image.Read8(0, -1); err != nil || value != image.Bytes[0x176f] {
		t.Fatal("signed raw alias lost")
	}
	patch, err := image.Patch(0, 0, image.Bytes[0x176e:0x1774])
	if err != nil || !patch.Changed || len(patch.Slots) != 1 || !patch.Slots[0].Reserved {
		t.Fatal("overlapping patch or reserved-slot metadata differs")
	}
	for index := range 6 {
		if image.Bytes[0x1770+index] != before.Bytes[0x176e+index] {
			t.Fatal("overlapping patch failed memmove semantics")
		}
	}
	patch, err = image.Patch(0, 0, image.Bytes[0x1770:0x1776])
	if err != nil || patch.Changed {
		t.Fatal("unchanged patch reported a mutation")
	}
	data, err := json.Marshal(image)
	if err != nil {
		t.Fatal(err)
	}
	var restored NativeRecordImage
	if err := json.Unmarshal(data, &restored); err != nil || restored != image {
		t.Fatal("raw image serialization lost retained bytes")
	}
}
