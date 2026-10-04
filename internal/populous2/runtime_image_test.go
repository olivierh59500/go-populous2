package populous2

import (
	"bytes"
	"math"
	"testing"
)

func TestNativeRuntimeMemoryCrossesActorGlobalSeam(t *testing.T) {
	var records NativeRecordImage
	var globals NativeGlobalImage
	memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
	patch, err := memory.Write32(NativeRecordImageEnd-2, 0x12345678)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch.RecordPatches) != 1 || !patch.Globals || len(patch.RecordPatches[0].Slots) != 1 || patch.RecordPatches[0].Slots[0].Location.Pool != NativeEffectPool || patch.RecordPatches[0].Slots[0].Location.Index != 249 {
		t.Fatal("cross-segment patch did not report its final effect record")
	}
	if !bytes.Equal(records.Bytes[len(records.Bytes)-2:], []byte{0x12, 0x34}) || !bytes.Equal(globals.Bytes[:2], []byte{0x56, 0x78}) {
		t.Fatal("cross-segment write changed the wrong physical bytes")
	}
	value, err := memory.Read32(NativeRecordImageEnd - 2)
	if err != nil || value != 0x12345678 {
		t.Fatalf("cross-segment read differs: %x, %v", value, err)
	}
}

func TestNativeRuntimeMemoryRetainsAliasedGlobalBytes(t *testing.T) {
	var records NativeRecordImage
	var globals NativeGlobalImage
	for index := range globals.Bytes {
		globals.Bytes[index] = byte(index)
	}
	memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
	before := globals
	// A deity word at an odd offset is a raw byte-image operation. The
	// processor's address-error trap is outside this bounded storage adapter.
	address, _ := NativeDeityAddress(2)
	if _, err := memory.Write16(address+0x29, 0xabcd); err != nil {
		t.Fatal(err)
	}
	for index, value := range globals.Bytes {
		expected := before.Bytes[index]
		if index == address-NativeMagnetImageStart+0x29 {
			expected = 0xab
		} else if index == address-NativeMagnetImageStart+0x2a {
			expected = 0xcd
		}
		if value != expected {
			t.Fatal("global patch regenerated retained bytes")
		}
	}
	// The patch overlaps its source and crosses into another native record.
	if _, err := memory.Patch(NativeMagnetImageStart+2, globals.Bytes[:20]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(globals.Bytes[2:22], before.Bytes[:20]) {
		t.Fatal("overlapping global patch lost memmove semantics")
	}
}

func TestNativeRuntimeMemoryRejectsOutOfBoundsWithoutMutation(t *testing.T) {
	var records NativeRecordImage
	var globals NativeGlobalImage
	memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
	for _, address := range []int{NativeRecordImageStart - 1, NativeRuntimeImageEnd - 3, math.MinInt, math.MaxInt} {
		if _, err := memory.Write32(address, 0xffffffff); err == nil {
			t.Fatalf("accepted out-of-bounds write at %x", address)
		}
		if _, err := memory.Read32(address); err == nil {
			t.Fatalf("accepted out-of-bounds read at %x", address)
		}
	}
	if records != (NativeRecordImage{}) || globals != (NativeGlobalImage{}) {
		t.Fatal("rejected write partially modified the retained images")
	}
	if _, err := (NativeRuntimeMemory{}).Read8(NativeRecordImageStart); err == nil {
		t.Fatal("uninitialized runtime memory was accepted")
	}
}

func TestNativeRuntimeOwnerRecordsUseOriginalAddresses(t *testing.T) {
	if NativeDeityImageStart != 0xe76a || NativeRuntimeImageEnd != 0xeb18 || len((NativeGlobalImage{}).Bytes) != 984 {
		t.Fatal("native global record bounds differ")
	}
	for owner := uint8(0); owner < 3; owner++ {
		address, ok := NativeDeityAddress(owner)
		marker, markerOK := NativeMagnetReference(owner)
		if !ok || !markerOK || address != 0xe76a+int(owner)*314 || marker != NativeRecordReference(0x7080+int(owner)*14) {
			t.Fatal("owner-zero or native deity/magnet stride differs")
		}
	}
	if _, ok := NativeDeityAddress(3); ok {
		t.Fatal("invalid deity owner accepted")
	}
	if _, ok := NativeMagnetReference(255); ok {
		t.Fatal("invalid magnet owner accepted")
	}
}
