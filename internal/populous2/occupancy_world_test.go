package populous2

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeWorldOccupancyAgainstMixed68000Graph(t *testing.T) {
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
		t.Fatal("native mixed graph catalog is incomplete")
	}
	var state NativeWorldOccupancy
	names := make(map[string]NativeRecordReference)
	if len(fixture.Snapshots[0].Cells) != 3 || len(fixture.Snapshots[0].Records) != 4 {
		t.Fatal("native initial graph is incomplete")
	}
	for _, cell := range fixture.Snapshots[0].Cells {
		if !inside(cell.X, cell.Y) || cell.Head != 0 {
			t.Fatal("unexpected native initial cell")
		}
		state.Grid.Cells[cell.X+cell.Y*64] = NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile, Head: cell.Head}
	}
	for _, actor := range fixture.Snapshots[0].Records {
		entry, ok := state.entry(actor.Reference)
		if !ok {
			t.Fatal("native fixture reference is outside the four pools")
		}
		entry.Record = NativeOccupancyRecord{Next: actor.Next, Previous: actor.Previous, X: actor.X, Y: actor.Y}
		names[actor.Name] = actor.Reference
	}
	for index, reference := range fixture.Operations {
		op := reference.Operation
		t.Run(op.Name, func(t *testing.T) {
			actor := names[op.Actor]
			var err error
			switch op.Kind {
			case "insert":
				record, ok := state.Record(actor)
				if !ok || record.X>>8 != op.X || record.Y>>8 != op.Y {
					t.Fatal("native placement position differs")
				}
				err = state.Place(actor, record.X, record.Y)
			case "remove":
				err = state.Remove(actor)
			case "move":
				var changed bool
				changed, err = state.Move(actor, op.X, op.Y)
				if changed != (reference.Return != 0) {
					t.Fatal("native move return differs")
				}
			case "position":
				state.Access().SetPosition(actor, op.X, op.Y)
			default:
				t.Fatalf("unknown native operation %q", op.Kind)
			}
			if err != nil {
				t.Fatal(err)
			}
			expected := fixture.Snapshots[index+1]
			if expected.Name != op.Name || len(expected.Cells) != 3 || len(expected.Records) != 4 {
				t.Fatal("native graph snapshot is incomplete")
			}
			for _, cell := range expected.Cells {
				if !inside(cell.X, cell.Y) {
					t.Fatal("native fixture cell outside map")
				}
				want := NativeOccupancyCell{Header: cell.Header, Tile: cell.Tile, Head: cell.Head}
				if got := state.Grid.Cells[cell.X+cell.Y*64]; got != want {
					t.Fatalf("cell %d,%d: got %+v, native %+v", cell.X, cell.Y, got, want)
				}
			}
			for _, actor := range expected.Records {
				want := NativeOccupancyRecord{Next: actor.Next, Previous: actor.Previous, X: actor.X, Y: actor.Y}
				if got, ok := state.Record(actor.Reference); !ok || got != want {
					t.Fatalf("record %s: got %+v, native %+v", actor.Name, got, want)
				}
			}
			if err := state.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeWorldOccupancyAllPoolReferencesAndSerialization(t *testing.T) {
	var state NativeWorldOccupancy
	count := 0
	for _, pool := range []struct {
		kind    NativeRecordPool
		count   int
		initial int
	}{
		{NativeWallPool, 200, -6000},
		{NativeSceneryPool, 200, -2800},
		{NativeFollowerPool, 399, 52},
		{NativeEffectPool, 250, 20800},
	} {
		for index := range pool.count {
			reference, ok := NativeWorldReference(pool.kind, index)
			location, found := LocateNativeRecord(reference)
			nativeIndex := index
			if pool.kind == NativeFollowerPool {
				nativeIndex++
			}
			if !ok || !found || location.Pool != pool.kind || location.Index != nativeIndex {
				t.Fatalf("native pool/index translation differs: %d/%d %+v", pool.kind, index, location)
			}
			if index == 0 && int(int16(reference)) != pool.initial {
				t.Fatal("native signed pool start differs")
			}
			x, y := uint16((count%64)*256+128), uint16((count/64)*256+128)
			if err := state.Place(reference, x, y); err != nil {
				t.Fatal(err)
			}
			count++
		}
		for _, index := range []int{-1, pool.count} {
			if _, ok := NativeWorldReference(pool.kind, index); ok {
				t.Fatal("out-of-pool index accepted")
			}
		}
	}
	if count != 1049 {
		t.Fatal("a native pool record was omitted")
	}
	if _, ok := NativeWorldReference(0, 0); ok {
		t.Fatal("unknown native pool accepted")
	}
	if _, ok := state.Record(0); ok {
		t.Fatal("reserved follower slot zero accepted")
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored NativeWorldOccupancy
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil || restored != state {
		t.Fatalf("native graph serialization lost state: %v", err)
	}
}

func TestNativeWorldOccupancyPreservesRawPressureAndDeathMembership(t *testing.T) {
	var state NativeWorldOccupancy
	follower, _ := NativeWorldReference(NativeFollowerPool, 0)
	tree, _ := NativeWorldReference(NativeSceneryPool, 0)
	index := 10 + 12*64
	state.Grid.Cells[index] = NativeOccupancyCell{Header: 0xed, Tile: 15}
	for _, reference := range []NativeRecordReference{tree, follower} {
		if err := state.Place(reference, 0x0a80, 0x0c80); err != nil {
			t.Fatal(err)
		}
	}
	head := state.Grid.Cells[index].Head
	if err := state.SetTile(10, 12, 95); err != nil {
		t.Fatal(err)
	}
	if got := state.Grid.Cells[index]; got.Header != 0xed || got.Tile != 95 || got.Head != head {
		t.Fatal("overlay changed raw height/pressure/membership")
	}
	if err := state.SetTerrain(10, 12, 3, 6, false); err != nil {
		t.Fatal(err)
	}
	if got := state.Grid.Cells[index]; got.Header != 0xeb || got.Head != head {
		t.Fatal("height synchronization discarded pressure or head")
	}
	if err := state.SetTerrain(10, 12, 4, 15, true); err != nil {
		t.Fatal(err)
	}
	if got := state.Grid.Cells[index]; got.Header != 4 || got.Head != head {
		t.Fatal("native sculpting failed to clear pressure or preserve occupants")
	}
	// An owner's death state is supplied separately. Reading owner0 must not
	// remove its reservation or discard the tree underneath that follower.
	if word, ok := state.FollowerPrefixWord(follower, 12, NativeFollowerPrefix{}); !ok || word != 0 {
		t.Fatal("inactive death prefix was not readable")
	}
	if linked, ok := state.Linked(follower); !ok || !linked {
		t.Fatal("owner/activity implicitly unlinked a death record")
	}
	if err := state.Remove(follower); err != nil {
		t.Fatal(err)
	}
	if state.Grid.Cells[index].Head != tree || state.Grid.Cells[index].Header != 4 {
		t.Fatal("death release lost mixed tail or altered pressure")
	}
	if err := state.Place(follower, 0x0a80, 0x0c80); err != nil {
		t.Fatal("released follower slot could not be reused:", err)
	}
	if err := state.Place(follower, 0x0b80, 0x0c80); err == nil {
		t.Fatal("mapped record was allocated twice")
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWorldOccupancyRawWordAndNativeFollowerPrefixes(t *testing.T) {
	var state NativeWorldOccupancy
	state.Grid.Cells[0] = NativeOccupancyCell{Header: 0xfd, Tile: 95, Head: 0xf510}
	state.Grid.Cells[1] = NativeOccupancyCell{Header: 3, Tile: 145, Head: 52}
	for offset, want := range []uint16{0xfd5f, 0xf510, 0x0391, 52} {
		if got, ok := state.Word(offset * 2); !ok || got != want {
			t.Fatal("raw map word does not preserve header/tile/signed head")
		}
	}
	for _, offset := range []int{-2, -1, 1, 16383, 16384} {
		if _, ok := state.Word(offset); ok {
			t.Fatal("unaligned or out-of-map native word accepted")
		}
	}
	if _, ok := state.Byte(16384); ok {
		t.Fatal("out-of-map native byte accepted")
	}
	// These are full raw follower records from independently executed native
	// Whirlwind lift/release goldens, including inactive cleanup and hero flags.
	cases := nativeInteractionFixtures(t)
	checked := 0
	for _, fixture := range cases {
		for _, group := range fixture.Groups {
			raw, err := hex.DecodeString(group.Raw)
			if err != nil || len(raw) != 52 || group.Slot < 1 || group.Slot > 399 {
				t.Fatal("invalid native follower prefix fixture")
			}
			reference, _ := NativeWorldReference(NativeFollowerPool, group.Slot-1)
			entry, _ := state.entry(reference)
			entry.Record = NativeOccupancyRecord{Next: NativeRecordReference(binary.BigEndian.Uint16(raw[2:])), Previous: NativeRecordReference(binary.BigEndian.Uint16(raw[4:])), X: binary.BigEndian.Uint16(raw[6:]), Y: binary.BigEndian.Uint16(raw[8:])}
			prefix := NativeFollowerPrefix{Kind: raw[0], Byte1: raw[1], Animation: binary.BigEndian.Uint16(raw[10:]), Owner: raw[12], Flags: raw[13], Word14: binary.BigEndian.Uint16(raw[14:])}
			for offset := uint16(0); offset <= 14; offset += 2 {
				want := binary.BigEndian.Uint16(raw[offset:])
				if got, ok := state.FollowerPrefixWord(reference, offset, prefix); !ok || got != want {
					t.Fatalf("native follower slot%d word%d: %04x, native %04x", group.Slot, offset, got, want)
				}
			}
			checked++
		}
	}
	if len(cases) != 92 || checked < 92 {
		t.Fatal("native prefix fixture catalog is empty or incomplete")
	}
	for _, test := range []struct {
		ref    NativeRecordReference
		offset uint16
	}{{0, 0}, {52, 1}, {52, 16}, {20800, 0}, {0xf510, 0}} {
		if _, ok := state.FollowerPrefixWord(test.ref, test.offset, NativeFollowerPrefix{}); ok {
			t.Fatal("invalid follower-prefix address accepted")
		}
	}
}

func TestNativeWorldOccupancyValidatesSerializedMixedGraphs(t *testing.T) {
	var valid NativeWorldOccupancy
	follower, _ := NativeWorldReference(NativeFollowerPool, 0)
	tree, _ := NativeWorldReference(NativeSceneryPool, 0)
	if err := valid.Place(tree, 0x0a80, 0x0c80); err != nil {
		t.Fatal(err)
	}
	if err := valid.Place(follower, 0x0a80, 0x0c80); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*NativeWorldOccupancy)
	}{
		{"unknown head", func(s *NativeWorldOccupancy) { s.Grid.Cells[778].Head = 1 }},
		{"head predecessor", func(s *NativeWorldOccupancy) { s.Followers[0].Record.Previous = tree }},
		{"tail predecessor", func(s *NativeWorldOccupancy) { s.Scenery[0].Record.Previous = 0 }},
		{"cycle", func(s *NativeWorldOccupancy) { s.Scenery[0].Record.Next = follower }},
		{"duplicate head", func(s *NativeWorldOccupancy) { s.Grid.Cells[779].Head = follower }},
		{"wrong cell", func(s *NativeWorldOccupancy) { s.Followers[0].Record.X = 0x0b80 }},
		{"unsafe coordinates", func(s *NativeWorldOccupancy) { s.Followers[0].Record.Y = 0xff80 }},
		{"unknown next", func(s *NativeWorldOccupancy) { s.Followers[0].Record.Next = 1 }},
		{"missing membership", func(s *NativeWorldOccupancy) { s.Followers[0].Linked = false }},
		{"orphan member", func(s *NativeWorldOccupancy) { s.Effects[249].Linked = true }},
		{"unmapped links", func(s *NativeWorldOccupancy) { s.Walls[199].Record.Next = tree }},
	} {
		t.Run(test.name, func(t *testing.T) {
			malformed := valid
			test.mutate(&malformed)
			before := malformed
			if err := malformed.Validate(); err == nil {
				t.Fatal("malformed native graph accepted")
			}
			if malformed != before {
				t.Fatal("validation repaired or normalized serialized native state")
			}
		})
	}
	// Controller/bookkeeping coordinates outside the map are legitimate when
	// unmapped. They must not acquire a false graph reservation during loading.
	valid.Effects[249].Record.X, valid.Effects[249].Record.Y = 0xffff, 0xffff
	if err := valid.Validate(); err != nil {
		t.Fatal("unmapped controller coordinates were normalized or rejected:", err)
	}
}

func TestNativeWorldOccupancyNativeAliasAndExplicitUnsafeMoves(t *testing.T) {
	var state NativeWorldOccupancy
	follower, _ := NativeWorldReference(NativeFollowerPool, 0)
	state.Grid.Cells[32*64].Header = 0xfd
	if err := state.Place(follower, 0x0080, 0x2080); err != nil {
		t.Fatal(err)
	}
	if changed, err := state.Move(follower, 0x4080, 0x2080); err != nil || !changed {
		t.Fatalf("native byte-scaled coordinate alias lost: %v %v", changed, err)
	}
	if state.Grid.Cells[32*64].Head != follower || state.Grid.Cells[32*64].Header != 5 {
		t.Fatal("native aliased move changed head or failed byte pressure wrap")
	}
	if err := state.Validate(); err != nil {
		t.Fatal("native address alias rejected by save validation:", err)
	}
	if changed, err := state.Move(follower, 0x0080, 0xff80); err == nil || changed || state.Followers[0].Record.Y != 0xff80 {
		t.Fatal("unsafe native move did not report coordinate-first failure")
	}
	var empty NativeWorldOccupancy
	if changed, err := empty.Move(20800, 0x8080, 0x8080); err == nil || changed {
		t.Fatal("unmapped effect controller was treated as a mapped actor")
	}
	if err := empty.SetTerrain(0, 0, 8, 15, false); err == nil {
		t.Fatal("unrepresentable native terrain height accepted")
	}
	if err := empty.SetTile(64, 0, 15); err == nil {
		t.Fatal("outside native tile accepted")
	}
	var missing *NativeWorldOccupancy
	if err := missing.Validate(); err == nil {
		t.Fatal("missing serialized occupancy accepted")
	}
	if _, ok := missing.Record(52); ok {
		t.Fatal("missing occupancy returned a record")
	}
}
