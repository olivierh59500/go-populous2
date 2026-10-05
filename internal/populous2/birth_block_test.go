package populous2

import (
	"bytes"
	"testing"
)

func TestNativeBirthBlockByteAliasesAndSaveMigration(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	m := w.nativeCleanupMemory()
	if err := m.Write32(0xdc2, 0xabcd1234); err != nil {
		t.Fatal(err)
	}
	if w.NativeBirthBlockWord != 0xabcd || !w.NativeBirthBlocked || w.NativeControlBytes[0] != 0x12 || w.NativeControlBytes[1] != 0x34 {
		t.Fatal("birth/control seam lost raw bytes")
	}
	if err := m.Write8(0xdc2, 0); err != nil {
		t.Fatal(err)
	}
	if word, err := m.Read16(0xdc2); err != nil || word != 0xcd || !w.NativeBirthBlocked {
		t.Fatal("partial write changed the surviving low byte")
	}
	s := w.Snapshot()
	loaded, err := Restore(b, s)
	if err != nil || loaded.NativeBirthBlockWord != 0xcd || !loaded.NativeBirthBlocked {
		t.Fatal("raw birth-block save continuation lost", err)
	}
	old := s
	old.Version = 28
	loaded, err = Restore(b, old)
	if err != nil || loaded.NativeBirthBlockWord != 1 || !loaded.NativeBirthBlocked {
		t.Fatal("older bool-only save did not migrate", err)
	}
	w.NativeBirthBlocked = false
	if word, err := m.Read16(0xdc2); err != nil || word != 0 {
		t.Fatal("public bool clear did not reach raw state")
	}
	w.NativeBirthBlocked = true
	if word, err := m.Read16(0xdc2); err != nil || word != 1 {
		t.Fatal("public bool set did not reach raw state")
	}
	w.beginNativeFollowerPass()
	if word, err := m.Read16(0xdc2); err != nil || word != 0 || w.NativeBirthBlocked {
		t.Fatal("follower pass did not execute CLR.W DC2")
	}
	before, err := w.ExportNativeGAM()
	if err != nil {
		t.Fatal(err)
	}
	w.setNativeBirthBlockWord(0xff00)
	after, err := w.ExportNativeGAM()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("birth-block backing changed original GAM transfer extent")
	}
}
