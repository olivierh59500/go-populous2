package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeMagnetRulesAgainstOriginalRecordsAndGrid(t *testing.T) {
	data, err := os.ReadFile("testdata/magnet_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Owner, X, Y uint8
				Mode        string
			}
			AfterSHA256, GridSHA256 string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 15 {
		t.Fatalf("invalid native magnet fixture catalog: %v", err)
	}
	rules, err := DecodeNativeMagnetRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for index, fixture := range catalog.Cases {
		var records NativeRecordImage
		var globals NativeGlobalImage
		var grid NativeOccupancyState
		for at := range records.Bytes {
			records.Bytes[at] = byte(at)
		}
		for at := range globals.Bytes {
			globals.Bytes[at] = byte(NativeRecordImageSize + at)
		}
		for pos := range grid.Cells {
			grid.Cells[pos] = NativeOccupancyCell{Header: 0xa8, Tile: 15}
		}
		memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
		input := fixture.Input
		ref, _ := NativeMagnetReference(input.Owner)
		access := memory.RecordAccess()
		access.SetLinks(ref, 0, 0)
		access.SetPosition(ref, 0x2080, 0x2080)
		deity, _ := NativeDeityAddress(input.Owner)
		if _, err := memory.Write16(deity+10, uint16(ref)); err != nil {
			t.Fatal(err)
		}
		if input.Mode == "create" {
			err = rules.Create(input.Owner, input.X, input.Y, memory, &grid)
		} else {
			grid.Cells[32+32*64].Head = ref
			err = rules.Relocate(input.Owner, input.X, input.Y, memory, &grid)
		}
		if err != nil {
			t.Fatal(err)
		}
		image := append(append([]byte(nil), records.Bytes[:]...), globals.Bytes[:]...)
		var mapBytes [16384]byte
		for offset := range mapBytes {
			mapBytes[offset], _ = grid.Byte(offset)
		}
		if fmt.Sprintf("%x", sha256.Sum256(image)) != fixture.AfterSHA256 || fmt.Sprintf("%x", sha256.Sum256(mapBytes[:])) != fixture.GridSHA256 {
			t.Fatalf("native magnet case %d (%s owner %d) changed retained image/grid bytes", index, input.Mode, input.Owner)
		}
	}
}
