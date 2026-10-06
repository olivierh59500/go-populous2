package populous2

import (
	"encoding/binary"
	"errors"
	"testing"
)

func TestNativeWallNeighborCodePortRetainsAbsolutePointersAndFailure(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	m := h.Memory.BSS
	for pos := 0; pos < 4096; pos++ {
		_ = m.Write32(0xf44+pos*4, 0x010f0000)
	}
	_ = m.Write16(0xe9de+16, 0)
	rules, err := DecodeNativeWallRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	state := NativeWallPlacementState{}
	first, err := rules.Create(2, 33, 32, &state, h.World.nativeWallCallbacks(0))
	if err != nil || !first.Created {
		t.Fatal(first, err)
	}
	second, err := rules.Create(2, 33, 33, &state, h.World.nativeWallCallbacks(0))
	if err != nil || !second.Created {
		t.Fatal(second, err)
	}
	at := cleanupRecordAddress(first.Reference)
	got, err := h.Memory.Code.Read32(0x1647c)
	if err != nil || got != h.Memory.BSSBase+uint32(at) {
		t.Fatal("native neighbor A0 did not retain physical BSS pointer", got, err)
	}
	// Pointer stores precede allocation's remaining record initialization.
	// A failing port preserves the genuine prior stores without a completedcast.
	source := h.World.nativeWallCallbacks(0)
	wanted := errors.New("explicit native CODE pointer port failure")
	writes := 0
	lastSlot := 0
	lastRef := NativeRecordReference(0)
	original := source.WriteNeighbor
	source.WriteNeighbor = func(slot int, ref NativeRecordReference) error {
		if writes == 1 {
			return wanted
		}
		writes++
		lastSlot, lastRef = slot, ref
		return original(slot, ref)
	}
	third, err := rules.Create(2, 34, 33, &state, source)
	if !errors.Is(err, wanted) || third.Created || writes != 1 {
		t.Fatal("native pointer port failure was acknowledged", third, writes, err)
	}
	raw := h.Code.RawData()
	expected := uint32(0)
	if lastRef != 0 {
		expected = h.Memory.BSSBase + uint32(cleanupRecordAddress(lastRef))
	}
	if binary.BigEndian.Uint32(raw[0x1647c+lastSlot*4:]) != expected {
		t.Fatal("native completed pointer prefix was rolled back")
	}
}
