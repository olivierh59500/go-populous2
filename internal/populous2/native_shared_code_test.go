package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func nativeSharedCodeTestView(t *testing.T) (*NativeSharedCode, *NativeHostMemory) {
	t.Helper()
	exe := testBundle(t).Executable
	bases := []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000}
	host, err := NewNativeHunkMemory(exe, bases)
	if err != nil {
		t.Fatal(err)
	}
	physical, err := host.Span(bases[0], int(exe.Hunks[0].AllocatedBytes))
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewNativeSharedCode(exe, physical, bases)
	if err != nil {
		t.Fatal(err)
	}
	return view, host
}

func TestNativeSharedCodeRelocationsAndPartialWrites(t *testing.T) {
	view, host := nativeSharedCodeTestView(t)
	original := testBundle(t).Executable.Hunks[0].Data
	logical := view.Logical()
	physical := view.Physical()
	for at, want := range original {
		got, err := logical.Read8(at)
		if err != nil || got != want {
			t.Fatalf("original logical byte%x differs: %x/%x/%v", at, got, want, err)
		}
	}
	for _, relocation := range view.relocations {
		start := relocation.Offset
		value := binary.BigEndian.Uint32(original[start:])
		got, err := physical.Read32(start)
		if err != nil || got != value+relocation.Base {
			t.Fatalf("actual relocation operand%x differs", start)
		}
		for index := 0; index < 4; index++ {
			// Independent expected linked arithmetic, including carries that
			// make a partial logical write change another physical byte.
			shift := uint((3 - index) * 8)
			value = value&^(255<<shift) | uint32(byte(0x90+index))<<shift
			if err := logical.Write8(start+index, byte(0x90+index)); err != nil {
				t.Fatal(err)
			}
			if got := binary.BigEndian.Uint32(view.Bytes[start:]); got != value+relocation.Base {
				t.Fatalf("partial pointer byte%x has stale physical value", start+index)
			}
		}
		if err := logical.Write16(start+2, 0xfffe); err != nil {
			t.Fatal(err)
		}
		value = value&0xffff0000 | 0xfffe
		if got := binary.BigEndian.Uint32(view.Bytes[start:]); got != value+relocation.Base {
			t.Fatal("partial pointer WORD is incoherent")
		}
		if err := physical.Write32(start, relocation.Base+0x1234); err != nil {
			t.Fatal(err)
		}
		if got, err := logical.Read32(start); err != nil || got != 0x1234 {
			t.Fatal("physical pointer mutation is stale in logical view")
		}
	}
	// Mutable instruction/data words share the exact allocation, without a
	// frame-time synchronization list between World scalars and host CODE.
	alias := view.RawData()
	if err := physical.Write16(0x3f90, 0x7241); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint16(alias[0x3f90:]) != 0x7241 {
		t.Fatal("canonical scalar alias detached")
	}
	alias[0x4468] = 0x56
	if got, _ := host.Memory().Read8(0x100000 + 0x4468); got != 0x56 {
		t.Fatal("World scalar mutation detached from physical HUNK")
	}
	if err := logical.Write32(0x33616, 0xe17a); err != nil {
		t.Fatal(err)
	}
	if got, _ := host.Memory().Read32(0x100000 + 0x33616); got != 0x10e17a {
		t.Fatalf("actual minimap pointer is not relocated: %x", got)
	}
	if _, err := logical.Read16(1); err == nil {
		t.Fatal("unaligned native WORD normalized")
	}
	if _, err := physical.Read8(len(alias)); err == nil {
		t.Fatal("unmapped CODE padded")
	}
}

func TestNativeSharedCodeMinimapAgainstOriginalCPU(t *testing.T) {
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
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 192 {
		t.Fatalf("actual minimap corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	for _, f := range corpus.Cases {
		view, host := nativeSharedCodeTestView(t)
		code := view.Physical()
		land := bundle.Raw[fmt.Sprintf("land%d.dat", f.Input.Land)]
		copy(view.RawData()[0x3365a:], land)
		if err := code.Write16(0x33612, f.Input.DX); err != nil {
			t.Fatal(err)
		}
		if err := code.Write16(0x33614, f.Input.DY); err != nil {
			t.Fatal(err)
		}
		if err := host.Memory().Write32(0x100000+0x33616, 0x100000+f.Input.Procedure); err != nil {
			t.Fatal(err)
		}
		before := append([]byte(nil), view.RawData()...)
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
		if err := DrawNativeSharedCodeMinimap(view, commandNativeMemory(raw), &frame, bitmap); err != nil {
			t.Fatal(f.Input, err)
		}
		if frame.D != f.D || fileFrameHash(bitmap) != f.Hash {
			t.Fatalf("physical/logical CODE minimap differs from original: %+v", f.Input)
		}
		if !bytes.Equal(before, view.RawData()) {
			t.Fatal("drawing patched/copied canonical procedure words")
		}
	}
}
