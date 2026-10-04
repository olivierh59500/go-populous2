package amiga

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func smallExecutable() []byte {
	values := []uint32{1011, 0, 2, 0, 1, 0x40000002, 0x100000, 1001, 2, 0x4e754e71, 0x00000000, 1004, 1, 1, 4, 0, 1008, 1, 0x73746172, 0, 0, 1010, 1003, 0x100000, 1010}
	var b bytes.Buffer
	for _, v := range values {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	return b.Bytes()
}

func TestHunkReadsRelocationsSymbolsAndBSS(t *testing.T) {
	data := smallExecutable()
	exe, err := ParseExecutable(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(exe.Hunks) != 2 || exe.Hunks[0].Kind != "code" || exe.Hunks[0].Memory != "chip" || exe.Hunks[1].AllocatedBytes != 4<<20 || len(exe.Hunks[1].Data) != 0 {
		t.Fatal("wrong segment allocation")
	}
	h := exe.Hunks[0]
	if len(h.Relocations) != 1 || h.Relocations[0].Target != 1 || h.Relocations[0].Offsets[0] != 4 || len(h.Symbols) != 1 || h.Symbols[0].Name != "star" {
		t.Fatal("missing relocation or symbol")
	}
	clear(data)
	if len(h.Data) != 8 || h.Data[0] != 0x4e {
		t.Fatal("code aliases caller's buffer")
	}
}

func TestHunkRejectsTruncationAndOutOfBoundsRelocations(t *testing.T) {
	data := smallExecutable()
	for end := 0; end < len(data); end++ {
		if _, err := ParseExecutable(data[:end]); err == nil {
			t.Fatalf("accepted truncation at %d", end)
		}
	}
	bad := bytes.Clone(data)
	binary.BigEndian.PutUint32(bad[56:], 8)
	if _, err := ParseExecutable(bad); err == nil {
		t.Fatal("relocation outside allocation accepted")
	}
	bad = append(bytes.Clone(data), 0, 0, 0, 1)
	if _, err := ParseExecutable(bad); err == nil {
		t.Fatal("nonzero trailer was ignored")
	}
}

func FuzzExecutable(f *testing.F) {
	f.Add(smallExecutable())
	f.Add([]byte{0, 0, 3, 0xf3})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_, _ = ParseExecutable(data)
	})
}
