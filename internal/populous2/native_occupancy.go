package populous2

import "fmt"

type NativeOccupancyCell struct {
	Header, Tile uint8
	Head         NativeRecordReference
}

// NativeOccupancyState preserves the native four-byte map cells. Header's
// upper bits are movement pressure, not a recomputed live-occupant count.
type NativeOccupancyState struct {
	Cells [4096]NativeOccupancyCell
}

type NativeOccupancyRecord struct {
	Next, Previous NativeRecordReference
	X, Y           uint16 // Full 8.8 coordinates, native record words 6 and 8.
}

// NativeRecordAccess adapts each differently sized actor pool. Metadata lookup
// below is independent of activity; the store decides which references exist.
type NativeRecordAccess struct {
	Record      func(NativeRecordReference) (NativeOccupancyRecord, bool)
	SetLinks    func(NativeRecordReference, NativeRecordReference, NativeRecordReference)
	SetPosition func(NativeRecordReference, uint16, uint16)
}

type NativeRecordPool uint8

const (
	NativeWallPool NativeRecordPool = iota + 1
	NativeSceneryPool
	NativeFollowerPool
	NativeEffectPool
)

type NativeRecordLocation struct {
	Pool                     NativeRecordPool
	Index, BSSOffset, Stride int
}

// LocateNativeRecord interprets the reference as a signed word relative to
// BSS $76c0. It validates alignment in the four proven pools; follower slot
// zero is the empty-reference sentinel. It does not consult an active flag.
func LocateNativeRecord(reference NativeRecordReference) (NativeRecordLocation, bool) {
	if reference == 0 {
		return NativeRecordLocation{}, false
	}
	address := 0x76c0 + int(int16(reference))
	for _, pool := range []struct {
		kind       NativeRecordPool
		start, end int
		stride     int
	}{
		{NativeWallPool, 0x5f50, 0x6bd0, 16},
		{NativeSceneryPool, 0x6bd0, 0x76c0, 14},
		{NativeFollowerPool, 0x76c0, 0xc800, 52},
		{NativeEffectPool, 0xc800, 0xe740, 32},
	} {
		if address >= pool.start && address < pool.end && (address-pool.start)%pool.stride == 0 {
			return NativeRecordLocation{Pool: pool.kind, Index: (address - pool.start) / pool.stride, BSSOffset: address, Stride: pool.stride}, true
		}
	}
	return NativeRecordLocation{}, false
}

// Byte exposes the original map's byte layout for native offset-based reads,
// including the exact big-endian head word. Out-of-map offsets are explicit.
func (state *NativeOccupancyState) Byte(offset int) (uint8, bool) {
	if state == nil || offset < 0 || offset >= len(state.Cells)*4 {
		return 0, false
	}
	cell := state.Cells[offset/4]
	switch offset & 3 {
	case 0:
		return cell.Header, true
	case 1:
		return cell.Tile, true
	case 2:
		return uint8(cell.Head >> 8), true
	default:
		return uint8(cell.Head), true
	}
}

func (access NativeRecordAccess) record(reference NativeRecordReference) (NativeOccupancyRecord, error) {
	if reference == 0 || access.Record == nil || access.SetLinks == nil {
		return NativeOccupancyRecord{}, fmt.Errorf("missing native occupancy reference/store")
	}
	record, ok := access.Record(reference)
	if !ok {
		return NativeOccupancyRecord{}, fmt.Errorf("unknown native record reference %04x", uint16(reference))
	}
	return record, nil
}

func (state *NativeOccupancyState) chain(index int, access NativeRecordAccess) ([]NativeRecordReference, error) {
	seen := make(map[NativeRecordReference]bool)
	chain := []NativeRecordReference{}
	for current := state.Cells[index].Head; current != 0; {
		if seen[current] {
			return nil, fmt.Errorf("cyclic native occupancy chain at reference %04x", uint16(current))
		}
		seen[current] = true
		record, err := access.record(current)
		if err != nil {
			return nil, err
		}
		chain = append(chain, current)
		current = record.Next
	}
	return chain, nil
}

// Insert follows $125a0: clear the selected links, prepend it, and set the old
// head's previous word. Coordinates are not written and Header is unchanged.
// The caller must already position this newly allocated, unlinked actor.
func (state *NativeOccupancyState) Insert(reference NativeRecordReference, x, y int, access NativeRecordAccess) error {
	if state == nil || !inside(x, y) {
		return fmt.Errorf("native occupancy insertion outside map")
	}
	if _, err := access.record(reference); err != nil {
		return err
	}
	index := x + y*64
	chain, err := state.chain(index, access)
	if err != nil {
		return err
	}
	for _, current := range chain {
		if current == reference {
			return fmt.Errorf("native actor is already linked at target cell")
		}
	}
	head := state.Cells[index].Head
	access.SetLinks(reference, head, 0)
	if head != 0 {
		record, _ := access.Record(head)
		access.SetLinks(head, record.Next, reference)
	}
	state.Cells[index].Head = reference
	return nil
}

