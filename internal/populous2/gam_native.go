package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const (
	NativeGAMStart = 0x0dd6
	NativeGAMEnd   = 0xeb48
	NativeGAMSize  = NativeGAMEnd - NativeGAMStart
)

// NativeGAMImage is the exact uncompressed BSS block written at $19afc.
// There is no magic, version, checksum or length prefix. Unknown bytes remain
// intact; actor references and scalar fields retain their big-endian encoding.
type NativeGAMImage struct {
	Bytes [NativeGAMSize]byte
}

func ValidateNativeGAMLayout(exe *amiga.Executable) error {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x19c74 {
		return fmt.Errorf("native GAM transfer instructions missing")
	}
	code := exe.Hunks[0].Data
	for _, at := range []int{0x19bbe, 0x19c68} {
		if binary.BigEndian.Uint16(code[at:]) != 0x2f3c || binary.BigEndian.Uint32(code[at+2:]) != NativeGAMStart || binary.BigEndian.Uint16(code[at+6:]) != 0x2f3c || binary.BigEndian.Uint32(code[at+8:]) != NativeGAMSize {
			return fmt.Errorf("unsupported native GAM transfer layout")
		}
	}
	return nil
}

// DecodeNativeGAM accepts a trailing file suffix, as native Read requests only
// NativeGAMSize bytes and checks its returned count. A short file is rejected
// here; the original reader's partial-memory mutation is a separate operation.
func DecodeNativeGAM(data []byte) (NativeGAMImage, error) {
	var image NativeGAMImage
	if len(data) < NativeGAMSize {
		return image, fmt.Errorf("native GAM contains %d bytes; need %d", len(data), NativeGAMSize)
	}
	copy(image.Bytes[:], data[:NativeGAMSize])
	return image, nil
}

func (image NativeGAMImage) MarshalBinary() ([]byte, error) {
	return append([]byte(nil), image.Bytes[:]...), nil
}

func (image *NativeGAMImage) span(address, length int) ([]byte, error) {
	if image == nil || length < 0 || address < NativeGAMStart || address > NativeGAMEnd-length {
		return nil, fmt.Errorf("native GAM access outside BSS image: %x length %d", address, length)
	}
	return image.Bytes[address-NativeGAMStart : address-NativeGAMStart+length], nil
}

func (image *NativeGAMImage) Read8(address int) (uint8, error) {
	data, err := image.span(address, 1)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}
