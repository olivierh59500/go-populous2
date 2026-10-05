package populous2

import (
	"encoding/binary"
	"fmt"
	"sort"

	"go-populous2/internal/amiga"
)

type nativeSharedCodeRelocation struct {
	Offset int
	Base   uint32
}

// NativeSharedCode has one authoritative physical HUNK0 byte owner. Scalar
// tables can alias those bytes directly; code interpreting linked pointer
// operands uses Logical(), not a second unrelocated byte-array copy.
type NativeSharedCode struct {
	Bytes       []byte
	CodeBase    uint32
	relocations []nativeSharedCodeRelocation
}

func NewNativeSharedCode(exe *amiga.Executable, physical []byte, bases []uint32) (*NativeSharedCode, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(bases) != len(exe.Hunks) || uint64(len(physical)) != exe.Hunks[0].AllocatedBytes {
		return nil, fmt.Errorf("native shared CODE actual allocation/layout missing")
	}
	byIndex := map[uint32]uint32{}
	for i, h := range exe.Hunks {
		if _, exists := byIndex[h.Index]; exists {
			return nil, fmt.Errorf("duplicate native HUNK index")
		}
		byIndex[h.Index] = bases[i]
	}
	s := &NativeSharedCode{Bytes: physical, CodeBase: bases[0]}
	for _, r := range exe.Hunks[0].Relocations {
		base, exists := byIndex[r.Target]
		if !exists || r.Bits != 32 {
			return nil, fmt.Errorf("native CODE relocation target/width missing")
		}
		for _, offset := range r.Offsets {
			if offset&1 != 0 || uint64(offset)+4 > uint64(len(physical)) {
				return nil, fmt.Errorf("native CODE relocation position invalid")
			}
			s.relocations = append(s.relocations, nativeSharedCodeRelocation{Offset: int(offset), Base: base})
		}
	}
	sort.Slice(s.relocations, func(i, j int) bool { return s.relocations[i].Offset < s.relocations[j].Offset })
	for i := 1; i < len(s.relocations); i++ {
		if s.relocations[i-1].Offset+4 > s.relocations[i].Offset {
			return nil, fmt.Errorf("native CODE relocation operands overlap")
		}
	}
	return s, nil
}

func (s *NativeSharedCode) relocation(at int) (nativeSharedCodeRelocation, bool) {
	i := sort.Search(len(s.relocations), func(i int) bool { return s.relocations[i].Offset > at }) - 1
	if i >= 0 && at < s.relocations[i].Offset+4 {
		return s.relocations[i], true
	}
	return nativeSharedCodeRelocation{}, false
}

func (s *NativeSharedCode) valid(at int) error {
	if s == nil || at < 0 || at >= len(s.Bytes) {
		return fmt.Errorf("native shared CODE byte offset %#x unavailable", at)
	}
	return nil
}

// Physical uses HUNK-relative offsets with raw relocated values. Its writes
// immediately affect scalar aliases and the Logical view of pointer operands.
func (s *NativeSharedCode) Physical() FollowerCleanupMemory {
	return nativeByteAddressMemory(func(at int) (byte, error) {
		if err := s.valid(at); err != nil {
			return 0, err
		}
		return s.Bytes[at], nil
	}, func(at int, v byte) error {
		if err := s.valid(at); err != nil {
			return err
		}
		s.Bytes[at] = v
		return nil
	})
}

// Logical applies linker subtraction/addition only at executable relocation
// operands. Byte/word operations overlapping such a LONG use that same linked
// value, including carry into another byte. Arithmetic wraps as original32bit
// addresses; NULL interpretation is never inferred from a numeric value.
func (s *NativeSharedCode) Logical() FollowerCleanupMemory {
	return nativeByteAddressMemory(func(at int) (byte, error) {
		if err := s.valid(at); err != nil {
			return 0, err
		}
		if r, ok := s.relocation(at); ok {
			value := binary.BigEndian.Uint32(s.Bytes[r.Offset:]) - r.Base
			return byte(value >> uint((3-(at-r.Offset))*8)), nil
		}
		return s.Bytes[at], nil
	}, func(at int, v byte) error {
		if err := s.valid(at); err != nil {
			return err
		}
		if r, ok := s.relocation(at); ok {
			value := binary.BigEndian.Uint32(s.Bytes[r.Offset:]) - r.Base
			shift := uint((3 - (at - r.Offset)) * 8)
			value = value&^(255<<shift) | uint32(v)<<shift
			binary.BigEndian.PutUint32(s.Bytes[r.Offset:], value+r.Base)
			return nil
		}
		s.Bytes[at] = v
		return nil
	})
}

// RawData returns the canonical physical alias. It is suitable only for
// scalar/table readers: procedure/pointer consumers must use Logical reads.
func (s *NativeSharedCode) RawData() []byte {
	if s == nil {
		return nil
	}
	return s.Bytes
}
