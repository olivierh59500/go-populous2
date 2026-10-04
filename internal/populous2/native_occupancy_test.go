package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func occupancyAccess(records map[NativeRecordReference]NativeOccupancyRecord) NativeRecordAccess {
	return NativeRecordAccess{
		Record: func(reference NativeRecordReference) (NativeOccupancyRecord, bool) {
			record, exists := records[reference]
			return record, exists
		},
		SetLinks: func(reference, next, previous NativeRecordReference) {
			record := records[reference]
			record.Next, record.Previous = next, previous
			records[reference] = record
		},
		SetPosition: func(reference NativeRecordReference, x, y uint16) {
			record := records[reference]
			record.X, record.Y = x, y
			records[reference] = record
		},
	}
}

func TestNativeOccupancyAgainstMixed68000Graph(t *testing.T) {
	data, err := os.ReadFile("testdata/native_occupancy.json")
	if err != nil {
		t.Fatal(err)
	}
	type snapshot struct {
		Name  string
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
	var fixture struct {
		Operations []struct {
			Operation struct {
				Name, Kind, Actor string
				X, Y              uint16
			}
			Return uint32
		}
		Snapshots []snapshot
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Operations) != 14 || len(fixture.Snapshots) != 15 {
		t.Fatal("native graph fixture missing operations/snapshots")
	}
	var state NativeOccupancyState
	records := make(map[NativeRecordReference]NativeOccupancyRecord)
	names := make(map[string]NativeRecordReference)
	for _, cell := range fixture.Snapshots[0].Cells {
		state.Cells[cell.X+cell.Y*64] = NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile, Head: cell.Head}
	}
	for _, actor := range fixture.Snapshots[0].Records {
		records[actor.Reference] = NativeOccupancyRecord{Next: actor.Next, Previous: actor.Previous, X: actor.X, Y: actor.Y}
		names[actor.Name] = actor.Reference
	}
	access := occupancyAccess(records)
	for index, reference := range fixture.Operations {
		op := reference.Operation
		t.Run(op.Name, func(t *testing.T) {
			actor := names[op.Actor]
			var err error
			switch op.Kind {
			case "insert":
				err = state.Insert(actor, int(op.X), int(op.Y), access)
			case "remove":
				err = state.Remove(actor, access)
			case "move":
				var moved bool
				moved, err = state.Move(actor, op.X, op.Y, access)
				if moved != (reference.Return != 0) {
					t.Fatal("native move return differs")
				}
			case "position":
				access.SetPosition(actor, op.X, op.Y)
			default:
				t.Fatalf("unknown native fixture operation %q", op.Kind)
			}
			if err != nil {
				t.Fatal(err)
			}
			expected := fixture.Snapshots[index+1]
			if expected.Name != op.Name || len(expected.Cells) != 3 || len(expected.Records) != 4 {
				t.Fatal("incomplete native graph snapshot")
			}
			for _, cell := range expected.Cells {
				if got := state.Cells[cell.X+cell.Y*64]; got != (NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile, Head: cell.Head}) {
					t.Fatalf("cell %d,%d: %+v, native %+v", cell.X, cell.Y, got, cell)
				}
			}
			for _, actor := range expected.Records {
				if got := records[actor.Reference]; got != (NativeOccupancyRecord{Next: actor.Next, Previous: actor.Previous, X: actor.X, Y: actor.Y}) {
					t.Fatalf("record %s: %+v, native %+v", actor.Name, got, actor)
				}
			}
		})
	}
}

func TestNativeRecordPoolBoundariesAndSignedReferences(t *testing.T) {
	for _, test := range []struct {
		pool                        NativeRecordPool
		start, stride, count, first int
	}{
		{NativeWallPool, 0x5f50, 16, 200, 0},
		{NativeSceneryPool, 0x6bd0, 14, 200, 0},
		{NativeFollowerPool, 0x76c0, 52, 400, 1},
		{NativeEffectPool, 0xc800, 32, 250, 0},
	} {
		for index := test.first; index < test.count; index++ {
			address := test.start + index*test.stride
			reference := NativeRecordReference(uint16(address - 0x76c0))
			location, ok := LocateNativeRecord(reference)
			if !ok || location.Pool != test.pool || location.Index != index || location.Stride != test.stride || location.BSSOffset != address {
				t.Fatalf("pool/address %d/%x: %+v %v", test.pool, address, location, ok)
			}
			if _, ok := LocateNativeRecord(reference + 1); ok {
				t.Fatalf("unaligned record %04x accepted", uint16(reference+1))
			}
		}
	}
	for _, reference := range []NativeRecordReference{0, 0x8000, 0x7080} {
		if _, ok := LocateNativeRecord(reference); ok {
			t.Fatalf("reference %04x outside proven pools accepted", uint16(reference))
		}
	}
}

func TestNativeOccupancyByteLayoutAndMalformedChains(t *testing.T) {
	var state NativeOccupancyState
	index := 10 + 10*64
	state.Cells[index] = NativeOccupancyCell{Header: 0xfd, Tile: 95, Head: 0xf510}
	for offset, expected := range []uint8{0xfd, 95, 0xf5, 0x10} {
		if got, ok := state.Byte(index*4 + offset); !ok || got != expected {
			t.Fatal("native byte layout differs")
		}
	}
	for _, offset := range []int{-1, 16384, 20000} {
		if _, ok := state.Byte(offset); ok {
			t.Fatal("out-of-map byte access accepted")
		}
	}
	state.Cells[index].Head = 52
	records := map[NativeRecordReference]NativeOccupancyRecord{
		52:  {Next: 104, X: 0x0a80, Y: 0x0a80},
		104: {Next: 52, Previous: 52, X: 0x0a80, Y: 0x0a80},
	}
	access := occupancyAccess(records)
	before := state
	if err := state.Remove(52, access); err == nil {
		t.Fatal("cycle accepted")
	}
	if state != before {
		t.Fatal("cycle failure mutated cell graph")
	}
	if moved, err := state.Move(52, 0x0b80, 0x0a80, access); err == nil || moved {
		t.Fatal("cyclic movement accepted")
	}
	if records[52].X != 0x0b80 || state != before {
		t.Fatal("failed move did not preserve native coordinate-first ordering")
	}
}

func TestNativeOccupancyNativeCoordinateAlias(t *testing.T) {
	var state NativeOccupancyState
	index := 32 * 64
	state.Cells[index] = NativeOccupancyCell{Header: 0xfd, Tile: 15, Head: 52}
	records := map[NativeRecordReference]NativeOccupancyRecord{52: {X: 0x0080, Y: 0x2080}}
	access := occupancyAccess(records)
	// X's byte scaling wraps after64 even though packed tile coordinates differ.
	// The native helper detaches/reinserts into the same aliased map address.
	if moved, err := state.Move(52, 0x4080, 0x2080, access); err != nil || !moved {
		t.Fatalf("native coordinate alias lost: %v %v", moved, err)
	}
	if state.Cells[index].Head != 52 || state.Cells[index].Header != 5 || records[52].Next != 0 || records[52].Previous != 0 {
		t.Fatal("aliased move corrupted its graph/header")
	}
	if _, err := state.Move(52, 0x0080, 0xff80, access); err == nil || records[52].Y != 0xff80 {
		t.Fatal("unsafe native address was not reported after coordinate write")
	}
}