func (image *NativeGAMImage) Read16(address int) (uint16, error) {
	data, err := image.span(address, 2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(data), nil
}
func (image *NativeGAMImage) Read32(address int) (uint32, error) {
	data, err := image.span(address, 4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(data), nil
}
func (image *NativeGAMImage) Write8(address int, value uint8) error {
	data, err := image.span(address, 1)
	if err != nil {
		return err
	}
	data[0] = value
	return nil
}
func (image *NativeGAMImage) Write16(address int, value uint16) error {
	data, err := image.span(address, 2)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(data, value)
	return nil
}
func (image *NativeGAMImage) Write32(address int, value uint32) error {
	data, err := image.span(address, 4)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint32(data, value)
	return nil
}

// FilePointers translates the caller's $4324 subtraction. RuntimePointers
// translates its $42dc restoration after a successful load. Only the two
// nonzero longs at F32/F36 are rebased; uint32 wrapping is native arithmetic.
func (image *NativeGAMImage) FilePointers(followerBase uint32) error {
	return image.rebase(followerBase, false)
}
func (image *NativeGAMImage) RuntimePointers(followerBase uint32) error {
	return image.rebase(followerBase, true)
}
func (image *NativeGAMImage) rebase(followerBase uint32, runtime bool) error {
	for _, address := range []int{0xf32, 0xf36} {
		value, err := image.Read32(address)
		if err != nil {
			return err
		}
		if value == 0 {
			continue
		}
		if runtime {
			value += followerBase
		} else {
			value -= followerBase
		}
		if err := image.Write32(address, value); err != nil {
			return err
		}
	}
	return nil
}

// CaptureNativeGAM requires a complete original BSS view. It fails at the first
// unretained byte rather than inventing zero padding from a partial World view.
// The callback's live pointers are left intact; only the captured file is rebased.
func CaptureNativeGAM(memory FollowerCleanupMemory, followerBase uint32) (NativeGAMImage, error) {
	var image NativeGAMImage
	if memory.Read8 == nil {
		return image, fmt.Errorf("native GAM source memory missing")
	}
	for index := range image.Bytes {
		value, err := memory.Read8(NativeGAMStart + index)
		if err != nil {
			return NativeGAMImage{}, fmt.Errorf("native GAM source byte %x: %w", NativeGAMStart+index, err)
		}
		image.Bytes[index] = value
	}
	if err := image.FilePointers(followerBase); err != nil {
		return NativeGAMImage{}, err
	}
	return image, nil
}

type NativeGAMReadResult struct {
	BytesRead int
	Complete  bool
}

// ReadNativeGAMInto reproduces a successful DOS Read's prefix writes and the
// caller's conditional relocation. Short reads leave the old tail unchanged
// and do not relocate pointers. Graphics reloading at $1a32a occurs even after
// native I/O failure; that external presentation operation is not this codec.
func ReadNativeGAMInto(memory FollowerCleanupMemory, data []byte, followerBase uint32) (NativeGAMReadResult, error) {
	var result NativeGAMReadResult
	if memory.Write8 == nil {
		return result, fmt.Errorf("native GAM destination memory missing")
	}
	result.BytesRead = min(len(data), NativeGAMSize)
	for index := range result.BytesRead {
		if err := memory.Write8(NativeGAMStart+index, data[index]); err != nil {
			return result, fmt.Errorf("native GAM destination byte %x: %w", NativeGAMStart+index, err)
		}
	}
	result.Complete = result.BytesRead == NativeGAMSize
	if !result.Complete {
		return result, nil
	}
	if memory.Read32 == nil || memory.Write32 == nil {
		return result, fmt.Errorf("native GAM pointer memory missing")
	}
	for _, address := range []int{0xf32, 0xf36} {
		value, err := memory.Read32(address)
		if err != nil {
			return result, err
		}
		if value != 0 {
			if err := memory.Write32(address, value+followerBase); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

type NativeGAMSession struct {
	Clock, Seed, Random                     uint32
	Landscape, ProfileSide, GameMode, World uint16
	Rules                                   [2]uint16
	Name                                    [16]byte
	AfterName                               [2]byte
}

func (image *NativeGAMImage) Session() (NativeGAMSession, error) {
	var session NativeGAMSession
	for _, field := range []struct {
		address int
		value   *uint32
	}{{0xf40, &session.Clock}, {0xeb24, &session.Seed}, {0xeb28, &session.Random}} {
		value, err := image.Read32(field.address)
		if err != nil {
			return session, err
		}
		*field.value = value
	}
	for _, field := range []struct {
		address int
		value   *uint16
	}{{0xeb22, &session.Landscape}, {0xeb2c, &session.Rules[0]}, {0xeb2e, &session.Rules[1]}, {0xeb42, &session.ProfileSide}, {0xeb44, &session.GameMode}, {0xeb46, &session.World}} {
		value, err := image.Read16(field.address)
		if err != nil {
			return session, err
		}
		*field.value = value
	}
	data, err := image.span(0xeb30, len(session.Name))
	if err != nil {
		return session, err
	}
	copy(session.Name[:], data)
	data, err = image.span(0xeb40, len(session.AfterName))
	if err != nil {
		return session, err
	}
	copy(session.AfterName[:], data)
	return session, nil
}

// NativeGAMPath follows $420e..$4276. The suffix comparison is case-sensitive;
// spaces are retained. Drawer names ending ':' or '/' need no separator.
// This pure helper does not open a path or emulate native scratch-buffer spills.
func NativeGAMPath(drawer, filename []byte) []byte {
	cstring := func(value []byte) []byte {
		if end := bytes.IndexByte(value, 0); end >= 0 {
			return value[:end]
		}
		return value
	}
	drawer, filename = cstring(drawer), cstring(filename)
	name := append([]byte(nil), filename...)
	if !bytes.HasSuffix(name, []byte(".GAM")) {
		name = append(name, []byte(".GAM")...)
	}
	path := append([]byte(nil), drawer...)
	if len(path) > 0 && path[len(path)-1] != ':' && path[len(path)-1] != '/' {
		path = append(path, '/')
	}
	return append(path, name...)
}
