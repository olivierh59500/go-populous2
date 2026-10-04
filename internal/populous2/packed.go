// Package populous2 contains formats and rules specific to Populous II. It does
// not assume that the Populous 1 simulation or asset formats are interchangeable.
package populous2

import (
	"encoding/binary"
	"fmt"
)

const MaxDecodedBytes = 16 << 20

// DecodePacked translates the backwards decompressor at hunk 0 offset 0x105ca
// in the supplied French populous.ii (SHA-256 c148b9bd...bfee803). The trailer is
// the initial bit register, XOR checksum and decoded length, all big endian.
// This ByteKiller variant is selected by the resource table's packed flag, not
// by a filename extension: some .DAT and .DIF files are intentionally unpacked.
func DecodePacked(data []byte) ([]byte, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("packed resource is shorter than its 12-byte trailer")
	}
	end := len(data)
	length := binary.BigEndian.Uint32(data[end-4:])
	if length == 0 || length > MaxDecodedBytes {
		return nil, fmt.Errorf("invalid decoded resource size %d (maximum %d)", length, MaxDecodedBytes)
	}
	bits := binary.BigEndian.Uint32(data[end-12 : end-8])
	r := packedReader{
		data:     data,
		pos:      end - 12,
		bits:     bits,
		checksum: binary.BigEndian.Uint32(data[end-8:end-4]) ^ bits,
	}
	output := make([]byte, int(length))
	position := len(output)
	for position > 0 {
		first, err := r.read(1)
		if err != nil {
			return nil, err
		}
		literal, count, offsetBits := false, 0, 0
		if first == 0 {
			second, err := r.read(1)
			if err != nil {
				return nil, err
			}
			if second == 0 {
				n, err := r.read(3)
				if err != nil {
					return nil, err
				}
				literal, count = true, int(n)+1
			} else {
				count, offsetBits = 2, 8
			}
		} else {
			mode, err := r.read(2)
			if err != nil {
				return nil, err
			}
			switch mode {
			case 0, 1:
				count, offsetBits = int(mode)+3, int(mode)+9
			case 2:
				n, err := r.read(8)
				if err != nil {
					return nil, err
				}
				count, offsetBits = int(n)+1, 12
			case 3:
				n, err := r.read(8)
				if err != nil {
					return nil, err
				}
				literal, count = true, int(n)+9
			}
		}
		if count > position {
			return nil, fmt.Errorf("packed run of %d bytes exceeds %d remaining output bytes", count, position)
		}
		if literal {
			for range count {
				value, err := r.read(8)
				if err != nil {
					return nil, err
				}
				position--
				output[position] = byte(value)
			}
		} else {
			offset, err := r.read(offsetBits)
			if err != nil {
				return nil, err
			}
			if offset == 0 || uint64(position-1)+uint64(offset) >= uint64(len(output)) {
				return nil, fmt.Errorf("packed reference offset %d points outside decoded suffix", offset)
			}
			for range count {
				position--
				output[position] = output[position+int(offset)]
			}
		}
	}
	if r.checksum != 0 {
		return nil, fmt.Errorf("packed resource checksum mismatch: 0x%08x", r.checksum)
	}
	return output, nil
}

type packedReader struct {
	data     []byte
	pos      int
	bits     uint32
	checksum uint32
}

func (r *packedReader) read(count int) (uint32, error) {
	var result uint32
	for range count {
		bit := r.bits & 1
		r.bits >>= 1
		if r.bits == 0 {
			if r.pos < 4 {
				return 0, fmt.Errorf("packed bitstream is truncated")
			}
			r.pos -= 4
			word := binary.BigEndian.Uint32(r.data[r.pos : r.pos+4])
			r.checksum ^= word
			bit = word & 1
			// The original ROXR inserts X=1 as a sentinel in bit 31.
			r.bits = word>>1 | 0x80000000
		}
		result = result<<1 | bit
	}
	return result, nil
}
