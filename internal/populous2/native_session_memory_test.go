package populous2

import (
	"bytes"
	"testing"
)

func TestNativeSessionMemorySharesBSSInputCODEAndScreens(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	host := nativeHostTestMemory(t)
	binding, err := NewNativeSessionMemory(host, session, w, 0x100000, 0x200000)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Begin(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	defer session.finish(nil)
	native := session.Presentation.Memory(w.nativeCleanupMemory())
	for _, patch := range []nativeHeroPatch{{0x134, 2, 123}, {0x14a, 4, 0xaabbccdd}, {0xf40, 4, 0x12345678}, {0x76f4 + 26, 4, 789}, {0xeb28, 4, 0x10203040}, {0x1127e, 2, 0x4567}} {
		switch patch.Width {
		case 2:
			err = binding.RAM.Write16(0x200000+patch.Address, uint16(patch.Value))
		case 4:
			err = binding.RAM.Write32(0x200000+patch.Address, patch.Value)
		}
		if err != nil {
			t.Fatal(err)
		}
		if value, err := native.Read8(patch.Address); err != nil || value != uint8(patch.Value>>uint((patch.Width-1)*8)) {
			t.Fatal("physical BSS write did not reach live owner", patch, err)
		}
	}
	if err := native.Write16(0xdce, 1); err != nil {
		t.Fatal(err)
	}
	if value, err := binding.RAM.Read16(0x200dce); err != nil || value != 1 {
		t.Fatal("native World write did not reach physical view", value, err)
	}
	if w.nativeCallDepth != 1 {
		t.Fatal("binding changed raw World ownership")
	}
	if err := binding.Code.Write16(0xa2a, 0x120); err != nil || session.Presentation.Input.Mouse.Image != 0x120 {
		t.Fatal("CODE cursor did not alias input", err)
	}
	session.Presentation.Input.Mouse.PositionY = 0x1234
	if value, err := binding.Code.Read16(0xa32); err != nil || value != 0x1234 {
		t.Fatal("input cursor did not alias CODE", value, err)
	}
	bitmap, err := session.Presentation.BackBuffer()
	if err != nil {
		t.Fatal(err)
	}
	address := session.Presentation.Input.long(0x1e)
	span, err := host.Span(address, 32000)
	if err != nil {
		t.Fatal(err)
	}
	if &span[0] != &bitmap[0] {
		t.Fatal("screens did not share their actual physical HUNK")
	}
	span[17] = 0xa5
	if bitmap[17] != 0xa5 {
		t.Fatal("physical screen mutation was not visible")
	}
	snapshot, err := binding.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snapshot, fileFrameMemoryBytes(t, native)) {
		t.Fatal("physical BSS snapshot diverged from live owner")
	}
	if err := binding.ImportBSS(); err == nil {
		t.Fatal("physical import overwrote a borrowed frame")
	}
}

func TestNativeSessionMemoryImportsInitializedStartupBSS(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	host := nativeHostTestMemory(t)
	physical, err := host.Span(0x200000, 0x11280)
	if err != nil {
		t.Fatal(err)
	}
	for i := range physical {
		physical[i] = byte(i*7 + 3)
	}
	binding, err := NewNativeSessionMemory(host, session, w, 0x100000, 0x200000)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.ImportBSS(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := binding.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snapshot, physical) {
		t.Fatal("startup initialized bytes were not imported exactly")
	}
	if _, err := binding.RAM.Read8(0x220000); err == nil {
		t.Fatal("binding invented physical RAM past BSS")
	}
}

func TestNativeSessionBeginRawPreservesAuthoritativeStartupRecords(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	host := nativeHostTestMemory(t)
	binding, err := NewNativeSessionMemory(host, session, w, 0x100000, 0x200000)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.BSS.Write32(0x76f4+26, 0x12345678); err != nil {
		t.Fatal(err)
	}
	if err := binding.BSS.Write16(0xdce, 0); err != nil {
		t.Fatal(err)
	}
	if err := session.BeginRaw(w, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	if w.nativeCallDepth != 1 {
		t.Fatal("raw frame did not borrow the World")
	}
	if value, err := binding.BSS.Read32(0x76f4 + 26); err != nil || value != 0x12345678 {
		t.Fatal("typed actors overwrote startup raw records", value, err)
	}
	session.finish(nil)
	if w.nativeCallDepth != 0 {
		t.Fatal("raw frame did not release ownership")
	}
}
