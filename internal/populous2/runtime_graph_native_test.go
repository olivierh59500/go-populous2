package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeRuntimeGraphAgainstOriginalMixedOperations(t *testing.T) {
	data, err := os.ReadFile("testdata/native_occupancy.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Operations []struct {
			Operation struct {
				Name, Kind, Actor string
				X, Y              uint16
			}
			Return uint32
		}
		Snapshots []struct {
			Cells []struct {
				X, Y         int
				Header, Tile uint8
				Head         NativeRecordReference
			}
			Records []struct {
				Name                      string
				Reference, Next, Previous NativeRecordReference
				X, Y                      uint16
			}
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Operations) != 14 || len(fixture.Snapshots) != 15 {
		t.Fatal("native mixed graph fixture coverage differs")
	}
	var records NativeRecordImage
	var globals NativeGlobalImage
	var grid NativeOccupancyState
	memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
	access := memory.RecordAccess()
	names := make(map[string]NativeRecordReference)
	for _, cell := range fixture.Snapshots[0].Cells {
		grid.Cells[cell.X+cell.Y*64] = NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile, Head: cell.Head}
	}
	for _, record := range fixture.Snapshots[0].Records {
		names[record.Name] = record.Reference
		access.SetLinks(record.Reference, record.Next, record.Previous)
		access.SetPosition(record.Reference, record.X, record.Y)
	}
	for index, operation := range fixture.Operations {
		op := operation.Operation
		ref := names[op.Actor]
		switch op.Kind {
		case "insert":
			err = grid.Insert(ref, int(op.X), int(op.Y), access)
		case "remove":
			err = grid.Remove(ref, access)
		case "position":
			access.SetPosition(ref, op.X, op.Y)
		case "move":
			var changed bool
			changed, err = grid.Move(ref, op.X, op.Y, access)
			if changed != (operation.Return != 0) {
				t.Fatalf("native movement result differs after %s", op.Name)
			}
		default:
			t.Fatalf("unknown native graph operation %s", op.Kind)
		}
		if err != nil {
			t.Fatalf("native graph operation %s: %v", op.Name, err)
		}
		for _, cell := range fixture.Snapshots[index+1].Cells {
			if grid.Cells[cell.X+cell.Y*64] != (NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile, Head: cell.Head}) {
				t.Fatalf("native cell bytes differ after %s", op.Name)
			}
		}
		for _, expected := range fixture.Snapshots[index+1].Records {
			record, ok := access.Record(expected.Reference)
			if !ok || record != (NativeOccupancyRecord{Next: expected.Next, Previous: expected.Previous, X: expected.X, Y: expected.Y}) {
				t.Fatalf("retained native graph bytes differ after %s", op.Name)
			}
		}
	}
}
