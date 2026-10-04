package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeNeutralCreatorAgainstCompleteOriginalImages(t *testing.T) {
	data, err := os.ReadFile("testdata/neutral_creator_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name       string
				D0, D1, D2 uint32
				Free       []int
			}
			D0                      uint32
			ImageSHA256, GridSHA256 string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 105 {
		t.Fatalf("native neutral creator catalog incomplete: %v", err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			var records NativeRecordImage
			var globals NativeGlobalImage
			var grid NativeOccupancyState
			for index := range records.Bytes {
				records.Bytes[index] = uint8(index)
			}
			for index := range globals.Bytes {
				globals.Bytes[index] = uint8(index + NativeRecordImageSize)
			}
			for index := range grid.Cells {
				grid.Cells[index] = NativeOccupancyCell{Header: 0xa8, Tile: 15}
			}
			memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
			for slot := 1; slot < 400; slot++ {
				if _, err := records.Write8(NativeRecordReference(slot*52), 12, 1); err != nil {
					t.Fatal(err)
				}
			}
			for _, slot := range fixture.Input.Free {
				if _, err := records.Write8(NativeRecordReference(slot*52), 12, 0); err != nil {
					t.Fatal(err)
				}
			}
			cb := NativeNeutralCallbacks{Memory: FollowerCleanupMemory{Read8: memory.Read8, Read16: memory.Read16, Read32: memory.Read32,
				Write8:  func(at int, value uint8) error { _, err := memory.Write8(at, value); return err },
				Write16: func(at int, value uint16) error { _, err := memory.Write16(at, value); return err },
				Write32: func(at int, value uint32) error { _, err := memory.Write32(at, value); return err }},
				Insert: func(ref NativeRecordReference) error {
					record, ok := memory.RecordAccess().Record(ref)
					if !ok {
						return fmt.Errorf("missing neutral graph prefix")
					}
					return grid.Insert(ref, int(record.X>>8), int(record.Y>>8), memory.RecordAccess())
				}}
			step, err := CreateNativeNeutral(FollowerCleanupRegisters{D0: fixture.Input.D0, D1: fixture.Input.D1, D2: fixture.Input.D2}, cb)
			if err != nil {
				t.Fatal(err)
			}
			if step.Created != (fixture.D0 != 0) {
				t.Fatal("native creation return differs")
			}
			image := append(append([]byte(nil), records.Bytes[:]...), globals.Bytes[:]...)
			var mapBytes [16384]byte
			for index := range mapBytes {
				mapBytes[index], _ = grid.Byte(index)
			}
			if fmt.Sprintf("%x", sha256.Sum256(image)) != fixture.ImageSHA256 || fmt.Sprintf("%x", sha256.Sum256(mapBytes[:])) != fixture.GridSHA256 {
				t.Fatal("complete neutral image/grid bytes differ")
			}
		})
	}
}
