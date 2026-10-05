package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeGAMFixture struct {
	Input struct {
		Name, Mode, Drawer, Filename string
		Pattern                      int
		Pointer1, Pointer2           uint32
		Length, Result               int
		Exists                       bool
		Confirm                      uint16
		Reference                    bool
	}
	D0                                    uint32
	Stop, MemoryHash, RequestedHash, Path string
	RequestedBytes                        int
	Pointers                              [2]uint32
	Calls                                 []string
}

func gamFixtureMemory(image *NativeGAMImage) FollowerCleanupMemory {
	return FollowerCleanupMemory{Read8: image.Read8, Read16: image.Read16, Read32: image.Read32, Write8: image.Write8, Write16: image.Write16, Write32: image.Write32}
}

func gamHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// These original CPU cases execute the complete original transfer bodies and
// callers. File I/O is deterministic; graphics/OS side effects are explicit
// callback boundaries, separate from the codec's exact saved-memory block.
func TestNativeGAMAgainstOriginalTransfers(t *testing.T) {
	data, err := os.ReadFile("testdata/gam_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases     []nativeGAMFixture
		Reference struct {
			SHA256  string
			Size    int
			Session NativeGAMSession
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 61 {
		t.Fatal("native GAM fixture catalog is incomplete")
	}
	if err := ValidateNativeGAMLayout(testBundle(t).Executable); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			c := fixture.Input
			if c.Mode == "path" {
				if got := string(NativeGAMPath([]byte(c.Drawer), []byte(c.Filename))); got != fixture.Path {
					t.Fatalf("native filename/path differs: %q/%q", got, fixture.Path)
				}
				return
			}
			var image NativeGAMImage
			for index := range image.Bytes {
				image.Bytes[index] = byte(index*13 + 17)
			}
			if err := image.Write32(0xf32, c.Pointer1); err != nil {
				t.Fatal(err)
			}
			if err := image.Write32(0xf36, c.Pointer2); err != nil {
				t.Fatal(err)
			}
			memory := gamFixtureMemory(&image)
			if c.Mode == "save" {
				before := image
				captured, err := CaptureNativeGAM(memory, 0x2076c0)
				if err != nil {
					t.Fatal(err)
				}
				if image != before {
					t.Fatal("pure export changed its source memory")
				}
				file, err := captured.MarshalBinary()
				if err != nil {
					t.Fatal(err)
				}
				if len(file) != NativeGAMSize || fixture.RequestedBytes == NativeGAMSize && gamHash(file) != fixture.RequestedHash {
					t.Fatal("original Write's complete source block differs")
				}
				if err := captured.RuntimePointers(0x2076c0); err != nil {
					t.Fatal(err)
				}
				if gamHash(captured.Bytes[:]) != fixture.MemoryHash {
					t.Fatal("native caller pointer restoration differs, including reserved-base zero alias")
				}
				return
			}
			file := make([]byte, NativeGAMSize+9)
			for index := range file {
				file[index] = byte(index*7 + c.Pattern)
			}
			binary.BigEndian.PutUint32(file[0xf32-NativeGAMStart:], c.Pointer1)
			binary.BigEndian.PutUint32(file[0xf36-NativeGAMStart:], c.Pointer2)
			if c.Reference {
				file, err = os.ReadFile("../../.local/native-audit/extracted-pop2-b/ARNY 1.GAM")
				if os.IsNotExist(err) {
					t.Skip("supplied original GAM remains private and excluded from source control")
				}
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := DecodeNativeGAM(file)
				if err != nil {
					t.Fatal(err)
				}
				roundtrip, err := decoded.MarshalBinary()
				if err != nil || !bytes.Equal(file, roundtrip) || gamHash(roundtrip) != catalog.Reference.SHA256 {
					t.Fatal("supplied native GAM byte roundtrip differs")
				}
				session, err := decoded.Session()
				if err != nil || !reflect.DeepEqual(session, catalog.Reference.Session) {
					t.Fatal("supplied native GAM session fields differ")
				}
			}
			file = file[:c.Length]
			if c.Result < 0 {
				file = nil
			}
			result, err := ReadNativeGAMInto(memory, file, 0x2076c0)
			if err != nil {
				t.Fatal(err)
			}
			if result.BytesRead != min(len(file), NativeGAMSize) || result.Complete != (len(file) >= NativeGAMSize) || gamHash(image.Bytes[:]) != fixture.MemoryHash {
				t.Fatal("original Read prefix writes or conditional pointer restoration differs")
			}
		})
	}
}

func TestNativeGAMSizeEndianAndUnretainedBytes(t *testing.T) {
	if NativeGAMSize != 56690 {
		t.Fatal("native GAM fixed transfer size differs")
	}
	for _, size := range []int{0, 1, NativeGAMSize - 1} {
		if _, err := DecodeNativeGAM(make([]byte, size)); err == nil {
			t.Fatal("short GAM file was accepted")
		}
	}
	data := make([]byte, NativeGAMSize+7)
	for index := range data {
		data[index] = byte(index*11 + 7)
	}
	image, err := DecodeNativeGAM(data)
	if err != nil {
		t.Fatal(err)
	}
	copyData, err := image.MarshalBinary()
	if err != nil || !bytes.Equal(copyData, data[:NativeGAMSize]) {
		t.Fatal("trailing-byte native acceptance or raw preservation differs")
	}
	copyData[0] ^= 255
	if image.Bytes[0] != data[0] {
		t.Fatal("exported bytes alias their retained image")
	}
	if err := image.Write32(0xf40, 0x11223344); err != nil {
		t.Fatal(err)
	}
	if value, err := image.Read16(0xf42); err != nil || value != 0x3344 {
		t.Fatal("native overlapping big-endian access differs")
	}
	for _, address := range []int{NativeGAMStart - 1, NativeGAMEnd - 3} {
		if _, err := image.Read32(address); err == nil {
			t.Fatal("out-of-span native access was accepted")
		}
	}
	partial := gamFixtureMemory(&image)
	partial.Read8 = func(address int) (uint8, error) {
		if address == 0x5f44 {
			return 0, fmt.Errorf("unretained view bytes")
		}
		return image.Read8(address)
	}
	if _, err := CaptureNativeGAM(partial, 0x2076c0); err == nil {
		t.Fatal("partial memory view silently invented save bytes")
	}
}
