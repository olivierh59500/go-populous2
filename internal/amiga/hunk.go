package amiga

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	HunkCode   = 1001
	HunkData   = 1002
	HunkBSS    = 1003
	HunkHeader = 1011
)

type Symbol struct {
	Name   string `json:"name"`
	Offset uint32 `json:"offset"`
}

type Relocation struct {
	Bits    int      `json:"bits"`
	Target  uint32   `json:"target_hunk"`
	Offsets []uint32 `json:"offsets"`
}

// Hunk offsets are relative to their segment, not absolute 68000 addresses.
// Data is the initialized code/data payload; BSS occupies no bytes in the file.
// No instructions are executed and no load addresses are invented.
type Hunk struct {
	Index          uint32       `json:"index"`
	Kind           string       `json:"kind"`
	Name           string       `json:"name,omitempty"`
	Memory         string       `json:"memory"`
	MemoryFlags    uint32       `json:"memory_flags,omitempty"`
	AllocatedBytes uint64       `json:"allocated_bytes"`
	PayloadBytes   uint64       `json:"payload_bytes"`
	FileOffset     int          `json:"file_offset"`
	Relocations    []Relocation `json:"relocations,omitempty"`
	Symbols        []Symbol     `json:"symbols,omitempty"`
	Data           []byte       `json:"-"`
}

type Executable struct {
	Libraries []string `json:"libraries,omitempty"`
	Hunks     []Hunk   `json:"hunks"`
}

// IsExecutable recognizes the load-file signature. It does not recognize raw
// 68000 blobs or packed code stored inside another executable.
func IsExecutable(data []byte) bool {
	return len(data) >= 4 && binary.BigEndian.Uint32(data[:4]) == HunkHeader
}

// ParseExecutable reads HUNK_HEADER load files with CODE/DATA/BSS, relocation,
// symbol, name and debug records. Unknown records are errors instead of guessed
// lengths. Object files and overlays require a different loader.
func ParseExecutable(data []byte) (*Executable, error) {
	if !IsExecutable(data) {
		return nil, fmt.Errorf("not an Amiga HUNK_HEADER load file")
	}
	r := hunkReader{data: data, pos: 4}
	exe := &Executable{}
	for {
		name, err := r.name()
		if err != nil {
			return nil, err
		}
		if name == "" {
			break
		}
		exe.Libraries = append(exe.Libraries, name)
	}
	table, err := r.u32()
	if err != nil {
		return nil, err
	}
	first, err := r.u32()
	if err != nil {
		return nil, err
	}
	last, err := r.u32()
	if err != nil {
		return nil, err
	}
	if last < first || last >= table || uint64(last)-uint64(first)+1 > uint64(r.remaining()/4) {
		return nil, fmt.Errorf("invalid hunk table: size %d, range %d..%d", table, first, last)
	}
	count := int(last-first) + 1
	exe.Hunks = make([]Hunk, count)
	for i := range exe.Hunks {
		size, err := r.u32()
		if err != nil {
			return nil, err
		}
		h := &exe.Hunks[i]
		h.Index = first + uint32(i)
		h.AllocatedBytes = uint64(size&0x3fffffff) * 4
		switch size >> 30 {
		case 0:
			h.Memory = "any"
		case 1:
			h.Memory = "chip"
		case 2:
			h.Memory = "fast"
		case 3:
			h.Memory = "explicit"
			h.MemoryFlags, err = r.u32()
			if err != nil {
				return nil, err
			}
		}
	}
	for i := 0; i < count; {
		h := &exe.Hunks[i]
		recordOffset := r.pos
		record, err := r.u32()
		if err != nil {
			return nil, err
		}
		record &= 0x3fffffff
		switch record {
		case 1000: // HUNK_NAME
			h.Name, err = r.name()
		case HunkCode, HunkData, HunkBSS:
			if h.Kind != "" {
				return nil, fmt.Errorf("multiple payloads in hunk %d", h.Index)
			}
			var longs uint32
			longs, err = r.u32()
			if err != nil {
				return nil, err
			}
			h.PayloadBytes = uint64(longs) * 4
			h.FileOffset = r.pos
			if h.PayloadBytes > h.AllocatedBytes {
				return nil, fmt.Errorf("hunk %d payload exceeds allocation", h.Index)
			}
			h.Kind = map[uint32]string{HunkCode: "code", HunkData: "data", HunkBSS: "bss"}[record]
			if record != HunkBSS {
				var payload []byte
				payload, err = r.take(h.PayloadBytes)
				h.Data = bytes.Clone(payload)
			}
		case 1004, 1005, 1006: // RELOC32, RELOC16, RELOC8
			if h.Kind == "" {
				return nil, fmt.Errorf("relocations precede hunk %d payload", h.Index)
			}
			bits := map[uint32]int{1004: 32, 1005: 16, 1006: 8}[record]
			err = r.relocations(h, bits, false, first, last)
		case 1015, 1020: // Load-file DREL32 / RELOC32SHORT (Kickstart 2+)
			if h.Kind == "" {
				return nil, fmt.Errorf("relocations precede hunk %d payload", h.Index)
			}
			err = r.relocations(h, 32, true, first, last)
		case 1008: // HUNK_SYMBOL
			for {
				var name string
				name, err = r.name()
				if err != nil || name == "" {
					break
				}
				var offset uint32
				offset, err = r.u32()
				if err != nil {
					break
				}
				h.Symbols = append(h.Symbols, Symbol{Name: name, Offset: offset})
			}
		case 1009: // HUNK_DEBUG
			var longs uint32
			longs, err = r.u32()
			if err == nil {
				_, err = r.take(uint64(longs) * 4)
			}
		case 1010: // HUNK_END
			if h.Kind == "" {
				return nil, fmt.Errorf("hunk %d ended without a payload", h.Index)
			}
			i++
		default:
			return nil, fmt.Errorf("unsupported hunk record 0x%x at file offset 0x%x", record, recordOffset)
		}
		if err != nil {
			return nil, err
		}
	}
	// Some tools pad files with zero longwords. Nonzero trailers may be a packed
	// overlay or another executable and must not disappear from an analysis.
	for _, b := range data[r.pos:] {
		if b != 0 {
			return nil, fmt.Errorf("unparsed nonzero trailer at file offset 0x%x", r.pos)
		}
	}
	return exe, nil
}

