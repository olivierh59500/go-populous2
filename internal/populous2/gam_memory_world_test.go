package populous2

import (
	"bytes"
	"testing"
)

func TestNativeGAMWorldMemoryRetainsAllRegionSeams(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	m := w.nativeCleanupMemory()
	// These are actual adjacent original BSS regions, not independent buffers.
	if err := m.Write32(0x4f42, 0x11223344); err != nil {
		t.Fatal(err)
	}
	if w.Occupancy.Grid.Cells[4095].Head != 0x1122 || w.NativeOverlays[0] != 0x33 || w.NativeOverlays[1] != 0x44 {
		t.Fatal("grid/overlay seam lost native byte aliases")
	}
	if err := m.Write32(0x5f42, 0x55667788); err != nil {
		t.Fatal(err)
	}
	if w.NativeOverlays[4094] != 0x55 || w.NativeOverlays[4095] != 0x66 || w.NativeViewBytes[0] != 0x77 || w.NativeViewBytes[1] != 0x88 {
		t.Fatal("overlay/view seam lost native byte aliases")
	}
	if err := m.Write16(0x5f4f, 0x99aa); err != nil {
		t.Fatal(err)
	}
	if w.NativeViewBytes[11] != 0x99 || w.RecordImage.Bytes[0] != 0xaa {
		t.Fatal("view/actor seam lost native byte aliases")
	}
	if err := m.Write32(0xe73e, 0xbbccddee); err != nil {
		t.Fatal(err)
	}
	if w.RecordImage.Bytes[len(w.RecordImage.Bytes)-2] != 0xbb || w.RecordImage.Bytes[len(w.RecordImage.Bytes)-1] != 0xcc || w.NativeGlobals.Bytes[0] != 0xdd || w.NativeGlobals.Bytes[1] != 0xee {
		t.Fatal("actor/global seam lost native byte aliases")
	}
	if err := m.Write32(0xeb16, 0x01020304); err != nil {
		t.Fatal(err)
	}
	if w.NativeGlobals.Bytes[len(w.NativeGlobals.Bytes)-2] != 1 || w.NativeGlobals.Bytes[len(w.NativeGlobals.Bytes)-1] != 2 || w.NativeCommandBytes[0] != 3 || w.NativeCommandBytes[1] != 4 {
		t.Fatal("global/session seam lost native byte aliases")
	}
	for _, address := range []int{0x4f42, 0x5f42, 0xe73e, 0xeb16} {
		if _, err := m.Read32(address); err != nil {
			t.Fatalf("retained long seam%x: %v", address, err)
		}
	}
	if _, err := m.Read8(0xdc3); err == nil {
		t.Fatal("unretained prefix was replaced with invented zero bytes")
	}
}

func TestNativeGAMViewBackingSnapshot25AndMigration(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	w.NativeViewBytes = [12]byte{0, 8, 0, 4, 0, 1, 0, 2, 0, 3, 0, 4}
	w.NativeOverlays[100] = 1
	snapshot := w.Snapshot()
	if snapshot.Version != SaveVersion || snapshot.NativeViewBytes != w.NativeViewBytes {
		t.Fatal("snapshot omitted native camera/hit-test words")
	}
	restored, err := Restore(b, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if restored.NativeViewBytes != w.NativeViewBytes || restored.NativeOverlays[100] != 1 {
		t.Fatal("native view/overlay state did not roundtrip")
	}
	image, err := CaptureNativeGAM(restored.nativeCleanupMemory(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(image.Bytes[0x5f44-NativeGAMStart:0x5f50-NativeGAMStart], w.NativeViewBytes[:]) {
		t.Fatal("GAM does not contain the exact saved view words")
	}
	old := snapshot
	old.Version = 24
	restored, err = Restore(b, old)
	if err != nil {
		t.Fatal(err)
	}
	if restored.NativeViewBytes != [12]byte{} {
		t.Fatal("older Go snapshots retained a field they never serialized")
	}
}
