package populous2

import (
	"encoding/binary"
	"fmt"
	"sort"

	"go-populous2/internal/amiga"
)

type NativeHostRegion struct {
	Name  string
	Base  uint32
	Bytes []byte
}

// NativeHostMemory owns only explicitly allocated physical regions. Gaps
// between relocated HUNKs are unavailable, rather than invented zero RAM.
type NativeHostMemory struct{ Regions []NativeHostRegion }

// NewNativeHunkMemory uses the caller's explicit load addresses. Each HUNK
// has its actual allocation size, including initialized payload and BSS.
func NewNativeHunkMemory(exe *amiga.Executable, bases []uint32) (*NativeHostMemory, error) {
	if exe == nil || len(bases) != len(exe.Hunks) {
		return nil, fmt.Errorf("native HUNK layout missing")
	}
	m := &NativeHostMemory{}
	indices := map[uint32]uint32{}
	for i, hunk := range exe.Hunks {
		if hunk.AllocatedBytes == 0 || hunk.AllocatedBytes > MaxDecodedBytes || uint64(len(hunk.Data)) > hunk.AllocatedBytes {
			return nil, fmt.Errorf("native HUNK%d allocation invalid", hunk.Index)
		}
		if _, exists := indices[hunk.Index]; exists {
			return nil, fmt.Errorf("native HUNK index duplicated")
		}
		indices[hunk.Index] = bases[i]
		data := make([]byte, int(hunk.AllocatedBytes))
		copy(data, hunk.Data)
		if err := m.MapRegion(NativeHostRegion{Name: fmt.Sprintf("HUNK%d %s", hunk.Index, hunk.Kind), Base: bases[i], Bytes: data}); err != nil {
			return nil, err
		}
	}
	for i, hunk := range exe.Hunks {
		for _, relocation := range hunk.Relocations {
			if relocation.Bits != 32 {
				return nil, fmt.Errorf("native HUNK relocation width%d unsupported", relocation.Bits)
			}
			target, exists := indices[relocation.Target]
			if !exists {
				return nil, fmt.Errorf("native relocation target%d missing", relocation.Target)
			}
			for _, offset := range relocation.Offsets {
				if offset&1 != 0 || uint64(offset)+4 > hunk.AllocatedBytes {
					return nil, fmt.Errorf("native HUNK%d relocation offset%x invalid", hunk.Index, offset)
				}
				span, err := m.Span(bases[i]+offset, 4)
				if err != nil {
					return nil, err
				}
				value := binary.BigEndian.Uint32(span)
				if uint64(value)+uint64(target) > 0xffffffff {
					return nil, fmt.Errorf("native relocated address overflow")
				}
				binary.BigEndian.PutUint32(span, value+target)
			}
		}
	}
	return m, nil
}

// MapRegion borrows real caller-owned RAM, such as an actual resource or
// audio allocation. It rejects overlap and never pads another region.
func (m *NativeHostMemory) MapRegion(region NativeHostRegion) error {
	if m == nil || region.Base&1 != 0 || len(region.Bytes) == 0 || uint64(region.Base)+uint64(len(region.Bytes)) > 1<<32 {
		return fmt.Errorf("native physical region invalid")
	}
	end := uint64(region.Base) + uint64(len(region.Bytes))
	for _, existing := range m.Regions {
		if uint64(region.Base) < uint64(existing.Base)+uint64(len(existing.Bytes)) && uint64(existing.Base) < end {
			return fmt.Errorf("native physical regions overlap")
		}
	}
	m.Regions = append(m.Regions, region)
	sort.Slice(m.Regions, func(i, j int) bool { return m.Regions[i].Base < m.Regions[j].Base })
	return nil
}

// Span returns an alias of one actual allocation. Cross-region operations
// use Memory's byte callbacks; a bitmap window cannot bridge detached slices.
func (m *NativeHostMemory) Span(address uint32, length int) ([]byte, error) {
	if m == nil || length < 0 || uint64(address)+uint64(length) > 1<<32 {
		return nil, fmt.Errorf("native physical span invalid")
	}
	for _, region := range m.Regions {
		if address >= region.Base {
			offset := uint64(address - region.Base)
			if offset+uint64(length) <= uint64(len(region.Bytes)) {
				return region.Bytes[int(offset) : int(offset)+length], nil
			}
		}
	}
	return nil, fmt.Errorf("native physical address%x length%d unavailable", address, length)
}

func (m *NativeHostMemory) BitmapWindow(address uint32) (NativeBitmapWindow, error) {
	if m != nil {
		for _, region := range m.Regions {
			if address >= region.Base {
				offset := uint64(address - region.Base)
				if offset+32000 <= uint64(len(region.Bytes)) {
					return NativeBitmapWindow{Bytes: region.Bytes, BitmapOffset: int(offset)}, nil
				}
			}
		}
	}
	return NativeBitmapWindow{}, fmt.Errorf("native physical bitmap%x unavailable", address)
}

func (m *NativeHostMemory) Memory() FollowerCleanupMemory {
	var out FollowerCleanupMemory
	valid := func(at int, width int) error {
		if at < 0 || uint64(at)+uint64(width) > 1<<32 || width > 1 && at&1 != 0 {
			return fmt.Errorf("native physical access%x width%d invalid", at, width)
		}
		return nil
	}
	out.Read8 = func(at int) (uint8, error) {
		if err := valid(at, 1); err != nil {
			return 0, err
		}
		span, err := m.Span(uint32(at), 1)
		if err != nil {
			return 0, err
		}
		return span[0], nil
	}
	out.Write8 = func(at int, value uint8) error {
		if err := valid(at, 1); err != nil {
			return err
		}
		span, err := m.Span(uint32(at), 1)
		if err != nil {
			return err
		}
		span[0] = value
		return nil
	}
	read := func(at, width int) (uint32, error) {
		if err := valid(at, width); err != nil {
			return 0, err
		}
		var value uint32
		for i := 0; i < width; i++ {
			b, err := out.Read8(at + i)
			if err != nil {
				return 0, err
			}
			value = value<<8 | uint32(b)
		}
		return value, nil
	}
	write := func(at, width int, value uint32) error {
		if err := valid(at, width); err != nil {
			return err
		}
		for i := 0; i < width; i++ {
			if err := out.Write8(at+i, uint8(value>>uint((width-1-i)*8))); err != nil {
				return err
			}
		}
		return nil
	}
	out.Read16 = func(at int) (uint16, error) { value, err := read(at, 2); return uint16(value), err }
	out.Read32 = func(at int) (uint32, error) { return read(at, 4) }
	out.Write16 = func(at int, value uint16) error { return write(at, 2, uint32(value)) }
	out.Write32 = func(at int, value uint32) error { return write(at, 4, value) }
	return out
}
