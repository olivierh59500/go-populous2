package populous2

import (
	"encoding/binary"
	"fmt"
)

const (
	NativeMagnetImageStart = NativeRecordImageEnd
	NativeMagnetStride     = 14
	NativeMagnetCount      = 3
	NativeDeityImageStart  = NativeMagnetImageStart + NativeMagnetStride*NativeMagnetCount
	NativeDeityStride      = 314
	NativeDeityCount       = 3
	NativeRuntimeImageEnd  = NativeDeityImageStart + NativeDeityStride*NativeDeityCount
)

// NativeGlobalImage retains the original three magnets and three deity
// records, including the reserved owner-zero records and unidentified bytes.
// Controllers assign individual fields; typed views must not regenerate it.
type NativeGlobalImage struct {
	Bytes [NativeRuntimeImageEnd - NativeMagnetImageStart]byte
}

// NativeRuntimeMemory borrows the actor and global images as one bounded BSS
// window. Addresses are original BSS byte offsets, not native host pointers.
// Writes can cross the actor/global seam, preserving their actual byte ranges.
type NativeRuntimeMemory struct {
	Records *NativeRecordImage
	Globals *NativeGlobalImage
}

type NativeRuntimePatch struct {
	RecordPatches []NativeRecordImagePatch
	Globals       bool
}

func NativeDeityAddress(owner uint8) (int, bool) {
	if int(owner) >= NativeDeityCount {
		return 0, false
	}
	return NativeDeityImageStart + int(owner)*NativeDeityStride, true
}

func NativeMagnetReference(owner uint8) (NativeRecordReference, bool) {
	if int(owner) >= NativeMagnetCount {
		return 0, false
	}
	return NativeRecordReference(NativeMagnetImageStart + int(owner)*NativeMagnetStride - 0x76c0), true
}

func LocateNativeMagnet(reference NativeRecordReference) (NativeRecordLocation, bool) {
	address := 0x76c0 + int(int16(reference))
	if address < NativeMagnetImageStart || address >= NativeDeityImageStart || (address-NativeMagnetImageStart)%NativeMagnetStride != 0 {
		return NativeRecordLocation{}, false
	}
	return NativeRecordLocation{Pool: NativeMagnetPool, Index: (address - NativeMagnetImageStart) / NativeMagnetStride, BSSOffset: address, Stride: NativeMagnetStride}, true
}

func (m NativeRuntimeMemory) validateSpan(address, length int) error {
	if m.Records == nil || m.Globals == nil || length < 0 || length > NativeRuntimeImageEnd-NativeRecordImageStart || address < NativeRecordImageStart || address > NativeRuntimeImageEnd-length {
		return fmt.Errorf("native runtime image access outside retained BSS: address %x length %d", address, length)
	}
	return nil
}

func (m NativeRuntimeMemory) Read(address int, target []byte) error {
	if err := m.validateSpan(address, len(target)); err != nil {
		return err
	}
	for len(target) > 0 {
		var source []byte
		if address < NativeMagnetImageStart {
			source = m.Records.Bytes[address-NativeRecordImageStart:]
		} else {
			source = m.Globals.Bytes[address-NativeMagnetImageStart:]
		}
		count := min(len(target), len(source))
		copy(target[:count], source[:count])
		target, address = target[count:], address+count
	}
	return nil
}

func (m NativeRuntimeMemory) Read8(address int) (uint8, error) {
	var data [1]byte
	err := m.Read(address, data[:])
	return data[0], err
}

func (m NativeRuntimeMemory) Read16(address int) (uint16, error) {
	var data [2]byte
	err := m.Read(address, data[:])
	return binary.BigEndian.Uint16(data[:]), err
}

func (m NativeRuntimeMemory) Read32(address int) (uint32, error) {
	var data [4]byte
	err := m.Read(address, data[:])
	return binary.BigEndian.Uint32(data[:]), err
}

func (m NativeRuntimeMemory) Patch(address int, data []byte) (NativeRuntimePatch, error) {
	var result NativeRuntimePatch
	if err := m.validateSpan(address, len(data)); err != nil {
		return result, err
	}
	// A caller may pass a slice borrowed from either retained segment. Copy
	// once so crossing the seam has ordinary memmove semantics as well.
	data = append([]byte(nil), data...)
	if address < NativeMagnetImageStart && len(data) > 0 {
		count := min(len(data), NativeMagnetImageStart-address)
		patch, err := m.Records.Patch(0, address-0x76c0, data[:count])
		if err != nil {
			return result, err
		}
		result.RecordPatches = append(result.RecordPatches, patch)
		address, data = address+count, data[count:]
	}
	if len(data) > 0 {
		copy(m.Globals.Bytes[address-NativeMagnetImageStart:], data)
		result.Globals = true
	}
	return result, nil
}

func (m NativeRuntimeMemory) Write8(address int, value uint8) (NativeRuntimePatch, error) {
	return m.Patch(address, []byte{value})
}

func (m NativeRuntimeMemory) Write16(address int, value uint16) (NativeRuntimePatch, error) {
	var data [2]byte
	binary.BigEndian.PutUint16(data[:], value)
	return m.Patch(address, data[:])
}

func (m NativeRuntimeMemory) Write32(address int, value uint32) (NativeRuntimePatch, error) {
	var data [4]byte
	binary.BigEndian.PutUint32(data[:], value)
	return m.Patch(address, data[:])
}

// RecordAccess adapts the original common graph prefix directly from retained
// bytes. It includes marker references and bounded aliases; it deliberately
// does not restrict primitive operations to aligned, active actor entities.
func (m NativeRuntimeMemory) RecordAccess() NativeRecordAccess {
	return NativeRecordAccess{
		Record: func(ref NativeRecordReference) (NativeOccupancyRecord, bool) {
			address := 0x76c0 + int(int16(ref))
			var bytes [8]byte
			if ref == 0 || m.Read(address+2, bytes[:]) != nil {
				return NativeOccupancyRecord{}, false
			}
			return NativeOccupancyRecord{Next: NativeRecordReference(binary.BigEndian.Uint16(bytes[0:2])), Previous: NativeRecordReference(binary.BigEndian.Uint16(bytes[2:4])), X: binary.BigEndian.Uint16(bytes[4:6]), Y: binary.BigEndian.Uint16(bytes[6:8])}, true
		},
		SetLinks: func(ref, next, previous NativeRecordReference) {
			address := 0x76c0 + int(int16(ref))
			if _, err := m.Write16(address+2, uint16(next)); err != nil {
				panic(err)
			}
			if _, err := m.Write16(address+4, uint16(previous)); err != nil {
				panic(err)
			}
		},
		SetPosition: func(ref NativeRecordReference, x, y uint16) {
			address := 0x76c0 + int(int16(ref))
			if _, err := m.Write16(address+6, x); err != nil {
				panic(err)
			}
			if _, err := m.Write16(address+8, y); err != nil {
				panic(err)
			}
		},
	}
}