// Remove follows $125da: search the tile chain instead of trusting the selected
// record's previous word, then clear both selected links even when absent.
// It neither deactivates the actor nor decrements movement pressure.
func (state *NativeOccupancyState) Remove(reference NativeRecordReference, access NativeRecordAccess) error {
	if state == nil {
		return fmt.Errorf("missing native occupancy map")
	}
	record, err := access.record(reference)
	if err != nil {
		return err
	}
	index, err := nativeOccupancyIndex(record.X, record.Y)
	if err != nil {
		return err
	}
	chain, err := state.chain(index, access)
	if err != nil {
		return err
	}
	for position, current := range chain {
		if current != reference {
			continue
		}
		if position == 0 {
			state.Cells[index].Head = record.Next
			if record.Next != 0 {
				next, _ := access.Record(record.Next)
				access.SetLinks(record.Next, next.Next, 0)
			}
		} else {
			previous := chain[position-1]
			before, _ := access.Record(previous)
			access.SetLinks(previous, record.Next, before.Previous)
			if record.Next != 0 {
				next, _ := access.Record(record.Next)
				access.SetLinks(record.Next, next.Next, record.Previous)
			}
		}
		break
	}
	access.SetLinks(reference, 0, 0)
	return nil
}

// Move follows $12518. It writes the full coordinate words first, including
// when the tile is unchanged. Changed tiles splice by the native previous
// link and prepend to the destination; only the new Header gains byte +8.
//
// For invalid references, cycles or out-of-map native addresses it reports an
// error. A coordinate write may already have occurred, as in the native order;
// callers must not assume failure restores that write or normalize coordinates.
func (state *NativeOccupancyState) Move(reference NativeRecordReference, x, y uint16, access NativeRecordAccess) (bool, error) {
	if state == nil || access.SetPosition == nil {
		return false, fmt.Errorf("missing native occupancy position store")
	}
	record, err := access.record(reference)
	if err != nil {
		return false, err
	}
	access.SetPosition(reference, x, y)
	if nativeOccupancyPackedTile(record.X, record.Y) == nativeOccupancyPackedTile(x, y) {
		return false, nil
	}
	oldIndex, err := nativeOccupancyIndex(record.X, record.Y)
	if err != nil {
		return false, err
	}
	newIndex, err := nativeOccupancyIndex(x, y)
	if err != nil {
		return false, err
	}
	oldChain, err := state.chain(oldIndex, access)
	if err != nil {
		return false, err
	}
	found := false
	for _, current := range oldChain {
		found = found || current == reference
	}
	if !found {
		return false, fmt.Errorf("native moving actor is not linked at old cell")
	}
	if _, err := state.chain(newIndex, access); err != nil {
		return false, err
	}
	if record.Previous != 0 {
		previous, err := access.record(record.Previous)
		if err != nil {
			return false, err
		}
		access.SetLinks(record.Previous, record.Next, previous.Previous)
		if record.Next != 0 {
			next, _ := access.Record(record.Next)
			access.SetLinks(record.Next, next.Next, record.Previous)
		}
	} else {
		state.Cells[oldIndex].Head = record.Next
		if record.Next != 0 {
			next, _ := access.Record(record.Next)
			access.SetLinks(record.Next, next.Next, 0)
		}
	}
	head := state.Cells[newIndex].Head
	access.SetLinks(reference, head, 0)
	if head != 0 {
		next, _ := access.Record(head)
		access.SetLinks(head, next.Next, reference)
	}
	state.Cells[newIndex].Head = reference
	state.Cells[newIndex].Header += 8 // Native ADDI.B: wrap, preserve low three bits.
	return true, nil
}

func nativeOccupancyPackedTile(x, y uint16) uint16 {
	return y&0xff00 | x>>8
}

// Byte ADDs scale only X before signed-word indexing; retain valid native
// linear aliases rather than treating the two high bytes as bounded Go X/Y.
func nativeOccupancyIndex(x, y uint16) (int, error) {
	packed := nativeOccupancyPackedTile(x, y)
	offset := int(int16(packed&0xff00 | uint16(uint8(packed)<<2)))
	index := offset / 4
	if index < 0 || index >= 4096 {
		return 0, fmt.Errorf("native occupancy address outside map: %04x", uint16(offset))
	}
	return index, nil
}
