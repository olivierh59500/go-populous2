package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeFullMinimapAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_minimap_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Input struct {
				Land      int
				Seed      uint8
				DX, DY    uint16
				Procedure uint32
				Pattern   bool
			}
			D    [8]uint32
			Hash string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 192 {
		t.Fatal("native minimap corpus incomplete")
	}
	b := testBundle(t)
	for _, f := range corpus.Cases {
		code := append([]byte(nil), b.Executable.Hunks[0].Data...)
		copy(code[0x3365a:], b.Raw[fmt.Sprintf("land%d.dat", f.Input.Land)])
		binary.BigEndian.PutUint16(code[0x33612:], f.Input.DX)
		binary.BigEndian.PutUint16(code[0x33614:], f.Input.DY)
		binary.BigEndian.PutUint32(code[0x33616:], f.Input.Procedure)
		raw := make([]byte, 0x11280)
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				raw[0xf45+(x+y*64)*4] = byte(x*17 + y*43 + int(f.Input.Seed))
			}
		}
		bitmap := make([]byte, 32000)
		if f.Input.Pattern {
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
			}
		}
		frame := NativeFrameRegisterContext{D: [8]uint32{0x12345678, 0x89abcdef, 0x13572468, 0x24681357, 0xaabbccdd, 0x11226778, 0x12345678, 0x33445566}}
		if err := DrawNativeMinimapFrame(code, commandNativeMemory(raw), &frame, bitmap); err != nil {
			t.Fatal(f.Input, err)
		}
		if frame.D != f.D {
			t.Fatalf("native minimap registers differ %+v: %x/%x", f.Input, frame.D, f.D)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.Hash {
			t.Fatalf("native minimap bitmap differs %+v: %s/%s", f.Input, got, f.Hash)
		}
	}
}