type hunkReader struct {
	data []byte
	pos  int
}

func (r *hunkReader) remaining() int { return len(r.data) - r.pos }

func (r *hunkReader) take(size uint64) ([]byte, error) {
	if size > uint64(r.remaining()) {
		return nil, fmt.Errorf("truncated hunk record at file offset 0x%x: need %d bytes, have %d", r.pos, size, r.remaining())
	}
	start := r.pos
	r.pos += int(size)
	return r.data[start:r.pos], nil
}

func (r *hunkReader) u32() (uint32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

func (r *hunkReader) u16() (uint32, error) {
	b, err := r.take(2)
	if err != nil {
		return 0, err
	}
	return uint32(binary.BigEndian.Uint16(b)), nil
}

func (r *hunkReader) name() (string, error) {
	longs, err := r.u32()
	if err != nil {
		return "", err
	}
	b, err := r.take(uint64(longs) * 4)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(b, "\x00")), nil
}

func (r *hunkReader) relocations(h *Hunk, bits int, short bool, first, last uint32) error {
	read := r.u32
	if short {
		read = r.u16
	}
	for {
		count, err := read()
		if err != nil {
			return err
		}
		if count == 0 {
			if short && r.pos%4 != 0 {
				_, err = r.take(2)
			}
			return err
		}
		target, err := read()
		if err != nil {
			return err
		}
		width := 4
		if short {
			width = 2
		}
		if target < first || target > last || uint64(count) > uint64(r.remaining()/width) {
			return fmt.Errorf("invalid relocation group in hunk %d", h.Index)
		}
		reloc := Relocation{Bits: bits, Target: target, Offsets: make([]uint32, count)}
		for i := range reloc.Offsets {
			offset, err := read()
			if err != nil {
				return err
			}
			if uint64(offset)+uint64(bits/8) > h.AllocatedBytes {
				return fmt.Errorf("relocation at 0x%x exceeds hunk %d allocation", offset, h.Index)
			}
			reloc.Offsets[i] = offset
		}
		h.Relocations = append(h.Relocations, reloc)
	}
}
