package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeMagnetRules struct {
	Animations [NativeMagnetCount]uint16
}

func DecodeNativeMagnetRules(exe *amiga.Executable) (NativeMagnetRules, error) {
	var rules NativeMagnetRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x140ae {
		return rules, fmt.Errorf("native magnet initialization table missing")
	}
	for owner := range rules.Animations {
		rules.Animations[owner] = binary.BigEndian.Uint16(exe.Hunks[0].Data[0x140a8+owner*2:])
	}
	return rules, nil
}

// Create translates $14048: store the deity's marker reference and assign
// only the marker's center coordinates, kind, owner and animation. It leaves
// retained bytes intact. Initial map insertion does not add movement pressure.
func (rules NativeMagnetRules) Create(owner uint8, x, y uint8, memory NativeRuntimeMemory, grid *NativeOccupancyState) error {
	ref, ok := NativeMagnetReference(owner)
	if !ok || grid == nil || x >= 64 || y >= 64 {
		return fmt.Errorf("native magnet initialization outside supported owner/map")
	}
	deity, _ := NativeDeityAddress(owner)
	if _, err := memory.Write16(deity+10, uint16(ref)); err != nil {
		return err
	}
	address := 0x76c0 + int(int16(ref))
	for _, field := range []struct {
		offset int
		value  uint16
	}{{6, uint16(x)*256 + 128}, {8, uint16(y)*256 + 128}, {10, rules.Animations[owner]}} {
		if _, err := memory.Write16(address+field.offset, field.value); err != nil {
			return err
		}
	}
	if _, err := memory.Write8(address, 0x14); err != nil {
		return err
	}
	if _, err := memory.Write8(address+12, owner); err != nil {
		return err
	}
	return grid.Insert(ref, int(x), int(y), memory.RecordAccess())
}

// Relocate translates the marker operation at $13fe4. Its signed byte clamps
// and remove/insert order differ from normal actor movement: pressure remains
// unchanged, and the marker is prepended even when staying in the same cell.
func (rules NativeMagnetRules) Relocate(owner uint8, x, y uint8, memory NativeRuntimeMemory, grid *NativeOccupancyState) error {
	deity, ok := NativeDeityAddress(owner)
	if !ok || grid == nil {
		return fmt.Errorf("native magnet relocation owner/map missing")
	}
	ref, err := memory.Read16(deity + 10)
	if err != nil {
		return err
	}
	access := memory.RecordAccess()
	if err := grid.Remove(NativeRecordReference(ref), access); err != nil {
		return err
	}
	clamp := func(value uint8) uint8 {
		if int8(value) < 0 {
			return 0
		}
		return min(value, 63)
	}
	x, y = clamp(x), clamp(y)
	access.SetPosition(NativeRecordReference(ref), uint16(x)*256+128, uint16(y)*256+128)
	return grid.Insert(NativeRecordReference(ref), int(x), int(y), access)
}
