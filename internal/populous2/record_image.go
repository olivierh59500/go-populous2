package populous2

import (
	"encoding/binary"
	"fmt"
)

const (
	NativeRecordImageStart = 0x5f50
	NativeRecordImageEnd   = 0xe740
	NativeRecordImageSize  = NativeRecordImageEnd - NativeRecordImageStart // 34800 bytes.
)

// NativeRecordImage retains the contiguous four original record pools,
// including reserved follower slot zero and bytes untouched by reallocation.
// Typed controllers must patch only the fields they actually write. Rebuilding
// entire records would discard native cross-record aliases and stale bytes.
type NativeRecordImage struct {
	Bytes [NativeRecordImageSize]byte
}

// NativeRecordImageSlot identifies a physical record, including reserved slot
// zero. Location.Index is the native pool index, not the Go follower index.
type NativeRecordImageSlot struct {
	Location  NativeRecordLocation
	Reference NativeRecordReference
	Reserved  bool
}

// NativeRecordImagePatch exposes every record overlapped by the assigned byte
// range. A caller can hydrate affected typed controllers before their next
// update, including when an alias changes a different follower's speed.
type NativeRecordImagePatch struct {
	Offset, BSSOffset, Length int
	Changed                   bool
	Slots                     []NativeRecordImageSlot
}

func nativeImageSlotAt(address int) (NativeRecordImageSlot, bool) {
	if address < NativeRecordImageStart || address >= NativeRecordImageEnd {
		return NativeRecordImageSlot{}, false
	}
	var pool NativeRecordPool
	var start, stride int
	switch {
	case address < 0x6bd0:
		pool, start, stride = NativeWallPool, 0x5f50, 16
	case address < 0x76c0:
		pool, start, stride = NativeSceneryPool, 0x6bd0, 14
	case address < 0xc800:
		pool, start, stride = NativeFollowerPool, 0x76c0, 52
	default:
		pool, start, stride = NativeEffectPool, 0xc800, 32
	}
	index := (address - start) / stride
	base := start + index*stride
	return NativeRecordImageSlot{
		Location:  NativeRecordLocation{Pool: pool, Index: index, BSSOffset: base, Stride: stride},
		Reference: NativeRecordReference(uint16(base - 0x76c0)),
		Reserved:  pool == NativeFollowerPool && index == 0,
	}, true
}

// LocateNativeRecordImageSlot requires an aligned entity address. Raw reads
// below intentionally do not use this lookup: $035c is not a follower record
// reference, but is a valid even memory alias within the follower pool.
func LocateNativeRecordImageSlot(reference NativeRecordReference) (NativeRecordImageSlot, bool) {
	address := 0x76c0 + int(int16(reference))
	slot, ok := nativeImageSlotAt(address)
	return slot, ok && slot.Location.BSSOffset == address
}

func (image *NativeRecordImage) span(reference NativeRecordReference, offset, length int) (int, error) {
	if image == nil || length < 0 || length > NativeRecordImageSize {
		return 0, fmt.Errorf("invalid native record image or byte range")
	}
	base := 0x76c0 + int(int16(reference)) - NativeRecordImageStart
	// Compare before adding offset, so even arbitrary Go int offsets cannot
	// overflow into an apparently valid native address.
	if offset < -base || offset > NativeRecordImageSize-length-base {
		return 0, fmt.Errorf("native record image access outside pools: ref%04x offset%d length%d", uint16(reference), offset, length)
	}
	return base + offset, nil
}

// Read8/16/32 interpret reference as a signed word relative to BSS $76c0.
// Offsets are byte counts and may cross records or pools. Raw reads permit
// odd byte addresses as a bounded byte-image operation; they do not model the
// 68000 processor's separate address-error exception on an odd word access.
func (image *NativeRecordImage) Read8(reference NativeRecordReference, offset int) (uint8, error) {
	at, err := image.span(reference, offset, 1)
	if err != nil {
		return 0, err
	}
	return image.Bytes[at], nil
}

func (image *NativeRecordImage) Read16(reference NativeRecordReference, offset int) (uint16, error) {
	at, err := image.span(reference, offset, 2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(image.Bytes[at : at+2]), nil
}

func (image *NativeRecordImage) Read32(reference NativeRecordReference, offset int) (uint32, error) {
	at, err := image.span(reference, offset, 4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(image.Bytes[at : at+4]), nil
}

// Patch writes only the assigned bytes, without allocation-state checks or
// clearing the rest of an actor. Slots describes physical overlap independently
// of activity, and includes both sides of a record or pool boundary.
func (image *NativeRecordImage) Patch(reference NativeRecordReference, offset int, bytes []byte) (NativeRecordImagePatch, error) {
	at, err := image.span(reference, offset, len(bytes))
	if err != nil {
		return NativeRecordImagePatch{}, err
	}
	patch := NativeRecordImagePatch{Offset: at, BSSOffset: NativeRecordImageStart + at, Length: len(bytes)}
	for index, value := range bytes {
		patch.Changed = patch.Changed || image.Bytes[at+index] != value
	}
	// copy retains ordinary memmove semantics even when bytes is a slice of
	// this same image and overlaps the destination.
	copy(image.Bytes[at:at+len(bytes)], bytes)
	for address, end := patch.BSSOffset, patch.BSSOffset+patch.Length; address < end; {
		slot, _ := nativeImageSlotAt(address)
		patch.Slots = append(patch.Slots, slot)
		address = slot.Location.BSSOffset + slot.Location.Stride
	}
	return patch, nil
}

func (image *NativeRecordImage) Write8(reference NativeRecordReference, offset int, value uint8) (NativeRecordImagePatch, error) {
	return image.Patch(reference, offset, []byte{value})
}

func (image *NativeRecordImage) Write16(reference NativeRecordReference, offset int, value uint16) (NativeRecordImagePatch, error) {
	var bytes [2]byte
	binary.BigEndian.PutUint16(bytes[:], value)
	return image.Patch(reference, offset, bytes[:])
}

func (image *NativeRecordImage) Write32(reference NativeRecordReference, offset int, value uint32) (NativeRecordImagePatch, error) {
	var bytes [4]byte
	binary.BigEndian.PutUint32(bytes[:], value)
	return image.Patch(reference, offset, bytes[:])
}
