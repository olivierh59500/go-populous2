package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type debugFrameFixture struct {
	Input struct {
		Name         string
		Format       []byte
		D            [8]uint32
		A            [7]uint32
		Prefix       []byte
		Code         []nativeHeroPatch
		Target       bool
		FormatOffset uint32
	}
	D                          [8]uint32
	A                          [8]uint32
	ReturnAddress              uint32
	Changes                    []nativeHeroPatch
	Text                       []byte
	BSSHash, BitmapHash, Error string
}

func TestNativeDebugOverlayAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_debug_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []debugFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 267 {
		t.Fatalf("native debug corpus incomplete: %v", err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original debug capture failed", f.Error)
			}
			code := fileFrameRelocatedCode(t)
			cm := commandNativeMemory(code)
			target := uint32(0)
			if f.Input.Target {
				target = 0xa10100
			}
			if err := cm.Write32(0x2e3a, target); err != nil {
				t.Fatal(err)
			}
			for _, p := range f.Input.Code {
				renderFramePatch(cm, p)
			}
			copy(code[f.Input.FormatOffset:], f.Input.Format)
			expected := append([]byte(nil), code...)
			for _, p := range f.Changes {
				renderFramePatch(commandNativeMemory(expected), p)
			}
			window := make([]byte, 33024)
			for i := range window {
				window[i] = byte(i*7 + 13)
			}
			registers := make([]byte, 96)
			copy(registers, f.Input.Prefix)
			for i, v := range f.Input.D {
				binary.BigEndian.PutUint32(registers[32+i*4:], v)
			}
			for i, v := range f.Input.A {
				binary.BigEndian.PutUint32(registers[64+i*4:], v)
			}
			binary.BigEndian.PutUint32(registers[92:], uint32(uint16(f.Input.D[0]))<<16|(0x100000+f.Input.FormatOffset)>>16)
			external := make([]byte, 257)
			for i := range 256 {
				external[i] = byte(i)
			}
			copy(external, []byte("Native player\x00"))
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			cb := NativeDebugFrameCallbacks{Code: cm, CodeBase: 0x100000, Frame: &frame, FormatAddress: 0x100000 + f.Input.FormatOffset, AddressRegisters: &f.Input.A,
				ReadStackLong: func(at int) (uint32, error) {
					at += 32
					if at < 0 || at+4 > len(registers) {
						return 0, fmt.Errorf("fixture stack read %d", at)
					}
					return binary.BigEndian.Uint32(registers[at:]), nil
				},
				ReadAbsolute: func(at uint32) (uint8, error) {
					if at >= 0x900000 && at <= 0x900100 {
						return external[at-0x900000], nil
					}
					// The original oracle's RAM at the saved-return alias $10 is
					// an actual zero-filled low-memory span, not a production default.
					if at < 256 {
						return 0, nil
					}
					return 0, fmt.Errorf("fixture absolute read %#x", at)
				},
				Bitmap: func(at uint32) (NativeBitmapWindow, error) {
					if at != 0xa10100 {
						return NativeBitmapWindow{}, fmt.Errorf("unexpected native debug pointer %#x", at)
					}
					return NativeBitmapWindow{Bytes: window, BitmapOffset: 256}, nil
				},
			}
			if strings.HasPrefix(f.Input.Name, "main-") {
				// The genuine $ef6 inline format consumes D0 alone; the live
				// caller must not supply invented A-register or stack inputs.
				cb.AddressRegisters = nil
				cb.ReadStackLong = nil
				cb.ReadAbsolute = nil
			}
			plan, err := RenderNativeDebugFrame(cb)
			if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native debug all8D differ: got%x want%x", frame.D, f.D)
			}
			if plan.ReturnAddress != f.ReturnAddress {
				t.Errorf("inline return address differs: got%x want%x", plan.ReturnAddress, f.ReturnAddress)
			}
			if string(plan.Text) != string(f.Text) {
				t.Errorf("native formatted text differs: got%x want%x", plan.Text, f.Text)
			}
			for at := 0; at < 0x33c68; at++ {
				if code[at] != expected[at] {
					t.Fatalf("native debug CODE%#x differs: got%x want%x", at, code[at], expected[at])
				}
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(window)); got != f.BitmapHash {
				t.Errorf("native software font bitmap differs: got%s want%s", got, f.BitmapHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(make([]byte, 0x11280))); got != f.BSSHash {
				t.Errorf("native debug unexpectedly writes BSS: %s", f.BSSHash)
			}
			for i, v := range f.Input.A {
				if f.A[i] != v {
					t.Errorf("original debug address register A%d not restored", i)
				}
			}
		})
	}
}

func TestNativeDebugRequiresRealAliasAndBitmapBacking(t *testing.T) {
	code := fileFrameRelocatedCode(t)
	cm := commandNativeMemory(code)
	copy(code[0x32000:], []byte{0xa0, 0})
	frame := NativeFrameRegisterContext{}
	cb := NativeDebugFrameCallbacks{Code: cm, CodeBase: 0x100000, Frame: &frame, FormatAddress: 0x132000}
	if _, err := RenderNativeDebugFrame(cb); err == nil {
		t.Fatal("negative saved-stack alias fabricated missing input")
	}
	copy(code[0x32000:], []byte{'A', 0})
	_ = cm.Write32(0x2e3a, 0xa10100)
	if _, err := RenderNativeDebugFrame(cb); err == nil {
		t.Fatal("debug target silently omitted missing bitmap")
	}
	cb.Bitmap = func(uint32) (NativeBitmapWindow, error) { return NativeBitmapWindow{Bytes: make([]byte, 1)}, nil }
	if _, err := RenderNativeDebugFrame(cb); err == nil {
		t.Fatal("debug text fabricated out-of-window planes")
	}
}
