package populous2

import (
	"bytes"
	"testing"
)

func TestNativeRuntimeWorldMarkersShareMapAndPreserveGlobalSaveBytes(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	for owner := uint8(1); owner <= 2; owner++ {
		ref, _ := NativeMagnetReference(owner)
		record, ok := w.runtimeMemory().RecordAccess().Record(ref)
		if !ok || !w.Occupancy.Magnets[owner].Linked || record != w.Occupancy.Magnets[owner].Record {
			t.Fatal("normal game marker is missing from its mixed map chain")
		}
		deity, _ := NativeDeityAddress(owner)
		pointer, err := w.runtimeMemory().Read16(deity + 10)
		if err != nil || pointer != uint16(ref) {
			t.Fatalf("native deity marker reference differs: %x, %v", pointer, err)
		}
	}
	if err := w.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
	deity, _ := NativeDeityAddress(1)
	if _, err := w.runtimeMemory().Write32(deity+0x22, 0xa1b2c3d4); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	value, err := restored.runtimeMemory().Read32(deity + 0x22)
	if err != nil || value != 0xa1b2c3d4 || !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("save regenerated retained deity/marker fields")
	}
}

func TestNativeRuntimeSaveRejectsMarkerGraphDisagreement(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := w.Snapshot()
	ref, _ := NativeMagnetReference(1)
	memory := NativeRuntimeMemory{Records: &snapshot.RecordImage, Globals: &snapshot.NativeGlobals}
	if _, err := memory.Write16(0x76c0+int(ref)+6, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(testBundle(t), snapshot); err == nil {
		t.Fatal("save accepted raw marker coordinates inconsistent with its graph")
	}
}

func TestNativeRuntimeMigratesEarlierSaveMarkersWithoutDuplicateLinks(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := w.Snapshot()
	// An old save has no global image. This also exercises a diagnostic
	// snapshot with already mapped markers: migration must remove them first.
	snapshot.Version, snapshot.NativeGlobals = 15, NativeGlobalImage{}
	restored, err := Restore(testBundle(t), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
	for owner := 1; owner < NativeMagnetCount; owner++ {
		if !restored.Occupancy.Magnets[owner].Linked {
			t.Fatal("old save did not receive its native marker records")
		}
	}
}
