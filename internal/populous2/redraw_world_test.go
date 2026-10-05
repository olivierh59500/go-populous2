package populous2

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeRedrawFIFOAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/redraw_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Offset, Cursor, Word uint16
			Address              int
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 15 {
		t.Fatal("native redraw fixture catalog incomplete")
	}
	for _, fixture := range catalog.Cases {
		w, err := NewWorld(testBundle(t), 0, false)
		if err != nil {
			t.Fatal(err)
		}
		m := w.nativeCleanupMemory()
		if fixture.Address < 0x11280 {
			if err := m.Write16(fixture.Address, 0); err != nil {
				t.Fatal(err)
			}
		}
		if err := m.Write16(0xeb6e, fixture.Offset); err != nil {
			t.Fatal(err)
		}
		accepted, err := w.AppendNativeRedrawWord(0xa083)
		if err != nil {
			t.Fatal(err)
		}
		cursor, err := m.Read16(0xeb6e)
		if err != nil {
			t.Fatal(err)
		}
		word := uint16(0)
		if accepted {
			word, err = m.Read16(fixture.Address)
			if err != nil {
				t.Fatal(err)
			}
		}
		if accepted != (fixture.Address < 0x11280) || cursor != fixture.Cursor || word != fixture.Word {
			t.Fatalf("native redraw offset%x differs: accepted%v cursor%x/%x word%x/%x", fixture.Offset, accepted, cursor, fixture.Cursor, word, fixture.Word)
		}
	}
}

func TestNativeRedrawBackingSeamAndSave28Continuation(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	m := w.nativeCleanupMemory()
	if err := m.Write32(0xeb8e, 0x11223344); err != nil {
		t.Fatal(err)
	}
	if w.NativeCommandBytes[0x76] != 0x11 || w.NativeCommandBytes[0x77] != 0x22 || w.NativeRedrawBytes[0] != 0x33 || w.NativeRedrawBytes[1] != 0x44 {
		t.Fatal("command/redraw seam lost exact bytes")
	}
	if err := m.Write16(0x1127e, 0xabcd); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Read8(0x11280); err == nil {
		t.Fatal("redraw backing exceeds native BSS allocation")
	}
	if err := m.Write16(0xeb6e, 0x20); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 100; n++ {
		if accepted, err := w.AppendNativeRedrawWord(uint16(n)); err != nil || !accepted {
			t.Fatal("native redraw queue stopped prematurely")
		}
	}
	s := w.Snapshot()
	if s.Version != SaveVersion {
		t.Fatal("redraw snapshot version stale")
	}
	loaded, err := Restore(b, s)
	if err != nil {
		t.Fatal(err)
	}
	for n := 100; n < 200; n++ {
		if _, err := w.AppendNativeRedrawWord(uint16(n)); err != nil {
			t.Fatal(err)
		}
		if _, err := loaded.AppendNativeRedrawWord(uint16(n)); err != nil {
			t.Fatal(err)
		}
	}
	if w.NativeCommandBytes != loaded.NativeCommandBytes || w.NativeRedrawBytes != loaded.NativeRedrawBytes {
		t.Fatal("saved native redraw continuation differs")
	}
	old := s
	old.Version = 27
	legacy, err := Restore(b, old)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.NativeRedrawBytes != [0x26f0]byte{} {
		t.Fatal("older save retained redraw data it never serialized")
	}
	before, err := w.ExportNativeGAM()
	if err != nil {
		t.Fatal(err)
	}
	w.NativeRedrawBytes[123] ^= 255
	after, err := w.ExportNativeGAM()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("redraw tail changed original GAM transfer endpoint")
	}
}
