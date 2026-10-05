package populous2

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func nativeHostTestMemory(t *testing.T) *NativeHostMemory {
	t.Helper()
	m, err := NewNativeHunkMemory(testBundle(t).Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNativeHostHunkMemoryUsesActualAllocationsAndRelocations(t *testing.T) {
	exe := testBundle(t).Executable
	m := nativeHostTestMemory(t)
	base := resourceFrameInitialRAM(t) // Independent reference relocator.
	bases := []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000}
	for i, hunk := range exe.Hunks {
		span, err := m.Span(bases[i], int(hunk.AllocatedBytes))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(span, base[int(bases[i]):int(bases[i])+int(hunk.AllocatedBytes)]) {
			t.Fatal("relocated HUNK differs", i)
		}
		if _, err := m.Span(bases[i]+uint32(hunk.AllocatedBytes), 1); err == nil {
			t.Fatal("invented RAM past real HUNK allocation", i)
		}
	}
	if _, err := m.Memory().Read8(0); err == nil {
		t.Fatal("unprovided low RAM was synthesized")
	}
	if _, err := m.Memory().Read16(0x100001); err == nil {
		t.Fatal("unaligned physical word was accepted")
	}
	if _, err := NewNativeHunkMemory(exe, []uint32{0x100000, 0x100002, 0x300000, 0x400000, 0x500000, 0x600000}); err == nil {
		t.Fatal("overlapping HUNK layout was accepted")
	}
}

func TestNativeHostMappedRAMAliasesAndAdjacentBoundaries(t *testing.T) {
	m := &NativeHostMemory{}
	a, b := []byte{1, 2, 3, 4}, []byte{5, 6, 7, 8}
	if err := m.MapRegion(NativeHostRegion{Base: 0x1000, Bytes: a}); err != nil {
		t.Fatal(err)
	}
	if err := m.MapRegion(NativeHostRegion{Base: 0x1004, Bytes: b}); err != nil {
		t.Fatal(err)
	}
	mem := m.Memory()
	if value, err := mem.Read32(0x1002); err != nil || value != 0x03040506 {
		t.Fatal("provided adjacent regions were not readable", value, err)
	}
	if err := mem.Write32(0x1002, 0xaabbccdd); err != nil || !bytes.Equal(a, []byte{1, 2, 0xaa, 0xbb}) || !bytes.Equal(b, []byte{0xcc, 0xdd, 7, 8}) {
		t.Fatal("physical mapped RAM did not alias its owners", err, a, b)
	}
	if _, err := m.Span(0x1002, 4); err == nil {
		t.Fatal("detached allocations were exposed as one contiguous slice")
	}
	if err := m.MapRegion(NativeHostRegion{Base: 0x1002, Bytes: make([]byte, 4)}); err == nil {
		t.Fatal("overlapping physical region accepted")
	}
	if err := mem.Write32(0x1006, 0x11223344); err == nil || binary.BigEndian.Uint16(b[2:]) != 0x1122 {
		t.Fatal("missing region did not retain completed write prefix", err, b)
	}
}

func TestNativeHostBitmapWindowAliasesActualChipAllocation(t *testing.T) {
	m := nativeHostTestMemory(t)
	window, err := m.BitmapWindow(0x500408)
	if err != nil {
		t.Fatal(err)
	}
	span, err := m.Span(0x500408, 32000)
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Bytes) != NativeFrameChipBytes || window.BitmapOffset != 0x408 || &span[0] != &window.Bytes[window.BitmapOffset] {
		t.Fatal("bitmap window replaced actual chip backing")
	}
	if _, err := m.BitmapWindow(0x500408 + 32001); err == nil {
		t.Fatal("unavailable bitmap tail was padded")
	}
}
