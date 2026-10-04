package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ApplySpriteDifference translates 0x1a51e. Each 10-byte record supplies a
// source file offset, destination buffer offset and length-minus-one. A source
// offset of 0xffffffff terminates the list. Both DIF and decoded PIF use this
// format. The base is copied so switching landscapes is independent of order.
func ApplySpriteDifference(base, difference []byte) ([]byte, error) {
	result := bytes.Clone(base)
	for p := 0; p+10 <= len(difference); p += 10 {
		source := binary.BigEndian.Uint32(difference[p:])
		if source == 0xffffffff {
			return result, nil
		}
		destination := binary.BigEndian.Uint32(difference[p+4:])
		length := uint64(binary.BigEndian.Uint16(difference[p+8:])) + 1
		if uint64(source)+length > uint64(len(difference)) || uint64(destination)+length > uint64(len(result)) {
			return nil, fmt.Errorf("sprite difference record at %d is outside source or destination", p)
		}
		copy(result[int(destination):int(uint64(destination)+length)], difference[int(source):int(uint64(source)+length)])
	}
	return nil, fmt.Errorf("sprite difference has no complete terminator")
}
